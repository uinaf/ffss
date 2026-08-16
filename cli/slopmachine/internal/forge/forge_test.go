package forge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestNewFailsClosedOnUnknownKind(t *testing.T) {
	if _, err := New("gitlab"); !errors.Is(err, ErrUnknownKind) {
		t.Fatalf("unknown kind: %v", err)
	}
	adapter, err := New(KindGitHub)
	if err != nil || adapter.Kind() != KindGitHub {
		t.Fatalf("github adapter: %v", err)
	}
}

func TestParseChangeRequestURL(t *testing.T) {
	g := NewGitHub(nil)
	ref, err := g.ParseChangeRequestURL("https://github.com/uinaf/slopmachine/pull/38")
	if err != nil || ref.Owner != "uinaf" || ref.Repo != "slopmachine" || ref.Number != 38 {
		t.Fatalf("ref=%+v err=%v", ref, err)
	}
	if ref.String() != "uinaf/slopmachine#38" {
		t.Fatalf("ref string: %s", ref.String())
	}
	for _, invalid := range []string{
		"",
		"https://gitlab.com/uinaf/slopmachine/pull/38",
		"https://github.com/uinaf/slopmachine/issues/38",
		"https://github.com/uinaf/slopmachine/pull/0",
		"https://github.com/uinaf/slopmachine/pull/38/files",
		"http://github.com/uinaf/slopshipper/pull/38",
		"https://github.com/uinaf/slopmachine/pull/38?diff=split#discussion",
	} {
		var forgeErr *Error
		if _, err := g.ParseChangeRequestURL(invalid); err == nil || !errors.As(err, &forgeErr) || forgeErr.Kind != ErrorNotFound {
			t.Errorf("accepted %q: %v", invalid, err)
		}
	}
}

func fakeRunner(view string, threads string, failWith error) Runner {
	return func(_ context.Context, args ...string) ([]byte, error) {
		if failWith != nil {
			return nil, failWith
		}
		if len(args) > 1 && args[0] == "pr" && args[1] == "view" {
			return []byte(view), nil
		}
		if len(args) > 1 && args[0] == "api" && args[1] == "graphql" {
			return []byte(threads), nil
		}
		return nil, fmt.Errorf("unexpected gh invocation: %v", args)
	}
}

const emptyThreads = `{"data":{"repository":{"pullRequest":{"reviewThreads":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`

func TestObserveMapsChecksMergeabilityAndThreads(t *testing.T) {
	view := `{
		"headRefOid": "abc123",
		"state": "OPEN",
		"mergeable": "MERGEABLE",
		"statusCheckRollup": [
			{"status": "COMPLETED", "conclusion": "SUCCESS"},
			{"status": "COMPLETED", "conclusion": "SKIPPED"}
		]
	}`
	threads := `{"data":{"repository":{"pullRequest":{"reviewThreads":{"pageInfo":{"hasNextPage":false},"nodes":[
		{"isResolved": true, "path": "old.go", "line": 1, "comments": {"nodes": [{"author": {"login": "x"}, "body": "done"}]}},
		{"isResolved": false, "path": "cmd/main.go", "line": 42, "comments": {"nodes": [{"author": {"login": "slopzapper[bot]"}, "body": "first line of finding\nmore detail"}]}}
	]}}}}}`
	g := NewGitHub(fakeRunner(view, threads, nil))
	observation, err := g.Observe(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 7})
	if err != nil {
		t.Fatal(err)
	}
	if observation.HeadSHA != "abc123" || observation.Checks != ChecksPassing || observation.Mergeability != MergeableClean {
		t.Fatalf("observation=%+v", observation)
	}
	if observation.UnresolvedThreads != 1 || len(observation.Threads) != 1 {
		t.Fatalf("threads=%+v", observation)
	}
	thread := observation.Threads[0]
	if thread.Author != "slopzapper[bot]" || thread.Path != "cmd/main.go" || thread.Line != 42 ||
		thread.Snippet != "first line of finding" {
		t.Fatalf("thread=%+v", thread)
	}
}

