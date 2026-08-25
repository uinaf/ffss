package forge

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestGitLabParseChangeRequestURL(t *testing.T) {
	g := NewGitLab(nil)
	ref, err := g.ParseChangeRequestURL("https://gitlab.example.com/platform/services/api/-/merge_requests/38/")
	if err != nil || ref.Host != "gitlab.example.com" || ref.Owner != "platform/services" || ref.Repo != "api" || ref.Number != 38 {
		t.Fatalf("ref=%+v err=%v", ref, err)
	}
	if ref.String() != "gitlab.example.com/platform/services/api!38" {
		t.Fatalf("ref string: %s", ref.String())
	}
	for _, invalid := range []string{
		"",
		"http://gitlab.com/o/r/-/merge_requests/1",
		"https://gitlab.com/o/r/merge_requests/1",
		"https://gitlab.com/o/r/-/issues/1",
		"https://gitlab.com/o/r/-/merge_requests/0",
		"https://gitlab.com/o/r/-/merge_requests/+1",
		"https://gitlab.com/o/r/-/merge_requests/-1",
		"https://gitlab.com//o/r/-/merge_requests/1",
		"https://gitlab.com/o/r/-/merge_requests/1//",
		"https://gitlab.com/o/r/-/merge_requests/1/diffs",
		"https://gitlab.com/o/r/-/merge_requests/1?view=parallel",
	} {
		var forgeErr *Error
		if _, err := g.ParseChangeRequestURL(invalid); err == nil || !errors.As(err, &forgeErr) || forgeErr.Kind != ErrorNotFound {
			t.Errorf("accepted %q: %v", invalid, err)
		}
	}
}

// TestGitLabObserveLive reads one real merge request when the operator opts
// in: SLOPMACHINE_FORGE_LIVE_MR=https://gitlab.example/GROUP/REPO/-/merge_requests/N
func TestGitLabObserveLive(t *testing.T) {
	raw := strings.TrimSpace(os.Getenv("SLOPMACHINE_FORGE_LIVE_MR"))
	if raw == "" {
		t.Skip("set SLOPMACHINE_FORGE_LIVE_MR to run the live observation read")
	}
	g := NewGitLab(nil)
	ref, err := g.ParseChangeRequestURL(raw)
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

func gitLabRunner(ref ChangeRequestRef, mr, discussions, approvals, notes string, fail error) Runner {
	host := ref.Host
	if host == "" {
		host = "gitlab.com"
	}
	base := fmt.Sprintf("projects/%s/merge_requests/%d", url.PathEscape(ref.Owner+"/"+ref.Repo), ref.Number)
	return func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) != 4 || args[0] != "api" || args[2] != "--hostname" || args[3] != host {
			return nil, fmt.Errorf("unexpected glab invocation: %v", args)
		}
		if fail != nil {
			return nil, fail
		}
		switch args[1] {
		case base:
			return []byte(mr), nil
		case base + "/discussions?per_page=100&page=1":
			return []byte(discussions), nil
		case base + "/approvals":
			return []byte(approvals), nil
		case base + "/notes?sort=asc&order_by=created_at&per_page=100&page=1":
			return []byte(notes), nil
		default:
			return nil, fmt.Errorf("unexpected glab endpoint: %v", args)
		}
	}
}

