package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/phase"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

func TestReviewOverheadSubprocessBudget(t *testing.T) {
	tests := []struct {
		name             string
		mode             protocol.TargetMode
		retries          int
		provider         string
		maximumProcesses int
		providerCalls    string
	}{
		{name: "cold local", mode: protocol.TargetLocal, provider: "clean", maximumProcesses: 80, providerCalls: "1"},
		{name: "cold branch", mode: protocol.TargetBranch, provider: "clean", maximumProcesses: 43, providerCalls: "1"},
		{name: "cold commit", mode: protocol.TargetCommit, provider: "clean", maximumProcesses: 33, providerCalls: "1"},
		{name: "malformed retry", mode: protocol.TargetLocal, retries: 1, provider: "retry", maximumProcesses: 104, providerCalls: "2"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tools, calls, _, processLog := writeFakeReviewTools(t, test.provider)
			t.Setenv("PATH", tools+string(os.PathListSeparator)+"/usr/bin:/bin")
			t.Setenv("OPENAI_API_KEY", "fake-provider-credential")
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			repository, targetArguments := benchmarkReviewRepository(t, test.mode, 4<<10)
			if err := os.WriteFile(processLog, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			var measurements []phase.Measurement
			dependencies := dependencies{
				lookupEnv: os.LookupEnv,
				homeDir:   func() (string, error) { return t.TempDir(), nil },
				observePhase: func(measurement phase.Measurement) {
					measurements = append(measurements, measurement)
				},
			}
			arguments := []string{
				"review", "--repository", repository, "--mode", string(test.mode), "--engine", "codex",
				"--isolation", "strict", "--retries", strconv.Itoa(test.retries), "--timeout", "8s", "--output", "json",
			}
			arguments = append(arguments, targetArguments...)
			var stdout strings.Builder
			if exit := run(t.Context(), arguments, &stdout, io.Discard, dependencies); exit != 0 {
				t.Fatalf("run() exit = %d, output = %s", exit, stdout.String())
			}
			var result protocol.Report
			if err := json.Unmarshal([]byte(stdout.String()), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Metadata.Attempts) != test.retries+1 {
				t.Fatalf("attempts = %+v", result.Metadata.Attempts)
			}
			processes, err := os.ReadFile(processLog)
			if err != nil {
				t.Fatal(err)
			}
			if count := len(strings.Fields(string(processes))); count > test.maximumProcesses {
				t.Fatalf("subprocesses = %d, budget = %d", count, test.maximumProcesses)
			}
			providerCalls, err := os.ReadFile(calls)
			if err != nil || strings.TrimSpace(string(providerCalls)) != test.providerCalls {
				t.Fatalf("provider calls = %q, error = %v", providerCalls, err)
			}
			seen := make(map[phase.Name]bool, len(measurements))
			for _, measurement := range measurements {
				seen[measurement.Name] = true
			}
			for _, expected := range []phase.Name{
				phase.Config,
				phase.DependencyProbes,
				phase.TargetFreeze,
				phase.SecretScan,
				phase.ProviderPreparation,
				phase.ProviderProcess,
				phase.ProtocolDecode,
				phase.SourceRevalidation,
				phase.ReportWrite,
			} {
				if !seen[expected] {
					t.Fatalf("phase %s missing from %+v", expected, measurements)
				}
			}
		})
	}
}

