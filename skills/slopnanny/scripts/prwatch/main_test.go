package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) Sleep(_ context.Context, d time.Duration) error {
	c.now = c.now.Add(d)
	return nil
}

// fakeForge answers each poll from a timeline keyed by minutes since t0: the
// latest entry at or before the current time wins.
type fakeForge struct {
	clock   *fakeClock
	prs     map[int]func() (pr, error)
	runs    map[int][]workflowRun
	eyes    map[int][]string
	jobs    map[int64][]job
	eyesErr error
	jobsErr error
	runFn   func(n int) ([]workflowRun, error)
	runPoll int
	polls   int
}

func at[T any](c *fakeClock, timeline map[int]T) T {
	minute := int(c.now.Sub(t0) / time.Minute)
	best := -1
	for m := range timeline {
		if m <= minute && m > best {
			best = m
		}
	}
	return timeline[best]
}

func (f *fakeForge) PullRequest(context.Context, string, int) (pr, error) {
	f.polls++
	return at(f.clock, f.prs)()
}

func (f *fakeForge) BotEyes(context.Context, string, int) ([]string, error) {
	return at(f.clock, f.eyes), f.eyesErr
}

func (f *fakeForge) Runs(context.Context, string, string, string) ([]workflowRun, error) {
	if f.runFn != nil {
		f.runPoll++
		return f.runFn(f.runPoll)
	}
	return at(f.clock, f.runs), nil
}

func (f *fakeForge) FailedJobs(_ context.Context, _ string, id int64) ([]job, error) {
	return f.jobs[id], f.jobsErr
}

func open(checks ...check) func() (pr, error) {
	return func() (pr, error) {
		return pr{Viewer: "me", State: "OPEN", MergeState: "CLEAN", Head: "aaaaaaa1", HeadCommitted: t0.Add(-time.Hour), Checks: checks}, nil
	}
}

func with(base func() (pr, error), edit func(*pr)) func() (pr, error) {
	return func() (pr, error) {
		p, err := base()
		edit(&p)
		return p, err
	}
}

var (
	passed  = func(name string) check { return check{Name: name, Status: "COMPLETED", Conclusion: "SUCCESS"} }
	running = func(name string) check { return check{Name: name, Status: "IN_PROGRESS"} }
	failed  = func(name string) check {
		return check{Name: name, Status: "COMPLETED", Conclusion: "FAILURE", URL: "https://ci/" + name}
	}
)

func watchFor(t *testing.T, f *fakeForge, args ...string) (int, string, time.Duration) {
	t.Helper()
	f.clock.now = t0
	var stdout, stderr bytes.Buffer
	d := deps{forge: func(string, string) forge { return f }, origin: func(context.Context) (string, error) { return "git@github.com:o/r.git", nil }, clock: f.clock}
	code := run(context.Background(), append(args, "7"), d, &stdout, &stderr)
	return code, stdout.String() + stderr.String(), f.clock.now.Sub(t0)
}

func newFake() *fakeForge {
	return &fakeForge{clock: &fakeClock{now: t0}, prs: map[int]func() (pr, error){}, runs: map[int][]workflowRun{}, eyes: map[int][]string{}}
}

func TestReadyOnlyAfterLateChecksFinish(t *testing.T) {
	f := newFake()
	f.prs[0] = open()
	f.prs[3] = open(passed("lint"))
	f.prs[4] = open(passed("lint"), running("e2e"))
	f.prs[9] = open(passed("lint"), passed("e2e"))
	code, out, took := watchFor(t, f)
	if code != exitDone || !strings.HasPrefix(out, "ready:") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
	if took < 10*time.Minute {
		t.Fatalf("reported ready after %s, before the late check settled", took)
	}
}