func TestGitLabObserveMapsPipelineMergeabilityAndDiscussions(t *testing.T) {
	mr := `{"sha":"abc1234","state":"opened","detailed_merge_status":"mergeable","merge_status":"can_be_merged","head_pipeline":{"status":"success"}}`
	discussions := `[
		{"id":"resolved","notes":[{"id":1,"resolvable":true,"resolved":true}]},
		{"id":"open","notes":[
			{"id":2,"body":"finding","author":{"username":"review_bot"},"resolvable":true,"resolved":false,"position":{"new_path":"cmd/main.go","new_line":42}},
			{"id":3,"body":"latest line\nmore","author":{"username":"human"},"updated_at":"2026-08-25T10:00:00Z"}
		]}
	]`
	ref := ChangeRequestRef{Host: "gitlab.example", Owner: "group/sub", Repo: "repo", Number: 7}
	g := NewGitLab(gitLabRunner(ref, mr, discussions, `{}`, `[]`, nil))
	obs, err := g.Observe(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if obs.HeadSHA != "abc1234" || obs.Checks != ChecksPassing || obs.Mergeability != MergeableClean {
		t.Fatalf("observation=%+v", obs)
	}
	if obs.UnresolvedThreads != 1 || len(obs.Threads) != 1 || obs.ThreadsDigest == "" {
		t.Fatalf("threads=%+v", obs)
	}
	thread := obs.Threads[0]
	if thread.ID != "open" || thread.LastCommentID != "3" || thread.Author != "human" || thread.Path != "cmd/main.go" || thread.Line != 42 || thread.Snippet != "latest line" {
		t.Fatalf("thread=%+v", thread)
	}
}

func TestGitLabChecksAndMergeability(t *testing.T) {
	for status, want := range map[string]ChecksState{
		"": ChecksNone, "success": ChecksPassing, "skipped": ChecksPassing,
		"failed": ChecksFailing, "canceled": ChecksFailing, "running": ChecksPending, "manual": ChecksPending,
	} {
		if got := gitLabPipelineChecks(status); got != want {
			t.Errorf("pipeline %q=%s want %s", status, got, want)
		}
	}
	for _, tt := range []struct {
		state, detailed, fallback string
		want                      Mergeability
	}{
		{"merged", "conflict", "cannot_be_merged", MergeableMerged},
		{"closed", "mergeable", "can_be_merged", MergeableClosed},
		{"opened", "mergeable", "", MergeableClean},
		{"opened", "conflict", "", MergeableConflicting},
		{"opened", "checking", "unchecked", MergeableUnknown},
	} {
		if got := gitLabMergeability(tt.state, tt.detailed, tt.fallback); got != tt.want {
			t.Errorf("mergeability %v=%s want %s", tt, got, tt.want)
		}
	}
}

func TestGitLabHeadAndReviews(t *testing.T) {
	ref := ChangeRequestRef{Host: "gitlab.com", Owner: "o", Repo: "r", Number: 3}
	g := NewGitLab(gitLabRunner(ref,
		`{"sha":"abc1234","state":"merged"}`,
		`[]`,
		`{"approved_by":[{"user":{"username":"approve_bot"}}]}`,
		`[{"id":9,"author":{"username":"comment.bot"},"body":"reviewed","created_at":"2026-08-25T09:00:00Z"},{"id":10,"system":true,"author":{"username":"system"}}]`, nil))
	head, err := g.Head(context.Background(), ref)
	if err != nil || head.SHA != "abc1234" || !head.Merged || head.Closed {
		t.Fatalf("head=%+v err=%v", head, err)
	}
	reviews, err := g.Reviews(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 2 || reviews[0].Author != "approve_bot" || reviews[0].State != "APPROVED" || reviews[1].Author != "comment.bot" || reviews[1].State != "COMMENTED" {
		t.Fatalf("reviews=%+v", reviews)
	}
}

func TestGitLabFailuresAreClassified(t *testing.T) {
	ref := ChangeRequestRef{Host: "gitlab.com", Owner: "o", Repo: "r", Number: 1}
	for message, want := range map[string]ErrorKind{
		"401 Unauthorized":      ErrorAuth,
		"429 Too Many Requests": ErrorRateLimit,
		"404 Not Found":         ErrorNotFound,
		"dial tcp: timeout":     ErrorTransient,
	} {
		g := NewGitLab(gitLabRunner(ref, "", "", "", "", errors.New(message)))
		_, err := g.Head(context.Background(), ref)
		var forgeErr *Error
		if !errors.As(err, &forgeErr) || forgeErr.Kind != want {
			t.Errorf("%q classified %v, want %s", message, err, want)
		}
	}
}

func TestGitLabMissingExecutableIsTransient(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	g := NewGitLab(nil)
	_, err := g.Head(context.Background(), ChangeRequestRef{Host: "gitlab.com", Owner: "o", Repo: "r", Number: 1})
	var forgeErr *Error
	if !errors.As(err, &forgeErr) || forgeErr.Kind != ErrorTransient {
		t.Fatalf("missing glab classified %v, want %s", err, ErrorTransient)
	}
}

func TestGitLabAPIUsesEncodedNestedProjectAndURLHost(t *testing.T) {
	g := NewGitLab(func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) != 4 || args[0] != "api" || args[1] != "projects/group%2Fsub%2Frepo/merge_requests/9" || args[2] != "--hostname" || args[3] != "gitlab.example:8443" {
			return nil, fmt.Errorf("unexpected glab invocation: %v", args)
		}
		return []byte(`{"sha":"abc1234","state":"opened"}`), nil
	})
	head, err := g.Head(context.Background(), ChangeRequestRef{Host: "gitlab.example:8443", Owner: "group/sub", Repo: "repo", Number: 9})
	if err != nil || head.SHA != "abc1234" {
		t.Fatalf("head=%+v err=%v", head, err)
	}
}

func TestGitLabRejectsMalformedPayloads(t *testing.T) {
	ref := ChangeRequestRef{Host: "gitlab.com", Owner: "o", Repo: "r", Number: 1}
	var forgeErr *Error
	g := NewGitLab(gitLabRunner(ref, "not json", `[]`, `{}`, `[]`, nil))
	if _, err := g.Head(context.Background(), ref); !errors.As(err, &forgeErr) || forgeErr.Kind != ErrorTransient {
		t.Fatalf("malformed merge request: %v", err)
	}
	g = NewGitLab(gitLabRunner(ref, `{"sha":"abc1234","state":"opened"}`, "not json", `{}`, `[]`, nil))
	if _, err := g.Observe(context.Background(), ref); !errors.As(err, &forgeErr) || forgeErr.Kind != ErrorTransient {
		t.Fatalf("malformed discussions: %v", err)
	}
}
