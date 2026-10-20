// Command evals measures review recall and false positives on planted-bug
// cases. Each case is a base tree plus a change; slopguard reviews the change
// as one commit under every configuration, and a finding catches the bug when
// it cites the planted file within lineTolerance lines of the planted line.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

const lineTolerance = 3

type caseSpec struct {
	ID           string `json:"-"`
	Summary      string `json:"summary"`
	Language     string `json:"language"`
	NeedsContext bool   `json:"needs_context"`
	Bug          *struct {
		File string `json:"file"`
		Line int    `json:"line"`
		// EndLine widens the accepted range when the defect spans lines,
		// such as a call and the early return it causes.
		EndLine int `json:"end_line,omitempty"`
	} `json:"bug"`
	directory string
}

type configuration struct {
	Name             string
	Engine           string
	RepositoryAccess bool
}

type result struct {
	Case         string  `json:"case"`
	Config       string  `json:"config"`
	Run          int     `json:"run"`
	Status       string  `json:"status"`
	FailureClass string  `json:"failure_class,omitempty"`
	Clean        bool    `json:"clean"`
	NeedsContext bool    `json:"needs_context"`
	Caught       bool    `json:"caught"`
	Findings     int     `json:"findings"`
	Unrelated    int     `json:"unrelated"`
	Seconds      float64 `json:"seconds"`
	Experiment   string  `json:"experiment"`
}

func main() {
	slopguard := flag.String("slopguard", "", "slopguard binary to evaluate (required unless -summary)")
	casesDir := flag.String("cases", "evals/testdata/cases", "directory of eval cases")
	configNames := flag.String("configs", "codex-diff,codex-snapshot,claude-diff,claude-snapshot", "comma-separated configurations")
	only := flag.String("case", "", "comma-separated case IDs to run; empty runs all")
	runs := flag.Int("runs", 3, "runs per case and configuration")
	parallel := flag.Int("parallel", 4, "concurrent reviews")
	effort := flag.String("effort", "medium", "reasoning effort for every review")
	timeout := flag.Duration("timeout", 20*time.Minute, "per-review timeout")
	out := flag.String("out", "eval-results.jsonl", "JSONL results file; existing rows are kept and skipped")
	summaryOnly := flag.Bool("summary", false, "print the summary of -out without running reviews")
	flag.Parse()

	if err := run(*slopguard, *casesDir, *configNames, *only, *runs, *parallel, *effort, *timeout, *out, *summaryOnly); err != nil {
		fmt.Fprintln(os.Stderr, "evals:", err)
		os.Exit(1)
	}
}

func run(slopguard, casesDir, configNames, only string, runs, parallel int, effort string, timeout time.Duration, out string, summaryOnly bool) error {
	configs, err := parseConfigurations(configNames)
	if err != nil {
		return err
	}
	if !summaryOnly {
		if slopguard == "" {
			return errors.New("-slopguard is required")
		}
		if runs < 1 {
			return errors.New("-runs must be at least 1")
		}
		if timeout <= 0 {
			return errors.New("-timeout must be positive")
		}
		cases, err := loadCases(casesDir, only)
		if err != nil {
			return err
		}
		if err := evaluate(slopguard, cases, configs, runs, parallel, effort, timeout, out); err != nil {
			return err
		}
	}
	if _, err := os.Stat(out); err != nil {
		return fmt.Errorf("results: %w", err)
	}
	rows, err := readResults(out)
	if err != nil {
		return err
	}
	return printSummary(os.Stdout, rows, configs)
}

func parseConfigurations(names string) ([]configuration, error) {
	known := map[string]configuration{
		"codex-diff":      {Name: "codex-diff", Engine: "codex"},
		"codex-snapshot":  {Name: "codex-snapshot", Engine: "codex", RepositoryAccess: true},
		"claude-diff":     {Name: "claude-diff", Engine: "claude"},
		"claude-snapshot": {Name: "claude-snapshot", Engine: "claude", RepositoryAccess: true},
	}
	var configs []configuration
	seen := map[string]bool{}
	for _, name := range strings.Split(names, ",") {
		name = strings.TrimSpace(name)
		config, ok := known[name]
		if !ok {
			return nil, fmt.Errorf("unknown configuration %q", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate configuration %q", name)
		}
		seen[name] = true
		configs = append(configs, config)
	}
	return configs, nil
}

func loadCases(directory, only string) ([]caseSpec, error) {
	wanted := map[string]bool{}
	for _, id := range strings.Split(only, ",") {
		if id = strings.TrimSpace(id); id != "" {
			wanted[id] = true
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	var cases []caseSpec
	for _, entry := range entries {
		if !entry.IsDir() || (len(wanted) > 0 && !wanted[entry.Name()]) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name(), "case.json"))
		if err != nil {
			return nil, err
		}
		var spec caseSpec
		if err := json.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("case %s: %w", entry.Name(), err)
		}
		spec.ID, spec.directory = entry.Name(), filepath.Join(directory, entry.Name())
		cases = append(cases, spec)
		delete(wanted, entry.Name())
	}
	for id := range wanted {
		return nil, fmt.Errorf("unknown case %q", id)
	}
	if len(cases) == 0 {
		return nil, errors.New("no cases selected")
	}
	return cases, nil
}