func TestObserveChecksMatrix(t *testing.T) {
	tests := []struct {
		name   string
		rollup string
		want   ChecksState
	}{
		{"none", `[]`, ChecksNone},
		{"failing", `[{"status":"COMPLETED","conclusion":"SUCCESS"},{"status":"COMPLETED","conclusion":"FAILURE"}]`, ChecksFailing},
		{"pending status", `[{"status":"IN_PROGRESS","conclusion":""}]`, ChecksPending},
		{"classic context", `[{"status":"","conclusion":"","state":"SUCCESS"}]`, ChecksPassing},
		{"cancelled", `[{"status":"COMPLETED","conclusion":"CANCELLED"}]`, ChecksFailing},
		{"stale", `[{"status":"COMPLETED","conclusion":"STALE"}]`, ChecksFailing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := fmt.Sprintf(`{"headRefOid":"h","state":"OPEN","mergeable":"UNKNOWN","statusCheckRollup":%s}`, tt.rollup)
			g := NewGitHub(fakeRunner(view, emptyThreads, nil))
			observation, err := g.Observe(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1})
			if err != nil {
				t.Fatal(err)
			}
			if observation.Checks != tt.want {
				t.Fatalf("checks=%s want=%s", observation.Checks, tt.want)
			}
			if observation.Mergeability != MergeableUnknown {
				t.Fatalf("mergeability=%s", observation.Mergeability)
			}
		})
	}
}

func TestObserveMergedAndClosedStates(t *testing.T) {
	for state, want := range map[string]Mergeability{"MERGED": MergeableMerged, "CLOSED": MergeableClosed} {
		view := fmt.Sprintf(`{"headRefOid":"h","state":%q,"mergeable":"MERGEABLE","statusCheckRollup":[]}`, state)
		g := NewGitHub(fakeRunner(view, emptyThreads, nil))
		observation, err := g.Observe(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1})
		if err != nil || observation.Mergeability != want {
			t.Fatalf("state %s: %+v %v", state, observation, err)
		}
	}
}

func TestClassifyMapsFailureTaxonomy(t *testing.T) {
	tests := map[string]ErrorKind{
		"gh pr view: HTTP 404: Not Found":                           ErrorNotFound,
		"gh pr view: could not resolve to a PullRequest":            ErrorNotFound,
		"gh api: HTTP 403: API rate limit exceeded":                 ErrorRateLimit,
		"gh api: HTTP 401: Bad credentials":                         ErrorAuth,
		"gh: To get started with GitHub CLI, run gh auth login":     ErrorAuth,
		"gh pr view: dial tcp: lookup api.github.com: no such host": ErrorTransient,
		"gh api: could not resolve host: api.github.com":            ErrorTransient,
		"gh api: HTTP 429: Too Many Requests":                       ErrorRateLimit,
	}
	for message, want := range tests {
		g := NewGitHub(fakeRunner("", "", errors.New(message)))
		_, err := g.Observe(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1})
		var forgeErr *Error
		if !errors.As(err, &forgeErr) || forgeErr.Kind != want {
			t.Errorf("%q classified %v, want %s", message, err, want)
		}
	}
}

func TestObserveRejectsMalformedPayloads(t *testing.T) {
	g := NewGitHub(fakeRunner("not json", emptyThreads, nil))
	var forgeErr *Error
	if _, err := g.Observe(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1}); !errors.As(err, &forgeErr) || forgeErr.Kind != ErrorTransient {
		t.Fatalf("malformed view: %v", err)
	}
	g = NewGitHub(fakeRunner(`{"headRefOid":"h","state":"OPEN","mergeable":"UNKNOWN","statusCheckRollup":[]}`, "not json", nil))
	if _, err := g.Observe(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1}); !errors.As(err, &forgeErr) || forgeErr.Kind != ErrorTransient {
		t.Fatalf("malformed threads: %v", err)
	}
}

func TestReviewThreadPagination(t *testing.T) {
	page1 := `{"data":{"repository":{"pullRequest":{"reviewThreads":{"pageInfo":{"hasNextPage":true,"endCursor":"CUR1"},"nodes":[
		{"isResolved": false, "path": "a.go", "line": 1, "comments": {"nodes": [{"author": {"login": "r1"}, "body": "one"}]}}
	]}}}}}`
	page2 := `{"data":{"repository":{"pullRequest":{"reviewThreads":{"pageInfo":{"hasNextPage":false},"nodes":[
		{"isResolved": false, "path": "b.go", "line": 2, "comments": {"nodes": [{"author": {"login": "r2"}, "body": "two"}]}},
		{"isResolved": true, "path": "c.go", "line": 3, "comments": {"nodes": []}}
	]}}}}}`
	view := `{"headRefOid":"h","state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[]}`
	g := NewGitHub(func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) > 1 && args[0] == "pr" {
			return []byte(view), nil
		}
		for _, a := range args {
			if a == "cursor=CUR1" {
				return []byte(page2), nil
			}
		}
		return []byte(page1), nil
	})
	observation, err := g.Observe(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1})
	if err != nil {
		t.Fatal(err)
	}
	if observation.UnresolvedThreads != 2 || len(observation.Threads) != 2 {
		t.Fatalf("pagination lost threads: %+v", observation)
	}
	if observation.Threads[1].Path != "b.go" {
		t.Fatalf("second page not sampled: %+v", observation.Threads)
	}
}

