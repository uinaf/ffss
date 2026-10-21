package orchestrator

import (
	"context"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/config"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
	"github.com/uinaf/ffss/cli/slopguard/internal/provider"
	"github.com/uinaf/ffss/cli/slopguard/internal/reviewcache"
	"github.com/uinaf/ffss/cli/slopguard/internal/target"
)

// cacheInputs holds one value per review cache key input: the snapshot (via
// the reviewed commit) and every reviewcache.KeyParts field Run fills in.
type cacheInputs struct {
	commit           int
	version          string
	engine           protocol.ProviderName
	model            string
	effort           config.ReasoningEffort
	webAccess        bool
	repositoryAccess bool
	retries          int
}

const cacheInputFields = 8

// vary replaces one field with a different value from its domain.
func (inputs cacheInputs) vary(field int, random *rand.Rand) cacheInputs {
	switch field {
	case 0:
		inputs.commit = 1 - inputs.commit
	case 1:
		inputs.version = other(random, inputs.version, "1.0.0", "1.0.1", "2.0.0")
	case 2:
		inputs.engine = other(random, inputs.engine, protocol.ProviderCodex, protocol.ProviderClaude)
	case 3:
		inputs.model = other(random, inputs.model, "", "model-a", "model-b")
	case 4:
		inputs.effort = other(random, inputs.effort, config.ReasoningMedium, config.ReasoningHigh, config.ReasoningXHigh)
	case 5:
		inputs.webAccess = !inputs.webAccess
	case 6:
		inputs.repositoryAccess = !inputs.repositoryAccess
	case 7:
		inputs.retries = 1 - inputs.retries
	}
	return inputs
}

func other[T comparable](random *rand.Rand, current T, domain ...T) T {
	for {
		if candidate := domain[random.IntN(len(domain))]; candidate != current {
			return candidate
		}
	}
}

func randomCacheInputs(random *rand.Rand) cacheInputs {
	inputs := cacheInputs{commit: random.IntN(2), version: "1.0.0", engine: protocol.ProviderCodex, model: "", effort: config.ReasoningMedium}
	for field := range cacheInputFields {
		if random.IntN(2) == 0 {
			inputs = inputs.vary(field, random)
		}
	}
	return inputs
}

type cacheOutcome int

const (
	outcomeClean cacheOutcome = iota
	outcomeFindings
	outcomeFailure
)

// TestRunReusesOnlyMatchingCompletedReviews checks that a review is a cache
// hit exactly when the snapshot and every key input match a stored review that
// finished clean or with findings. Each freeze costs Git subprocesses, so the
// seeded generator spreads one lone change per key input across the stored
// reviews instead of sampling pairs blindly.
func TestRunReusesOnlyMatchingCompletedReviews(t *testing.T) {
	t.Parallel()

	repository, commits := cacheRepository(t)
	const seed = 20261021
	random := rand.New(rand.NewPCG(seed, seed))
	fields := random.Perm(cacheInputFields)
	lone := map[cacheOutcome][]int{
		outcomeClean:    fields[:cacheInputFields/2],
		outcomeFindings: fields[cacheInputFields/2:],
	}
	for _, outcome := range []cacheOutcome{outcomeClean, outcomeFindings, outcomeFailure} {
		store, err := reviewcache.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		stored := randomCacheInputs(random)
		first := Run(t.Context(), cacheOptions(t, repository, commits, store, stored, &cacheReviewer{outcome: outcome}, nil))
		if (first.Status == protocol.StatusFailure) != (outcome == outcomeFailure) {
			t.Fatalf("stored review = %s, failure %+v", first.Status, first.Failure)
		}
		probes := []cacheInputs{stored}
		for _, field := range lone[outcome] {
			probes = append(probes, stored.vary(field, random))
		}
		for _, probe := range probes {
			want := probe == stored && outcome != outcomeFailure
			// A miss reviews again and fails, so the store keeps one entry.
			reviewer := &cacheReviewer{outcome: outcomeFailure}
			hit := false
			report := Run(t.Context(), cacheOptions(t, repository, commits, store, probe, reviewer, func() { hit = true }))
			if hit != want || hit == (reviewer.calls != 0) {
				t.Fatalf("seed %d: hit = %t after %d reviewer calls, want %t\nstored %+v (outcome %d)\nprobe  %+v", seed, hit, reviewer.calls, want, stored, outcome, probe)
			}
			if hit && (report.Status != first.Status || report.Metadata.Target.SnapshotHash != first.Metadata.Target.SnapshotHash) {
				t.Fatalf("hit returned %s for %s, stored %s for %s", report.Status, report.Metadata.Target.SnapshotHash, first.Status, first.Metadata.Target.SnapshotHash)
			}
		}
	}
}