func BenchmarkReviewOverhead(b *testing.B) {
	scenarios := []struct {
		name     string
		mode     protocol.TargetMode
		size     int
		retries  int
		provider string
	}{
		{name: "cold/local/4KiB", mode: protocol.TargetLocal, size: 4 << 10, provider: "clean"},
		{name: "cold/branch/4KiB", mode: protocol.TargetBranch, size: 4 << 10, provider: "clean"},
		{name: "cold/commit/4KiB", mode: protocol.TargetCommit, size: 4 << 10, provider: "clean"},
		{name: "malformed-retry/local/4KiB", mode: protocol.TargetLocal, size: 4 << 10, retries: 1, provider: "retry"},
		{name: "cold/local/1MiB", mode: protocol.TargetLocal, size: 1 << 20, provider: "clean"},
	}
	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			tools, calls, _, processLog := writeFakeReviewTools(b, scenario.provider)
			b.Setenv("PATH", tools+string(os.PathListSeparator)+"/usr/bin:/bin")
			b.Setenv("OPENAI_API_KEY", "fake-provider-credential")
			b.Setenv("XDG_CONFIG_HOME", b.TempDir())
			repository, targetArguments := benchmarkReviewRepository(b, scenario.mode, scenario.size)
			arguments := []string{
				"review", "--repository", repository, "--mode", string(scenario.mode),
				"--engine", "codex", "--isolation", "strict", "--retries", strconv.Itoa(scenario.retries),
				"--timeout", "8s", "--max-bytes", strconv.Itoa(max(scenario.size*2, 1<<20)), "--output", "json",
			}
			arguments = append(arguments, targetArguments...)
			totals := make([]time.Duration, 0, b.N)
			overheads := make([]time.Duration, 0, b.N)
			processCounts := make([]int, 0, b.N)
			phaseSamples := make(map[phase.Name][]time.Duration)
			var currentPhases map[phase.Name]time.Duration
			home := b.TempDir()
			dependencies := dependencies{
				lookupEnv: os.LookupEnv,
				homeDir:   func() (string, error) { return home, nil },
				observePhase: func(measurement phase.Measurement) {
					currentPhases[measurement.Name] += measurement.Duration
				},
			}

			b.ResetTimer()
			for range b.N {
				b.StopTimer()
				if err := os.WriteFile(calls, []byte("0\n"), 0o600); err != nil {
					b.Fatal(err)
				}
				if err := os.WriteFile(processLog, nil, 0o600); err != nil {
					b.Fatal(err)
				}
				currentPhases = make(map[phase.Name]time.Duration)
				b.StartTimer()
				started := time.Now()
				exit := run(b.Context(), arguments, io.Discard, io.Discard, dependencies)
				total := time.Since(started)
				b.StopTimer()
				if exit != 0 {
					b.Fatalf("run() exit = %d", exit)
				}
				processes, err := os.ReadFile(processLog)
				if err != nil {
					b.Fatal(err)
				}
				count := len(strings.Fields(string(processes)))
				providerTime := currentPhases[phase.ProviderProcess]
				overhead := total - providerTime
				if overhead < 0 {
					overhead = 0
				}
				totals = append(totals, total)
				overheads = append(overheads, overhead)
				processCounts = append(processCounts, count)
				for name, duration := range currentPhases {
					phaseSamples[name] = append(phaseSamples[name], duration)
				}
				b.StartTimer()
			}
			b.StopTimer()
			b.ReportMetric(durationPercentile(totals, 50).Seconds()*1000, "total_p50_ms/op")
			b.ReportMetric(durationPercentile(totals, 95).Seconds()*1000, "total_p95_ms/op")
			b.ReportMetric(durationPercentile(overheads, 50).Seconds()*1000, "overhead_p50_ms/op")
			b.ReportMetric(durationPercentile(overheads, 95).Seconds()*1000, "overhead_p95_ms/op")
			b.ReportMetric(float64(intPercentile(processCounts, 50)), "subprocesses_p50/op")
			for _, name := range []phase.Name{
				phase.Config,
				phase.DependencyProbes,
				phase.TargetFreeze,
				phase.SecretScan,
				phase.ProviderPreparation,
				phase.ProviderProcess,
				phase.ProtocolDecode,
				phase.SourceRevalidation,
				phase.ReportWrite,
			} {
				if samples := phaseSamples[name]; len(samples) > 0 {
					b.ReportMetric(durationPercentile(samples, 50).Seconds()*1000, string(name)+"_p50_ms/op")
				}
			}
		})
	}
}

func benchmarkReviewRepository(t testing.TB, mode protocol.TargetMode, size int) (string, []string) {
	t.Helper()
	repository := t.TempDir()
	benchmarkGit(t, repository, "init", "-q", "-b", "main")
	benchmarkGit(t, repository, "config", "user.name", "Slopguard Benchmark")
	benchmarkGit(t, repository, "config", "user.email", "benchmark@example.invalid")
	content := strings.Repeat("a", size) + "\n"
	if err := os.WriteFile(filepath.Join(repository, "fixture.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	benchmarkGit(t, repository, "add", "fixture.txt")
	benchmarkGit(t, repository, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "base")
	switch mode {
	case protocol.TargetLocal:
		if err := os.WriteFile(filepath.Join(repository, "fixture.txt"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return repository, nil
	case protocol.TargetBranch:
		benchmarkGit(t, repository, "switch", "-q", "-c", "feature")
		if err := os.WriteFile(filepath.Join(repository, "fixture.txt"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		benchmarkGit(t, repository, "-c", "commit.gpgsign=false", "commit", "-q", "-am", "feature")
		return repository, []string{"--base", "main"}
	case protocol.TargetCommit:
		if err := os.WriteFile(filepath.Join(repository, "fixture.txt"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		benchmarkGit(t, repository, "-c", "commit.gpgsign=false", "commit", "-q", "-am", "reviewed")
		return repository, []string{"--commit", "HEAD"}
	default:
		t.Fatalf("unsupported target mode %q", mode)
		return "", nil
	}
}

func benchmarkGit(t testing.TB, repository string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = repository
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
}

func durationPercentile(values []time.Duration, percentile int) time.Duration {
	ordered := append([]time.Duration(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return ordered[percentileIndex(len(ordered), percentile)]
}

func intPercentile(values []int, percentile int) int {
	ordered := append([]int(nil), values...)
	sort.Ints(ordered)
	return ordered[percentileIndex(len(ordered), percentile)]
}

func percentileIndex(length, percentile int) int {
	if length < 1 {
		return 0
	}
	index := (length*percentile + 99) / 100
	if index < 1 {
		index = 1
	}
	if index > length {
		index = length
	}
	return index - 1
}