type job struct {
	spec       caseSpec
	repository string
	config     configuration
	run        int
}

func evaluate(slopguard string, cases []caseSpec, configs []configuration, runs, parallel int, effort string, timeout time.Duration, out string) error {
	experiment, err := fingerprint(slopguard, effort, timeout, parallel, cases)
	if err != nil {
		return err
	}
	done, err := readResults(out)
	if err != nil {
		return err
	}
	finished := map[string]bool{}
	for _, row := range done {
		if row.Experiment != experiment {
			return fmt.Errorf("%s holds rows from another binary, effort, or case set; pass a new -out", out)
		}
		finished[fmt.Sprintf("%s/%s/%d", row.Case, row.Config, row.Run)] = true
	}
	workspace, err := os.MkdirTemp("", "slopguard-eval-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	configHome := filepath.Join(workspace, "config")
	if err := os.Mkdir(configHome, 0o700); err != nil {
		return err
	}
	var jobs []job
	for _, spec := range cases {
		repository, err := prepareRepository(workspace, spec)
		if err != nil {
			return fmt.Errorf("case %s: %w", spec.ID, err)
		}
		for runNumber := 1; runNumber <= runs; runNumber++ {
			for _, config := range configs {
				if !finished[fmt.Sprintf("%s/%s/%d", spec.ID, config.Name, runNumber)] {
					jobs = append(jobs, job{spec: spec, repository: repository, config: config, run: runNumber})
				}
			}
		}
	}
	file, err := os.OpenFile(out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	var writeMu sync.Mutex
	var writeErr error
	failed := func() bool {
		writeMu.Lock()
		defer writeMu.Unlock()
		return writeErr != nil
	}
	queue := make(chan job)
	var workers sync.WaitGroup
	completed := 0
	for range max(parallel, 1) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for next := range queue {
				if failed() {
					continue
				}
				row := review(slopguard, configHome, effort, timeout, next)
				row.Experiment = experiment
				encoded, err := json.Marshal(row)
				writeMu.Lock()
				if err == nil && writeErr == nil {
					_, err = file.Write(append(encoded, '\n'))
				}
				if err != nil && writeErr == nil {
					writeErr = fmt.Errorf("record %s %s run %d: %w", row.Case, row.Config, row.Run, err)
				}
				completed++
				fmt.Fprintf(os.Stderr, "[%d/%d] %s %s run %d: %s caught=%t findings=%d %.0fs\n", completed, len(jobs), row.Case, row.Config, row.Run, row.Status, row.Caught, row.Findings, row.Seconds)
				writeMu.Unlock()
			}
		}()
	}
	// A failed write stops scheduling, so no review spends quota without a row.
	for _, next := range jobs {
		if failed() {
			break
		}
		queue <- next
	}
	close(queue)
	workers.Wait()
	return writeErr
}

func prepareRepository(workspace string, spec caseSpec) (string, error) {
	repository := filepath.Join(workspace, "cases", spec.ID)
	if err := os.MkdirAll(repository, 0o700); err != nil {
		return "", err
	}
	git := func(arguments ...string) error {
		command := exec.Command("git", append([]string{"-c", "user.name=slopguard eval", "-c", "user.email=eval@example.invalid", "-c", "commit.gpgsign=false"}, arguments...)...)
		command.Dir = repository
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %w: %s", arguments[0], err, output)
		}
		return nil
	}
	if err := git("init", "--quiet", "--initial-branch=main"); err != nil {
		return "", err
	}
	if err := copyTree(filepath.Join(spec.directory, "base"), repository); err != nil {
		return "", err
	}
	if err := git("add", "--all"); err != nil {
		return "", err
	}
	if err := git("commit", "--quiet", "--message", "base"); err != nil {
		return "", err
	}
	if err := copyTree(filepath.Join(spec.directory, "change"), repository); err != nil {
		return "", err
	}
	if err := git("add", "--all"); err != nil {
		return "", err
	}
	return repository, git("commit", "--quiet", "--message", "change")
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o600)
	})
}