func TestRequiredFailureNeedsAttention(t *testing.T) {
	f := newFake()
	req := failed("verify")
	req.Required = true
	f.prs[0] = open(req, passed("lint"))
	code, out, _ := watchFor(t, f)
	if code != exitAttention || !strings.HasPrefix(out, "checks-failed:") || !strings.Contains(out, "https://ci/verify") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestOptionalFailureDoesNotBlockWhenRequiredPass(t *testing.T) {
	f := newFake()
	req := passed("verify")
	req.Required = true
	f.prs[0] = open(req, failed("flaky"))
	code, out, _ := watchFor(t, f)
	if code != exitDone || !strings.Contains(out, "flaky failure") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestSurfacesGhErrors(t *testing.T) {
	boom := &cliError{name: "gh", args: []string{"api", "graphql"}, stderr: "HTTP 502", err: errors.New("exit 1")}
	t.Run("transient", func(t *testing.T) {
		f := newFake()
		f.prs[0] = func() (pr, error) { return pr{}, boom }
		f.prs[1] = open(passed("ci"))
		code, out, _ := watchFor(t, f)
		if code != exitDone || !strings.Contains(out, "poll 1/3 failed: gh api graphql: HTTP 502") {
			t.Fatalf("one failed poll should be printed and retried; code %d, output:\n%s", code, out)
		}
	})
	t.Run("repeated after the pull request read", func(t *testing.T) {
		f := newFake()
		f.prs[0] = open(passed("ci"))
		f.eyesErr = boom
		if code, out, _ := watchFor(t, f); code != exitError || !strings.Contains(out, "poll 3/3 failed") {
			t.Fatalf("code %d, output:\n%s", code, out)
		}
	})
	t.Run("failed job lookup", func(t *testing.T) {
		f := newFake()
		f.prs[0] = with(open(), func(p *pr) { p.State, p.MergeCommit = "MERGED", "bbbbbbb2" })
		f.runs[0] = []workflowRun{{ID: 1, Name: "CI", Event: "push", Status: "completed", Conclusion: "failure"}}
		f.jobsErr = boom
		if code, out, _ := watchFor(t, f); code != exitError || strings.Contains(out, "runs-failed") {
			t.Fatalf("an incomplete failure report must not pass as one; code %d, output:\n%s", code, out)
		}
	})
	t.Run("repeated", func(t *testing.T) {
		f := newFake()
		f.prs[0] = func() (pr, error) { return pr{}, boom }
		code, out, _ := watchFor(t, f)
		if code != exitError || !strings.Contains(out, "HTTP 502") || f.polls != 3 {
			t.Fatalf("code %d after %d polls, output:\n%s", code, f.polls, out)
		}
	})
	t.Run("rate limit", func(t *testing.T) {
		f := newFake()
		f.prs[0] = func() (pr, error) {
			return pr{}, &cliError{name: "gh", args: []string{"api"}, stderr: "API rate limit exceeded", err: errors.New("exit 1")}
		}
		if code, _, _ := watchFor(t, f); code != exitError || f.polls != 1 {
			t.Fatalf("rate limit should stop at once; code %d after %d polls", code, f.polls)
		}
	})
}

func TestDeadlineReportsWhatIsStillRunning(t *testing.T) {
	f := newFake()
	f.prs[0] = open(running("e2e"))
	code, out, took := watchFor(t, f, "-timeout", "10m")
	if code != exitTimeout || !strings.Contains(out, "still running: e2e") || took > 10*time.Minute {
		t.Fatalf("code %d after %s, output:\n%s", code, took, out)
	}
}

func TestNewCommentFromSomeoneElseEndsTheWait(t *testing.T) {
	f := newFake()
	f.prs[0] = open(running("ci"))
	f.prs[2] = with(open(running("ci")), func(p *pr) {
		p.Comments = []comment{{Author: actor{Login: "me"}, Created: t0.Add(2 * time.Minute), Body: "my own reply"}}
	})
	f.prs[5] = with(f.prs[2], func(p *pr) {
		p.Comments = append(p.Comments, comment{Author: actor{Login: "reviewbot", Bot: true}, Created: t0.Add(5 * time.Minute), Body: "P1: nil map write", URL: "https://c/1"})
	})
	code, out, took := watchFor(t, f)
	if code != exitAttention || !strings.HasPrefix(out, "activity:") || !strings.Contains(out, "P1: nil map write") || strings.Contains(out, "my own reply") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
	if took < 5*time.Minute {
		t.Fatalf("the viewer's own comment ended the wait after %s", took)
	}
}

func TestBlockedByThreadsAndChangesRequested(t *testing.T) {
	f := newFake()
	f.prs[0] = with(open(passed("ci")), func(p *pr) {
		p.Threads = []thread{{Path: "a.go", Line: 3, Last: comment{Author: actor{Login: "ann"}, Body: "this leaks"}}}
		p.Verdicts = []review{{Author: actor{Login: "ann"}, State: "CHANGES_REQUESTED", Submitted: t0.Add(-time.Hour)}}
	})
	code, out, _ := watchFor(t, f)
	for _, want := range []string{"blocked:", "1 unresolved thread", "ann requested changes", "a.go:3 ann: this leaks"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q; code %d, output:\n%s", want, code, out)
		}
	}
	if code != exitAttention {
		t.Fatalf("code %d", code)
	}
}

func TestWaitsForWorkingBotThenGivesUp(t *testing.T) {
	f := newFake()
	f.prs[0] = open(passed("ci"))
	f.eyes[0] = []string{"codex-bot"}
	code, out, took := watchFor(t, f, "-bot-wait", "5m")
	if code != exitDone || !strings.Contains(out, "still working after 5m0s: codex-bot") || took < 5*time.Minute {
		t.Fatalf("code %d after %s, output:\n%s", code, took, out)
	}
}

func TestMergeWatchesRunsItTriggers(t *testing.T) {
	f := newFake()
	f.prs[0] = with(open(passed("ci")), func(p *pr) { p.State, p.MergeCommit = "MERGED", "bbbbbbb2" })
	ci := workflowRun{ID: 1, Name: "CI", Event: "push", Status: "in_progress"}
	nightly := workflowRun{ID: 9, Name: "Nightly", Event: "schedule", Status: "completed", Conclusion: "failure"}
	f.runs[0] = []workflowRun{nightly}
	f.runs[1] = []workflowRun{ci, nightly}
	ci.Status, ci.Conclusion = "completed", "success"
	f.runs[4] = []workflowRun{ci, nightly}
	deploy := workflowRun{ID: 2, Name: "Deploy", Event: "workflow_run", Status: "completed", Conclusion: "failure", URL: "https://runs/2"}
	f.runs[5] = []workflowRun{ci, deploy, nightly}
	f.jobs = map[int64][]job{2: {{Name: "smoke", Conclusion: "failure", URL: "https://jobs/smoke"}}}
	code, out, _ := watchFor(t, f)
	if code != exitAttention || !strings.HasPrefix(out, "runs-failed:") || !strings.Contains(out, "job smoke failure https://jobs/smoke") || strings.Contains(out, "Nightly") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestMergeWithoutRuns(t *testing.T) {
	f := newFake()
	f.prs[0] = with(open(), func(p *pr) { p.State = "MERGED" })
	code, out, _ := watchFor(t, f)
	if code != exitDone || !strings.HasPrefix(out, "no-runs:") || !strings.Contains(out, "aaaaaaa1") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestParseTarget(t *testing.T) {
	var opts options
	if err := parseTarget("https://github.com/uinaf/ffss/pull/236/files", &opts); err != nil || opts.repo != "uinaf/ffss" || opts.number != 236 {
		t.Fatalf("got %+v, %v", opts, err)
	}
	if err := parseTarget("main", &opts); err == nil {
		t.Fatal("accepted a branch name")
	}
}

// fakeGlab puts a glab on PATH that answers `glab api [--paginate] <path>`
// from responses and fails on any other path.
func fakeGlab(t *testing.T, responses map[string]string) {
	t.Helper()
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("#!/bin/sh\nshift\n[ \"$1\" = --paginate ] && shift\ncase \"$1\" in\n")
	for path, body := range responses {
		fmt.Fprintf(&b, "'%s') cat <<'JSON'\n%s\nJSON\n;;\n", path, body)
	}
	b.WriteString("*) echo \"404 Not Found: $1\" >&2; exit 1;;\nesac\n")
	if err := os.WriteFile(filepath.Join(dir, "glab"), []byte(b.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func watchGitLab(t *testing.T, args ...string) (int, string) {
	t.Helper()
	clock := &fakeClock{now: t0}
	d := deps{forge: newForge, origin: func(context.Context) (string, error) { return "", errors.New("no remote") }, clock: clock}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), append(args, "https://gitlab.example.com/group/sub/app/-/merge_requests/12"), d, &stdout, &stderr)
	return code, stdout.String() + stderr.String()
}

const glMR = "projects/group%2Fsub%2Fapp/merge_requests/12"

func glabOpenMR(pipeline, jobs, discussions, reviewers, mergeStatus string) map[string]string {
	return map[string]string{
		"user": `{"username":"me"}`,
		glMR: `{"state":"opened","draft":false,"web_url":"https://gitlab.example.com/group/sub/app/-/merge_requests/12",
			"sha":"abc1234def","target_branch":"main","detailed_merge_status":"` + mergeStatus + `",
			"head_pipeline":` + pipeline + `}`,
		"projects/99/pipelines/500/jobs?per_page=100": jobs,
		glMR + "/reviewers":                           reviewers,
		glMR + "/discussions?per_page=100":            discussions,
		glMR + "/award_emoji?per_page=100":            `[]`,
	}
}

func TestGitLabFailedJobInForkPipeline(t *testing.T) {
	fakeGlab(t, glabOpenMR(
		`{"id":500,"project_id":99,"status":"failed","web_url":"https://gitlab.example.com/fork/app/-/pipelines/500"}`,
		`[{"name":"lint","status":"success","allow_failure":false,"web_url":"https://j/1"},
		  {"name":"docs","status":"manual","allow_failure":true,"web_url":"https://j/2"}]
		 [{"name":"e2e","status":"failed","allow_failure":false,"web_url":"https://j/3"}]`,
		`[]`, `[]`, "ci_must_pass"))
	code, out := watchGitLab(t)
	for _, want := range []string{"checks-failed: group/sub/app!12", "e2e (required) failure https://j/3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q; code %d, output:\n%s", want, code, out)
		}
	}
	if code != exitAttention || strings.Contains(out, "docs") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestGitLabBlockedByThreadReviewerAndApprovals(t *testing.T) {
	fakeGlab(t, glabOpenMR(
		`{"id":500,"project_id":99,"status":"success","web_url":"https://p/500"}`,
		`[{"name":"lint","status":"success","allow_failure":false,"web_url":"https://j/1"}]`,
		`[{"individual_note":false,"notes":[
		    {"id":7,"body":"this leaks","author":{"username":"ann"},"created_at":"2026-10-07T11:00:00Z","resolvable":true,"resolved":false,
		     "position":{"new_path":"app.go","new_line":42}}]},
		  {"individual_note":true,"notes":[{"id":8,"body":"approved this merge request","author":{"username":"ann"},"created_at":"2026-10-07T11:00:00Z","system":true}]}]`,
		`[{"user":{"username":"ann"},"state":"requested_changes"},{"user":{"username":"GitLabDuo"},"state":"reviewed"}]`,
		"not_approved"))
	code, out := watchGitLab(t)
	for _, want := range []string{"blocked:", "1 unresolved thread", "ann requested changes", "merge blocked: not_approved",
		"app.go:42 ann: this leaks https://gitlab.example.com/group/sub/app/-/merge_requests/12#note_7"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q; code %d, output:\n%s", want, code, out)
		}
	}
	if code != exitAttention {
		t.Fatalf("code %d", code)
	}
}

func TestGitLabNewCommentEndsTheWait(t *testing.T) {
	fakeGlab(t, glabOpenMR(`null`, `[]`,
		`[{"individual_note":true,"notes":[{"id":9,"body":"P1: wrong key","author":{"username":"review-bot"},"created_at":"2026-10-07T12:00:01Z"}]}]`,
		`[]`, "checking"))
	code, out := watchGitLab(t, "-since", "2026-10-07T12:00:00Z")
	if code != exitAttention || !strings.Contains(out, "review-bot: P1: wrong key") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestGitLabMergeWatchesDefaultBranchPipelines(t *testing.T) {
	fakeGlab(t, map[string]string{
		"user": `{"username":"me"}`,
		glMR: `{"state":"merged","web_url":"https://gitlab.example.com/group/sub/app/-/merge_requests/12","sha":"abc1234",
			"merge_commit_sha":null,"squash_commit_sha":"squash99","target_branch":"main","detailed_merge_status":"not_open"}`,
		"projects/group%2Fsub%2Fapp/pipelines?sha=squash99&ref=main&per_page=100": `[
			{"id":601,"status":"success","source":"push","ref":"main","web_url":"https://p/601"},
			{"id":603,"status":"failed","source":"schedule","ref":"main","web_url":"https://p/603"},
			{"id":604,"status":"failed","source":"web","ref":"main","web_url":"https://p/604"}]`,
		"projects/group%2Fsub%2Fapp/pipelines?sha=squash99&ref=main&per_page=100&source=parent_pipeline": `[
			{"id":602,"status":"failed","source":"parent_pipeline","ref":"main","web_url":"https://p/602"}]`,
		"projects/group%2Fsub%2Fapp/pipelines/602/jobs?scope[]=failed&per_page=100": `[
			{"name":"deploy","status":"failed","allow_failure":false,"web_url":"https://j/deploy"},
			{"name":"lint-optional","status":"failed","allow_failure":true,"web_url":"https://j/opt"}]`,
	})
	code, out := watchGitLab(t)
	for _, want := range []string{"runs-failed: group/sub/app!12 merged as squash99", "job deploy failed https://j/deploy"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q; code %d, output:\n%s", want, code, out)
		}
	}
	if code != exitAttention || strings.Contains(out, "603") || strings.Contains(out, "604") || strings.Contains(out, "lint-optional") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestGitLabErrorsSurface(t *testing.T) {
	fakeGlab(t, map[string]string{"user": `{"username":"me"}`})
	code, out := watchGitLab(t)
	if code != exitError || !strings.Contains(out, "404 Not Found: "+glMR) {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestPicksForge(t *testing.T) {
	cases := []struct{ remote, args, want string }{
		{"git@github.com:o/r.git", "", "github"},
		{"https://gitlab.com/group/sub/app.git", "", "gitlab"},
		{"ssh://git@gitlab.corp.example:2222/group/app.git", "", "gitlab"},
		{"git@git.example.com:group/app.git", "-forge gitlab", "gitlab"},
		{"git@git.example.com:group/app.git", "", ""},
	}
	for _, c := range cases {
		var got string
		d := deps{
			forge: func(kind, _ string) forge {
				got = kind
				return &fakeForge{clock: &fakeClock{now: t0}, prs: map[int]func() (pr, error){0: open()}}
			},
			origin: func(context.Context) (string, error) { return c.remote, nil },
			clock:  &fakeClock{now: t0},
		}
		args := append(strings.Fields(c.args), "-settle", "0s", "7")
		code := run(context.Background(), args, d, io.Discard, io.Discard)
		if got != c.want || (c.want == "" && code != exitError) {
			t.Errorf("%s %q: picked %q (exit %d), want %q", c.remote, c.args, got, code, c.want)
		}
	}
}

func TestThreadActivityEvenAfterViewerRepliesOrResolves(t *testing.T) {
	f := newFake()
	f.prs[0] = open(running("ci"))
	f.prs[3] = with(open(running("ci")), func(p *pr) {
		p.ThreadComments = []comment{
			{Author: actor{Login: "ann"}, Created: t0.Add(2 * time.Minute), Body: "off by one"},
			{Author: actor{Login: "me"}, Created: t0.Add(3 * time.Minute), Body: "fixed"},
		}
	})
	code, out, _ := watchFor(t, f)
	if code != exitAttention || !strings.Contains(out, "ann: off by one") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestEventInTheSinceSecondCounts(t *testing.T) {
	f := newFake()
	f.prs[0] = with(open(running("ci")), func(p *pr) {
		p.Comments = []comment{{Author: actor{Login: "ann"}, Created: t0, Body: "same second"}}
	})
	if code, out, _ := watchFor(t, f, "-since", t0.Format(time.RFC3339)); code != exitAttention {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestUnknownMergeStateNeverReportsReady(t *testing.T) {
	f := newFake()
	f.prs[0] = with(open(passed("ci")), func(p *pr) { p.MergeState = "UNKNOWN" })
	code, out, _ := watchFor(t, f, "-timeout", "10m")
	if code != exitTimeout || !strings.Contains(out, "merge state still unknown") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestRepositoryDispatchRunsCount(t *testing.T) {
	f := newFake()
	f.prs[0] = with(open(), func(p *pr) { p.State, p.MergeCommit = "MERGED", "bbbbbbb2" })
	f.runs[0] = []workflowRun{
		{ID: 1, Name: "CI", Event: "push", Status: "completed", Conclusion: "success"},
		{ID: 2, Name: "Deploy", Event: "repository_dispatch", Status: "completed", Conclusion: "failure"},
	}
	if code, out, _ := watchFor(t, f); code != exitAttention || !strings.Contains(out, "Deploy (repository_dispatch) failure") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestGitLabRequiredManualJobNeedsAttention(t *testing.T) {
	fakeGlab(t, glabOpenMR(
		`{"id":500,"project_id":99,"status":"manual","web_url":"https://p/500"}`,
		`[{"name":"approve-release","status":"manual","allow_failure":false,"web_url":"https://j/9"}]`,
		`[]`, `[]`, "mergeable"))
	code, out := watchGitLab(t)
	if code != exitAttention || !strings.Contains(out, "approve-release (required) action_required https://j/9") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestGitLabPipelineRequiredButMissing(t *testing.T) {
	fakeGlab(t, glabOpenMR(`null`, `[]`, `[]`, `[]`, "ci_must_pass"))
	code, out := watchGitLab(t)
	if code != exitAttention || !strings.Contains(out, "merge blocked: ci_must_pass") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestGitLabApprovalWithoutCommentIsActivity(t *testing.T) {
	fakeGlab(t, glabOpenMR(`{"id":500,"project_id":99,"status":"running","web_url":"https://p/500"}`,
		`[{"name":"lint","status":"running","allow_failure":false,"web_url":"https://j/1"}]`,
		`[{"individual_note":true,"notes":[{"id":5,"body":"approved this merge request","author":{"username":"ann"},"created_at":"2026-10-07T12:00:30Z","system":true}]}]`,
		`[{"user":{"username":"ann"},"state":"approved"}]`, "ci_still_running"))
	code, out := watchGitLab(t, "-since", "2026-10-07T12:00:00Z")
	if code != exitAttention || !strings.Contains(out, "ann reviewed: APPROVED") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestForgeFlagMustAgreeWithURL(t *testing.T) {
	code, out := watchGitLab(t, "-forge", "github")
	if code != exitError || !strings.Contains(out, "contradicts") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestExplicitRepoStillUsesOriginHost(t *testing.T) {
	var host string
	d := deps{
		forge: func(_, h string) forge {
			host = h
			return &fakeForge{clock: &fakeClock{now: t0}, prs: map[int]func() (pr, error){0: open(failed("ci"))}}
		},
		origin: func(context.Context) (string, error) { return "git@gitlab.corp.example:group/app.git", nil },
		clock:  &fakeClock{now: t0},
	}
	run(context.Background(), []string{"-R", "group/other", "-forge", "gitlab", "7"}, d, io.Discard, io.Discard)
	if host != "gitlab.corp.example" {
		t.Fatalf("glab would query host %q", host)
	}
}

func TestDefaultSinceIncludesTheStartingSecond(t *testing.T) {
	f := newFake()
	f.prs[0] = with(open(running("ci")), func(p *pr) {
		p.Comments = []comment{{Author: actor{Login: "ann"}, Created: t0, Body: "stamped to the second"}}
	})
	clock := &fakeClock{now: t0.Add(700 * time.Millisecond)}
	f.clock = clock
	d := deps{forge: func(string, string) forge { return f }, origin: func(context.Context) (string, error) { return "git@github.com:o/r.git", nil }, clock: clock}
	var out bytes.Buffer
	if code := run(context.Background(), []string{"7"}, d, &out, io.Discard); code != exitAttention {
		t.Fatalf("code %d, output:\n%s", code, out.String())
	}
}

func TestRunErrorsMustBeConsecutive(t *testing.T) {
	f := newFake()
	f.prs[0] = with(open(), func(p *pr) { p.State, p.MergeCommit = "MERGED", "bbbbbbb2" })
	boom := &cliError{name: "gh", args: []string{"api"}, stderr: "HTTP 502", err: errors.New("exit 1")}
	pending := []workflowRun{{ID: 1, Name: "CI", Event: "push", Status: "in_progress"}}
	f.runFn = func(n int) ([]workflowRun, error) {
		if n%2 == 1 {
			return nil, boom
		}
		if n < 8 {
			return pending, nil
		}
		return []workflowRun{{ID: 1, Name: "CI", Event: "push", Status: "completed", Conclusion: "success"}}, nil
	}
	if code, out, _ := watchFor(t, f); code != exitDone {
		t.Fatalf("separated failures ended the watch; code %d, output:\n%s", code, out)
	}
}

func TestGitHubUsesTargetHost(t *testing.T) {
	for _, tc := range []struct {
		name, host, ambient, target, origin, state, want string
		code                                             int
	}{
		{"explicit URL", "github.com", "github.corp.example", "https://github.com/o/r/pull/7", "", "OPEN", "ready: o/r#7", exitDone},
		{"origin after merge", "github.corp.example", "github.com", "7", "git@github.corp.example:o/r.git", "MERGED", "job build failure https://ci/job", exitAttention},
		{"CLI default", "github.corp.example", "github.corp.example", "7", "", "OPEN", "ready: o/r#7", exitDone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			script := `#!/bin/sh
selected_host=${GH_HOST:-github.com}
endpoint=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --hostname) selected_host=$2; shift ;;
    graphql|repos/*) endpoint=$1 ;;
  esac
  shift
done
if [ "$selected_host" != '__HOST__' ]; then
  echo "wrong forge selected: $selected_host" >&2
  exit 1
fi
case "$endpoint" in
  graphql)
    echo '{"data":{"viewer":{"login":"me"},"repository":{"pullRequest":{"url":"https://__HOST__/o/r/pull/7","state":"__STATE__","mergeStateStatus":"CLEAN","headRefOid":"aaaaaaa1","baseRefName":"main","mergeCommit":{"oid":"bbbbbbb2"}}}}}' ;;
  repos/o/r/issues/7/reactions*) ;;
  repos/o/r/actions/runs\?*)
    echo '{"id":1,"name":"CI","event":"push","status":"completed","conclusion":"failure","url":"https://ci/run"}' ;;
  repos/o/r/actions/runs/1/jobs*)
    echo '{"name":"build","conclusion":"failure","url":"https://ci/job"}' ;;
  *) echo "unexpected endpoint: $endpoint" >&2; exit 1 ;;
esac
`
			script = strings.NewReplacer("__HOST__", tc.host, "__STATE__", tc.state).Replace(script)
			if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("GH_HOST", tc.ambient)
			d := deps{forge: newForge, origin: func(context.Context) (string, error) { return tc.origin, nil }, clock: &fakeClock{now: t0}}
			var out bytes.Buffer
			code := run(context.Background(), []string{"-R", "o/r", "-appear", "0s", "-settle", "0s", tc.target}, d, &out, &out)
			if code != tc.code || !strings.Contains(out.String(), tc.want) {
				t.Fatalf("code %d, output:\n%s", code, out.String())
			}
		})
	}
}
