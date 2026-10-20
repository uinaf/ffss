package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

func TestMain(m *testing.M) {
	// Binary end-to-end tests inherit this, so they never touch the account
	// cache.
	cache, err := os.MkdirTemp("", "slopguard-test-cache-")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("XDG_CACHE_HOME", cache); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(cache)
	os.Exit(code)
}

func TestReviewReusesCachedResultForUnchangedSnapshot(t *testing.T) {
	t.Parallel()

	repository := reviewRepository(t)
	reviewer := &scriptedReviewer{results: []reviewStep{{result: cleanResult()}, {result: cleanResult()}}}
	dependencies := reviewDependencies(t, reviewer)
	cache := t.TempDir()
	dependencies.cacheDirectory = func() (string, error) { return cache, nil }
	review := func(arguments ...string) (protocol.Report, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		run(t.Context(), append([]string{
			"review", "--repository", repository, "--mode", "local", "--engine", "codex",
			"--prompt", "Check the change.", "--output", "json",
		}, arguments...), &stdout, &stderr, dependencies)
		var report protocol.Report
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("decode report: %v: %s", err, stdout.String())
		}
		return report, stderr.String()
	}

	first, _ := review()
	second, progress := review()
	if first.Status != protocol.StatusClean || second.Status != protocol.StatusClean || len(reviewer.prompts) != 1 {
		t.Fatalf("statuses %s/%s after %d reviewer calls, want one call", first.Status, second.Status, len(reviewer.prompts))
	}
	if !strings.Contains(progress, "reusing the cached review") {
		t.Fatalf("stderr = %q, want the cache notice", progress)
	}
	if _, _ = review("--cache=false"); len(reviewer.prompts) != 2 {
		t.Fatalf("--cache=false made %d reviewer calls, want a second one", len(reviewer.prompts))
	}
	if err := os.WriteFile(filepath.Join(repository, "app.go"), []byte("changed again\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The scripted reviewer has no third result, so a fresh review fails.
	if third, _ := review(); third.Status != protocol.StatusFailure || len(reviewer.prompts) != 3 {
		t.Fatalf("changed snapshot reused the cache: %s after %d calls", third.Status, len(reviewer.prompts))
	}
}