func review(slopguard, configHome, effort string, timeout time.Duration, next job) result {
	row := result{Case: next.spec.ID, Config: next.config.Name, Run: next.run, Clean: next.spec.Bug == nil, NeedsContext: next.spec.NeedsContext}
	arguments := []string{"review", "--repository", next.repository, "--mode", "commit", "--commit", "HEAD",
		"--engine", next.config.Engine, "--reasoning-effort", effort, "--web-access=false", "--cache=false",
		"--output", "json", "--prompt", "Review this commit."}
	if next.config.RepositoryAccess {
		arguments = append(arguments, "--repository-access")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, slopguard, arguments...)
	// Interrupt instead of kill so slopguard stops its provider process group.
	command.Cancel = func() error { return command.Process.Signal(os.Interrupt) }
	command.WaitDelay = time.Minute
	command.Env = append(withoutSlopguardEnvironment(os.Environ()), "XDG_CONFIG_HOME="+configHome)
	var stdout bytes.Buffer
	command.Stdout, command.Stderr = &stdout, io.Discard
	started := time.Now()
	_ = command.Run()
	row.Seconds = time.Since(started).Seconds()
	report, err := protocol.DecodeReport(stdout.Bytes())
	if err != nil {
		row.Status, row.FailureClass = "failure", "unparsable"
		return row
	}
	row.Status = string(report.Status)
	if report.Failure != nil {
		row.FailureClass = string(report.Failure.Class)
	}
	if report.Review == nil {
		return row
	}
	row.Findings = len(report.Review.Findings)
	for _, finding := range report.Review.Findings {
		if next.spec.Bug != nil && finding.Location.FilePath == next.spec.Bug.File &&
			finding.Location.StartLine <= max(next.spec.Bug.Line, next.spec.Bug.EndLine)+lineTolerance &&
			finding.Location.EndLine >= next.spec.Bug.Line-lineTolerance {
			row.Caught = true
			continue
		}
		row.Unrelated++
	}
	return row
}

func readResults(path string) ([]result, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	var rows []result
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var row result
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		rows = append(rows, row)
	}
	return rows, scanner.Err()
}

func printSummary(output io.Writer, rows []result, configs []configuration) error {
	for _, row := range rows {
		if row.Experiment != rows[0].Experiment {
			return errors.New("results mix experiments; summarize one -out per experiment")
		}
	}
	fmt.Fprintln(output, "| Config | Bugs caught | Needs-context caught | Clean runs flagged | Unrelated findings per bug run | Failed runs | Median time |")
	fmt.Fprintln(output, "| --- | --- | --- | --- | --- | --- | --- |")
	for _, config := range configs {
		var bugRuns, caught, contextRuns, contextCaught, cleanRuns, cleanFlagged, unrelated, failed int
		var seconds []float64
		for _, row := range rows {
			if row.Config != config.Name {
				continue
			}
			if row.Status == "failure" {
				failed++
				// A failed bug review still counts as a missed bug.
				if !row.Clean {
					bugRuns++
					if row.NeedsContext {
						contextRuns++
					}
				}
				continue
			}
			seconds = append(seconds, row.Seconds)
			if row.Clean {
				cleanRuns++
				if row.Findings > 0 {
					cleanFlagged++
				}
				continue
			}
			bugRuns++
			unrelated += row.Unrelated
			if row.Caught {
				caught++
			}
			if row.NeedsContext {
				contextRuns++
				if row.Caught {
					contextCaught++
				}
			}
		}
		fmt.Fprintf(output, "| %s | %s | %s | %s | %s | %d | %s |\n", config.Name,
			ratio(caught, bugRuns), ratio(contextCaught, contextRuns), ratio(cleanFlagged, cleanRuns),
			perRun(unrelated, bugRuns), failed, median(seconds))
	}
	return nil
}

func ratio(part, whole int) string {
	if whole == 0 {
		return "–"
	}
	return fmt.Sprintf("%d/%d (%.0f%%)", part, whole, 100*float64(part)/float64(whole))
}

func perRun(total, runs int) string {
	if runs == 0 {
		return "–"
	}
	return fmt.Sprintf("%.2f", float64(total)/float64(runs))
}

func median(values []float64) string {
	if len(values) == 0 {
		return "–"
	}
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 0 {
		return fmt.Sprintf("%.0fs", (values[middle-1]+values[middle])/2)
	}
	return fmt.Sprintf("%.0fs", values[middle])
}

// withoutSlopguardEnvironment drops SLOPGUARD_* settings so every row runs on
// engine defaults plus the runner's flags.
func withoutSlopguardEnvironment(environment []string) []string {
	kept := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, "SLOPGUARD_") {
			kept = append(kept, entry)
		}
	}
	return kept
}

// fingerprint identifies an experiment by the binary, effort, timeout,
// parallelism, and case files,
// so resuming never mixes rows produced under different settings.
func fingerprint(slopguard, effort string, timeout time.Duration, parallel int, cases []caseSpec) (string, error) {
	hash := sha256.New()
	binary, err := os.Open(slopguard)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(hash, binary)
	_ = binary.Close()
	if err != nil {
		return "", err
	}
	fmt.Fprintf(hash, "\x00effort=%s\x00timeout=%s\x00parallel=%d\x00", effort, timeout, parallel)
	for _, spec := range cases {
		err := filepath.WalkDir(spec.directory, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(hash, "%s\x00%d\x00", filepath.ToSlash(path), len(content))
			_, _ = hash.Write(content)
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil))[:16], nil
}
