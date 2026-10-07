package main

import (
	"bytes"
	"context"
	"errors"
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
	clock *fakeClock
	prs   map[int]func() (pr, error)
	runs  map[int][]workflowRun
	eyes  map[int][]string
	jobs  map[int64][]job
	polls int
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

func (f *fakeForge) CurrentRepo(context.Context) (string, error) { return "o/r", nil }

func (f *fakeForge) PullRequest(context.Context, string, int) (pr, error) {
	f.polls++
	return at(f.clock, f.prs)()
}

func (f *fakeForge) BotEyes(context.Context, string, int) ([]string, error) {
	return at(f.clock, f.eyes), nil
}

func (f *fakeForge) Runs(context.Context, string, string) ([]workflowRun, error) {
	return at(f.clock, f.runs), nil
}

func (f *fakeForge) FailedJobs(_ context.Context, _ string, id int64) ([]job, error) {
	return f.jobs[id], nil
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
	code := run(context.Background(), append(args, "7"), f, f.clock, &stdout, &stderr)
	return code, stdout.String(), f.clock.now.Sub(t0)
}

func newForge() *fakeForge {
	return &fakeForge{clock: &fakeClock{now: t0}, prs: map[int]func() (pr, error){}, runs: map[int][]workflowRun{}, eyes: map[int][]string{}}
}

func TestReadyOnlyAfterLateChecksFinish(t *testing.T) {
	f := newForge()
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
	f := newForge()
	req := failed("verify")
	req.Required = true
	f.prs[0] = open(req, passed("lint"))
	code, out, _ := watchFor(t, f)
	if code != exitAttention || !strings.HasPrefix(out, "checks-failed:") || !strings.Contains(out, "https://ci/verify") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestOptionalFailureDoesNotBlockWhenRequiredPass(t *testing.T) {
	f := newForge()
	req := passed("verify")
	req.Required = true
	f.prs[0] = open(req, failed("flaky"))
	code, out, _ := watchFor(t, f)
	if code != exitDone || !strings.Contains(out, "flaky failure") {
		t.Fatalf("code %d, output:\n%s", code, out)
	}
}

func TestSurfacesGhErrors(t *testing.T) {
	boom := &ghError{args: []string{"api", "graphql"}, stderr: "HTTP 502", err: errors.New("exit 1")}
	t.Run("transient", func(t *testing.T) {
		f := newForge()
		f.prs[0] = func() (pr, error) { return pr{}, boom }
		f.prs[1] = open(passed("ci"))
		if code, out, _ := watchFor(t, f); code != exitDone {
			t.Fatalf("one failed poll should be retried; code %d, output:\n%s", code, out)
		}
	})
	t.Run("repeated", func(t *testing.T) {
		f := newForge()
		f.prs[0] = func() (pr, error) { return pr{}, boom }
		code, out, _ := watchFor(t, f)
		if code != exitError || !strings.Contains(out, "HTTP 502") || f.polls != 3 {
			t.Fatalf("code %d after %d polls, output:\n%s", code, f.polls, out)
		}
	})
	t.Run("rate limit", func(t *testing.T) {
		f := newForge()
		f.prs[0] = func() (pr, error) {
			return pr{}, &ghError{args: []string{"api"}, stderr: "API rate limit exceeded", err: errors.New("exit 1")}
		}
		if code, _, _ := watchFor(t, f); code != exitError || f.polls != 1 {
			t.Fatalf("rate limit should stop at once; code %d after %d polls", code, f.polls)
		}
	})
}

func TestDeadlineReportsWhatIsStillRunning(t *testing.T) {
	f := newForge()
	f.prs[0] = open(running("e2e"))
	code, out, took := watchFor(t, f, "-timeout", "10m")
	if code != exitTimeout || !strings.Contains(out, "still running: e2e") || took > 10*time.Minute {
		t.Fatalf("code %d after %s, output:\n%s", code, took, out)
	}
}

func TestNewCommentFromSomeoneElseEndsTheWait(t *testing.T) {
	f := newForge()
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
	f := newForge()
	f.prs[0] = with(open(passed("ci")), func(p *pr) {
		p.Threads = []thread{{Path: "a.go", Line: 3, Last: comment{Author: actor{Login: "ann"}, Body: "this leaks"}}}
		p.Reviews = []review{
			{Author: actor{Login: "ann"}, State: "CHANGES_REQUESTED", Submitted: t0.Add(-time.Hour)},
			{Author: actor{Login: "ann"}, State: "COMMENTED", Submitted: t0.Add(-time.Minute)},
		}
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
	f := newForge()
	f.prs[0] = open(passed("ci"))
	f.eyes[0] = []string{"codex-bot"}
	code, out, took := watchFor(t, f, "-bot-wait", "5m")
	if code != exitDone || !strings.Contains(out, "still working after 5m0s: codex-bot") || took < 5*time.Minute {
		t.Fatalf("code %d after %s, output:\n%s", code, took, out)
	}
}

func TestMergeWatchesRunsItTriggers(t *testing.T) {
	f := newForge()
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
	f := newForge()
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