// TestObserveLive reads one real pull request when the operator opts in:
// SLOPMACHINE_FORGE_LIVE_PR=https://github.com/OWNER/REPO/pull/N
func TestObserveLive(t *testing.T) {
	url := os.Getenv("SLOPMACHINE_FORGE_LIVE_PR")
	if url == "" {
		t.Skip("set SLOPMACHINE_FORGE_LIVE_PR to run the live observation read")
	}
	g := NewGitHub(nil)
	ref, err := g.ParseChangeRequestURL(strings.TrimSpace(url))
	if err != nil {
		t.Fatal(err)
	}
	observation, err := g.Observe(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if observation.HeadSHA == "" || observation.Checks == "" || observation.Mergeability == "" {
		t.Fatalf("live observation incomplete: %+v", observation)
	}
	t.Logf("live %s: head=%s checks=%s mergeable=%s unresolved=%d",
		ref, observation.HeadSHA, observation.Checks, observation.Mergeability, observation.UnresolvedThreads)
}

func TestReviewsDecodeAndClassify(t *testing.T) {
	reviews := `{"reviews":[
		{"author":{"login":"zapbot"},"state":"APPROVED","submittedAt":"2026-08-15T00:00:00Z"},
		{"author":{"login":"human"},"state":"COMMENTED","submittedAt":"2026-08-15T01:00:00Z"}
	]}`
	g := NewGitHub(fakeRunner(reviews, emptyThreads, nil))
	got, err := g.Reviews(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Author != "zapbot" || got[0].State != "APPROVED" || got[1].SubmittedAt != "2026-08-15T01:00:00Z" {
		t.Fatalf("reviews did not decode: %+v", got)
	}

	g = NewGitHub(fakeRunner("not json", emptyThreads, nil))
	var forgeErr *Error
	if _, err := g.Reviews(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1}); !errors.As(err, &forgeErr) || forgeErr.Kind != ErrorTransient {
		t.Fatalf("bad payload must classify transient: %v", err)
	}

	g = NewGitHub(fakeRunner("", "", errors.New("HTTP 404: Not Found")))
	if _, err := g.Reviews(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1}); !errors.As(err, &forgeErr) || forgeErr.Kind != ErrorNotFound {
		t.Fatalf("missing object must classify not_found: %v", err)
	}
}

func TestHeadReadsOnlyTheRevision(t *testing.T) {
	g := NewGitHub(fakeRunner(`{"headRefOid":"abc1234","state":"OPEN"}`, emptyThreads, nil))
	head, err := g.Head(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1})
	if err != nil || head.SHA != "abc1234" || head.Merged || head.Closed {
		t.Fatalf("head=%+v err=%v", head, err)
	}
	g = NewGitHub(fakeRunner(`{"headRefOid":"abc1234","state":"MERGED"}`, emptyThreads, nil))
	if head, err = g.Head(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1}); err != nil || !head.Merged {
		t.Fatalf("merged state must be reported: head=%+v err=%v", head, err)
	}
	g = NewGitHub(fakeRunner(`{"headRefOid":"abc1234","state":"CLOSED"}`, emptyThreads, nil))
	if head, err = g.Head(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1}); err != nil || !head.Closed {
		t.Fatalf("closed state must be reported: head=%+v err=%v", head, err)
	}
	g = NewGitHub(fakeRunner("", "", errors.New("HTTP 404: Not Found")))
	var forgeErr *Error
	if _, err := g.Head(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1}); !errors.As(err, &forgeErr) || forgeErr.Kind != ErrorNotFound {
		t.Fatalf("missing object must classify not_found: %v", err)
	}
	g = NewGitHub(fakeRunner("not json", emptyThreads, nil))
	if _, err := g.Head(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1}); !errors.As(err, &forgeErr) || forgeErr.Kind != ErrorTransient {
		t.Fatalf("bad payload must classify transient: %v", err)
	}
}

func TestReviewsTolerateNullSubmittedAt(t *testing.T) {
	// PENDING reviews carry submittedAt: null; JSON null into a string field
	// is a no-op, so decoding must not fail and the record must survive with
	// an empty timestamp.
	reviews := `{"reviews":[
		{"author":{"login":"zapbot"},"state":"PENDING","submittedAt":null},
		{"author":{"login":"zapbot"},"state":"APPROVED","submittedAt":"2026-08-15T00:00:00Z"}
	]}`
	g := NewGitHub(fakeRunner(reviews, emptyThreads, nil))
	got, err := g.Reviews(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].SubmittedAt != "" || got[1].State != "APPROVED" {
		t.Fatalf("null submittedAt must decode benignly: %+v", got)
	}
}