func cacheOptions(t *testing.T, repository string, commits [2]string, store *reviewcache.Store, inputs cacheInputs, reviewer provider.Reviewer, hit func()) Options {
	t.Helper()
	collector, err := target.New(target.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return Options{
		Collector:   collector,
		NewReviewer: func(protocol.ProviderName, string) provider.Reviewer { return reviewer },
		Repository:  repository,
		Target:      target.Request{Mode: protocol.TargetCommit, Commit: commits[inputs.commit], Prompt: "Review the change.", MaxBytes: target.DefaultMaxBytes},
		Config: config.Effective{
			Engine:           config.Value[protocol.ProviderName]{Value: inputs.engine, Source: config.SourceFlag},
			Model:            config.Value[string]{Value: inputs.model, Source: config.SourceFlag},
			ReasoningEffort:  config.Value[config.ReasoningEffort]{Value: inputs.effort, Source: config.SourceFlag},
			Timeout:          config.Value[config.Duration]{Value: config.Duration(time.Minute), Source: config.SourceDefault},
			Retries:          config.Value[int]{Value: inputs.retries, Source: config.SourceFlag},
			MaxBytes:         config.Value[int64]{Value: target.DefaultMaxBytes, Source: config.SourceDefault},
			WebAccess:        config.Value[bool]{Value: inputs.webAccess, Source: config.SourceFlag},
			RepositoryAccess: config.Value[bool]{Value: inputs.repositoryAccess, Source: config.SourceFlag},
		},
		Cache:            store,
		SlopguardVersion: inputs.version,
		CacheHit:         hit,
	}
}

type cacheReviewer struct {
	outcome cacheOutcome
	calls   int
}

func (reviewer *cacheReviewer) Review(_ context.Context, request provider.Request) (provider.Result, error) {
	reviewer.calls++
	if reviewer.outcome == outcomeFailure {
		class := protocol.FailureProvider
		return provider.Result{}, &provider.Error{
			Class:   class,
			Message: "injected provider failure",
			Attempt: &protocol.Attempt{Number: 1, Outcome: protocol.AttemptFailed, DurationMS: 1, ErrorClass: &class},
		}
	}
	findings := []protocol.Finding{}
	if reviewer.outcome == outcomeFindings {
		file := request.Target.Files[0]
		line := file.LineRanges[0].StartLine
		findings = append(findings, protocol.Finding{
			Title: "Injected defect", Body: "This behavior is incorrect.", Priority: protocol.PriorityP1,
			Confidence: 0.9, Category: protocol.CategoryBug,
			Location: protocol.Location{FilePath: file.FilePath, StartLine: line, EndLine: line},
		})
	}
	return provider.Result{
		Review:    protocol.Review{Findings: findings, OverallExplanation: "Reviewed the injected change.", OverallConfidence: 0.9},
		Provider:  protocol.Provider{Name: request.Config.Engine.Value, Model: "fake-model", Version: "1.2.3"},
		Attempt:   protocol.Attempt{Number: 1, Outcome: protocol.AttemptValid, DurationMS: 1},
		Duration:  time.Millisecond,
		WebAccess: request.Config.WebAccess.Value,
	}, nil
}

// cacheRepository returns a repository with two commits whose changes freeze
// to different snapshots.
func cacheRepository(t *testing.T) (string, [2]string) {
	t.Helper()
	repository := t.TempDir()
	git := func(arguments ...string) string {
		command := exec.CommandContext(t.Context(), "git", arguments...)
		command.Dir = repository
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q", "-b", "main")
	var commits [2]string
	for index, content := range []string{"base\n", "first change\n", "second change\n"} {
		if err := os.WriteFile(filepath.Join(repository, "app.go"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		git("add", "app.go")
		git("-c", "user.name=Slopguard Test", "-c", "user.email=test@example.invalid", "commit", "-q", "-m", content)
		if index > 0 {
			commits[index-1] = git("rev-parse", "HEAD")
		}
	}
	return repository, commits
}
