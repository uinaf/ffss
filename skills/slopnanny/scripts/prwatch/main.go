// Command prwatch waits on a GitHub pull request until slopnanny has something
// to act on, then prints what it saw and exits. Run it where `gh` is signed in:
//
//	go run <skill dir>/scripts/prwatch/main.go [flags] <number | url>
//
// An open pull request is watched until its checks fail, someone else posts a
// review or comment, its head moves, it merges or closes, or its checks settle
// and it is ready or blocked. A merged one has the runs its merge commit
// started watched until they finish. One poll is one GraphQL query.
//
// Exit codes: 0 ready, merged, or runs passed; 1 needs attention; 2 usage or
// gh failure; 3 deadline reached.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	exitDone      = 0
	exitAttention = 1
	exitError     = 2
	exitTimeout   = 3
)

type options struct {
	repo                      string
	number                    int
	timeout, interval, settle time.Duration
	appear, botWait           time.Duration
	since                     time.Time
	maxErrors                 int
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], ghForge{}, realClock{}, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, f forge, clock clock, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("prwatch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var opts options
	var since string
	flags.StringVar(&opts.repo, "R", "", "owner/repo (default: the repository of the current directory, or the URL's)")
	flags.DurationVar(&opts.timeout, "timeout", 30*time.Minute, "give up and report what is still pending after this long")
	flags.DurationVar(&opts.interval, "interval", 30*time.Second, "time between polls")
	flags.DurationVar(&opts.settle, "settle", time.Minute, "how long finished checks or runs must stay unchanged before they count as settled, so late ones are caught")
	flags.DurationVar(&opts.appear, "appear", 2*time.Minute, "how long to wait for the first check or run to appear")
	flags.DurationVar(&opts.botWait, "bot-wait", 5*time.Minute, "how long to wait for a review bot that is visibly working")
	flags.StringVar(&since, "since", "", "RFC 3339 time; reviews and comments after it count as new (default: now). Pass the previous run's since line")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: prwatch [flags] <number | url>")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitError
	}
	if flags.NArg() != 1 || opts.interval <= 0 {
		flags.Usage()
		return exitError
	}
	opts.maxErrors = 3
	if err := parseTarget(flags.Arg(0), &opts); err != nil {
		fmt.Fprintln(stderr, "prwatch:", err)
		return exitError
	}
	opts.since = clock.Now()
	if since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			fmt.Fprintln(stderr, "prwatch: -since:", err)
			return exitError
		}
		opts.since = t
	}
	if opts.repo == "" {
		repo, err := f.CurrentRepo(ctx)
		if err != nil {
			fmt.Fprintln(stderr, "prwatch: no -R and no repository here:", err)
			return exitError
		}
		opts.repo = repo
	}
	w := &watcher{f: f, clock: clock, opts: opts, out: stdout, log: stderr}
	return w.watch(ctx)
}

var prURL = regexp.MustCompile(`^https?://[^/]+/([^/]+/[^/]+)/pull/(\d+)`)

func parseTarget(arg string, opts *options) error {
	if m := prURL.FindStringSubmatch(arg); m != nil {
		if opts.repo == "" {
			opts.repo = m[1]
		}
		arg = m[2]
	}
	n, err := strconv.Atoi(strings.TrimPrefix(arg, "#"))
	if err != nil || n <= 0 {
		return fmt.Errorf("want a pull request number or URL, got %q", arg)
	}
	opts.number = n
	return nil
}

// Forge data, as prwatch needs it.

type pr struct {
	Viewer, URL, State, MergeState string
	Draft                          bool
	Head, MergeCommit              string
	HeadCommitted                  time.Time
	Checks                         []check
	ReviewRequests                 []actor
	Reviews                        []review
	Comments                       []comment
	Threads                        []thread
}

type actor struct {
	Login string
	Bot   bool
}

type check struct {
	Name, Status, Conclusion, URL string
	Required                      bool
}

type review struct {
	Author    actor
	State     string
	Submitted time.Time
	Commit    string
	URL, Body string
}

type comment struct {
	Author  actor
	Created time.Time
	URL     string
	Body    string
}

type thread struct {
	Path     string
	Line     int
	Outdated bool
	Last     comment
}

type workflowRun struct {
	ID                                   int64
	Name, Event, Status, Conclusion, URL string
}

type job struct {
	Name, Conclusion, URL string
}

type forge interface {
	CurrentRepo(ctx context.Context) (string, error)
	PullRequest(ctx context.Context, repo string, number int) (pr, error)
	BotEyes(ctx context.Context, repo string, number int) ([]string, error)
	Runs(ctx context.Context, repo, sha string) ([]workflowRun, error)
	FailedJobs(ctx context.Context, repo string, runID int64) ([]job, error)
}

type clock interface {
	Now() time.Time
	Sleep(ctx context.Context, d time.Duration) error
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Watching.

type watcher struct {
	f     forge
	clock clock
	opts  options
	out   io.Writer
	log   io.Writer

	start, deadline time.Time
	head            string
	signature       string
	stableSince     time.Time
	progress        string
}

type verdict struct {
	state   string
	code    int
	reasons []string
}

func (w *watcher) watch(ctx context.Context) int {
	w.start = w.clock.Now()
	w.deadline = w.start.Add(w.opts.timeout)
	errorsInRow := 0
	var last pr
	var lastPolled time.Time
	for {
		polled := w.clock.Now()
		p, err := w.f.PullRequest(ctx, w.opts.repo, w.opts.number)
		var v *verdict
		if err == nil {
			errorsInRow = 0
			last, lastPolled = p, polled
			if p.State == "MERGED" {
				return w.watchRuns(ctx, p)
			}
			v, err = w.judge(ctx, p, polled)
			if err == nil && v != nil {
				w.report(p, *v, polled)
				return v.code
			}
		}
		if err != nil {
			if code, stop := w.failed(err, &errorsInRow); stop {
				return code
			}
		}
		if code, stop := w.wait(ctx); stop {
			if code == exitTimeout {
				w.report(last, verdict{state: "timeout", code: exitTimeout, reasons: pendingReasons(last, w.opts)}, lastPolled)
			}
			return code
		}
	}
}

// failed reports a gh failure and says whether to give up. Rate limits and
// repeated failures end the watch; nothing is retried silently.
func (w *watcher) failed(err error, inRow *int) (int, bool) {
	*inRow++
	fmt.Fprintf(w.log, "prwatch: poll %d/%d failed: %v\n", *inRow, w.opts.maxErrors, err)
	if errors.Is(err, context.Canceled) || isRateLimit(err) || *inRow >= w.opts.maxErrors {
		fmt.Fprintln(w.out, "error:", err)
		return exitError, true
	}
	return 0, false
}

func (w *watcher) wait(ctx context.Context) (int, bool) {
	left := w.deadline.Sub(w.clock.Now())
	if left <= 0 {
		return exitTimeout, true
	}
	if err := w.clock.Sleep(ctx, min(w.opts.interval, left)); err != nil {
		fmt.Fprintln(w.out, "error:", err)
		return exitError, true
	}
	return 0, false
}

func (w *watcher) note(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	if line != w.progress {
		w.progress = line
		fmt.Fprintf(w.log, "%s %s\n", w.clock.Now().Format("15:04:05"), line)
	}
}

// stable reports whether sig has stayed unchanged for the settle period.
func (w *watcher) stable(sig string, now time.Time) bool {
	if sig != w.signature || w.stableSince.IsZero() {
		w.signature, w.stableSince = sig, now
	}
	return now.Sub(w.stableSince) >= w.opts.settle
}

var failing = map[string]bool{
	"FAILURE": true, "ERROR": true, "TIMED_OUT": true, "CANCELLED": true,
	"ACTION_REQUIRED": true, "STARTUP_FAILURE": true, "STALE": true,
}

// gating returns the checks merging waits on: the required ones, or every
// check when the repository requires none.
func gating(checks []check) []check {
	var required []check
	for _, c := range checks {
		if c.Required {
			required = append(required, c)
		}
	}
	if len(required) > 0 {
		return required
	}
	return checks
}

func pending(c check) bool {
	return c.Status != "COMPLETED" && c.Status != ""
}

// judge returns a verdict once the pull request needs slopnanny, or nil to
// keep waiting.
func (w *watcher) judge(ctx context.Context, p pr, now time.Time) (*verdict, error) {
	if w.head == "" {
		w.head = p.Head
	}
	switch {
	case p.State == "CLOSED":
		return &verdict{state: "closed", code: exitAttention, reasons: []string{"closed without merging"}}, nil
	case p.Head != w.head:
		return &verdict{state: "pushed", code: exitAttention, reasons: []string{fmt.Sprintf("head moved %s -> %s", short(w.head), short(p.Head))}}, nil
	}
	if news := newActivity(p, w.opts.since); len(news) > 0 {
		return &verdict{state: "activity", code: exitAttention, reasons: news}, nil
	}
	gate := gating(p.Checks)
	var failed, waiting []string
	for _, c := range gate {
		switch {
		case pending(c):
			waiting = append(waiting, c.Name)
		case failing[c.Conclusion]:
			failed = append(failed, c.Name)
		}
	}
	if len(failed) > 0 {
		return &verdict{state: "checks-failed", code: exitAttention, reasons: []string{"failed: " + strings.Join(failed, ", ")}}, nil
	}
	if len(waiting) > 0 {
		w.stable("running", now)
		w.note("waiting on %d/%d checks: %s", len(waiting), len(gate), strings.Join(waiting, ", "))
		return nil, nil
	}
	if p.MergeState == "UNKNOWN" && now.Sub(w.start) < w.opts.appear {
		w.note("waiting for GitHub to compute the merge state")
		return nil, nil
	}
	var notes []string
	if len(gate) == 0 {
		if now.Sub(w.start) < w.opts.appear {
			w.note("no checks yet")
			return nil, nil
		}
		notes = append(notes, fmt.Sprintf("no checks appeared within %s", w.opts.appear))
	}
	if !w.stable(checkSignature(p.Checks), now) {
		w.note("checks finished; settling for %s", w.opts.settle)
		return nil, nil
	}
	bots, err := w.workingBots(ctx, p, now)
	if err != nil {
		return nil, err
	}
	if len(bots) > 0 {
		if now.Sub(w.stableSince) < w.opts.botWait {
			w.note("checks settled; waiting on %s", strings.Join(bots, ", "))
			return nil, nil
		}
		notes = append(notes, fmt.Sprintf("still working after %s: %s", w.opts.botWait, strings.Join(bots, ", ")))
	}
	if blockers := blockers(p); len(blockers) > 0 {
		return &verdict{state: "blocked", code: exitAttention, reasons: append(blockers, notes...)}, nil
	}
	return &verdict{state: "ready", code: exitDone, reasons: notes}, nil
}

func checkSignature(checks []check) string {
	parts := make([]string, 0, len(checks))
	for _, c := range checks {
		parts = append(parts, c.Name+"="+c.Status+"/"+c.Conclusion)
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

// newActivity lists reviews and comments from anyone but the viewer posted
// after since.
func newActivity(p pr, since time.Time) []string {
	var news []string
	for _, r := range p.Reviews {
		if r.Author.Login != p.Viewer && r.Submitted.After(since) {
			news = append(news, fmt.Sprintf("%s reviewed: %s", r.Author.Login, r.State))
		}
	}
	for _, c := range p.Comments {
		if c.Author.Login != p.Viewer && c.Created.After(since) {
			news = append(news, c.Author.Login+" commented")
		}
	}
	for _, t := range p.Threads {
		if t.Last.Author.Login != p.Viewer && t.Last.Created.After(since) {
			news = append(news, fmt.Sprintf("%s replied on %s", t.Last.Author.Login, t.Path))
		}
	}
	return dedupe(news)
}

// workingBots lists review bots visibly working on the current head: a review
// request, an eyes reaction, or a review of an earlier head shortly after a
// push.
func (w *watcher) workingBots(ctx context.Context, p pr, now time.Time) ([]string, error) {
	var bots []string
	for _, r := range p.ReviewRequests {
		if r.Bot {
			bots = append(bots, r.Login)
		}
	}
	eyes, err := w.f.BotEyes(ctx, w.opts.repo, w.opts.number)
	if err != nil {
		return nil, err
	}
	bots = append(bots, eyes...)
	if now.Sub(p.HeadCommitted) < w.opts.botWait {
		reviewedHead := map[string]bool{}
		for _, r := range p.Reviews {
			if r.Commit == p.Head {
				reviewedHead[r.Author.Login] = true
			}
		}
		for _, r := range p.Reviews {
			if r.Author.Bot && !reviewedHead[r.Author.Login] {
				bots = append(bots, r.Author.Login)
			}
		}
	}
	return dedupe(bots), nil
}

func blockers(p pr) []string {
	var out []string
	if p.Draft {
		out = append(out, "draft")
	}
	if n := len(p.Threads); n > 0 {
		out = append(out, fmt.Sprintf("%d unresolved thread(s)", n))
	}
	for _, r := range latestReviews(p.Reviews) {
		if r.State == "CHANGES_REQUESTED" {
			out = append(out, r.Author.Login+" requested changes")
		}
	}
	for _, r := range p.ReviewRequests {
		if !r.Bot {
			out = append(out, "review requested from "+r.Login)
		}
	}
	switch p.MergeState {
	case "DIRTY":
		out = append(out, "merge conflicts")
	case "BEHIND":
		out = append(out, "branch is behind its base")
	case "BLOCKED":
		out = append(out, "merge blocked by repository rules (a required review or check)")
	}
	return out
}

// latestReviews keeps each author's last approving, rejecting, or dismissed
// review; comment-only reviews don't change a verdict.
func latestReviews(reviews []review) []review {
	last := map[string]review{}
	var order []string
	for _, r := range reviews {
		if r.State == "COMMENTED" || r.State == "PENDING" {
			continue
		}
		if _, seen := last[r.Author.Login]; !seen {
			order = append(order, r.Author.Login)
		}
		last[r.Author.Login] = r
	}
	out := make([]review, 0, len(order))
	for _, login := range order {
		out = append(out, last[login])
	}
	return out
}

func pendingReasons(p pr, opts options) []string {
	var waiting []string
	for _, c := range gating(p.Checks) {
		if pending(c) {
			waiting = append(waiting, c.Name)
		}
	}
	reasons := []string{"deadline " + opts.timeout.String() + " reached"}
	if len(waiting) > 0 {
		reasons = append(reasons, "still running: "+strings.Join(waiting, ", "))
	}
	return append(reasons, blockers(p)...)
}

// After merge.

var skippedEvents = map[string]bool{"schedule": true, "workflow_dispatch": true, "repository_dispatch": true}

var passing = map[string]bool{"success": true, "skipped": true, "neutral": true}

func (w *watcher) watchRuns(ctx context.Context, p pr) int {
	sha := p.MergeCommit
	if sha == "" {
		sha = p.Head
	}
	w.start = w.clock.Now()
	w.deadline = w.start.Add(w.opts.timeout)
	w.signature, w.stableSince = "", time.Time{}
	errorsInRow := 0
	var runs []workflowRun
	for {
		now := w.clock.Now()
		all, err := w.f.Runs(ctx, w.opts.repo, sha)
		if err != nil {
			if code, stop := w.failed(err, &errorsInRow); stop {
				return code
			}
		} else {
			errorsInRow = 0
			runs = runs[:0]
			for _, r := range all {
				if !skippedEvents[r.Event] {
					runs = append(runs, r)
				}
			}
			if code, done := w.judgeRuns(ctx, p, sha, runs, now); done {
				return code
			}
		}
		if code, stop := w.wait(ctx); stop {
			if code == exitTimeout {
				w.reportRuns(p, sha, "timeout", runs, nil)
			}
			return code
		}
	}
}

func (w *watcher) judgeRuns(ctx context.Context, p pr, sha string, runs []workflowRun, now time.Time) (int, bool) {
	if len(runs) == 0 {
		if now.Sub(w.start) < w.opts.appear {
			w.note("merged as %s; no runs yet", short(sha))
			return 0, false
		}
		w.reportRuns(p, sha, "no-runs", nil, nil)
		return exitDone, true
	}
	var running, failed []string
	parts := make([]string, 0, len(runs))
	for _, r := range runs {
		parts = append(parts, fmt.Sprintf("%d=%s/%s", r.ID, r.Status, r.Conclusion))
		if r.Status != "completed" {
			running = append(running, r.Name)
		} else if !passing[r.Conclusion] {
			failed = append(failed, r.Name)
		}
	}
	sort.Strings(parts)
	if len(running) > 0 {
		w.stable("running", now)
		w.note("merged as %s; running: %s", short(sha), strings.Join(running, ", "))
		return 0, false
	}
	if !w.stable(strings.Join(parts, ";"), now) {
		w.note("runs finished; settling for %s in case they trigger more", w.opts.settle)
		return 0, false
	}
	if len(failed) == 0 {
		w.reportRuns(p, sha, "runs-passed", runs, nil)
		return exitDone, true
	}
	jobs := map[int64][]job{}
	for _, r := range runs {
		if r.Status == "completed" && !passing[r.Conclusion] {
			js, err := w.f.FailedJobs(ctx, w.opts.repo, r.ID)
			if err != nil {
				fmt.Fprintf(w.log, "prwatch: jobs of run %d: %v\n", r.ID, err)
			}
			jobs[r.ID] = js
		}
	}
	w.reportRuns(p, sha, "runs-failed", runs, jobs)
	return exitAttention, true
}

// Output.

func (w *watcher) header(state string, p pr, at string) {
	fmt.Fprintf(w.out, "%s: %s#%d %s (%s)\n", state, w.opts.repo, w.opts.number, at, w.clock.Now().Sub(w.start).Round(time.Second))
	fmt.Fprintln(w.out, p.URL)
}

func (w *watcher) report(p pr, v verdict, polled time.Time) {
	w.header(v.state, p, "head "+short(p.Head))
	for _, r := range v.reasons {
		fmt.Fprintln(w.out, "  -", r)
	}
	var failedChecks, waitingChecks []string
	for _, c := range p.Checks {
		label := c.Name
		if c.Required {
			label += " (required)"
		}
		switch {
		case pending(c):
			waitingChecks = append(waitingChecks, label)
		case failing[c.Conclusion]:
			failedChecks = append(failedChecks, fmt.Sprintf("%s %s %s", label, strings.ToLower(c.Conclusion), c.URL))
		}
	}
	section(w.out, "failed checks", failedChecks)
	section(w.out, "running checks", waitingChecks)
	var requests []string
	for _, r := range p.ReviewRequests {
		requests = append(requests, r.Login)
	}
	section(w.out, "review requests", requests)
	var reviews []string
	for _, r := range latestReviews(p.Reviews) {
		reviews = append(reviews, fmt.Sprintf("%s %s on %s %s", r.Author.Login, r.State, short(r.Commit), r.URL))
	}
	section(w.out, "reviews", reviews)
	var threads []string
	for _, t := range p.Threads {
		where := t.Path
		if t.Line > 0 {
			where += ":" + strconv.Itoa(t.Line)
		}
		if t.Outdated {
			where += " (outdated)"
		}
		threads = append(threads, fmt.Sprintf("%s %s: %s %s", where, t.Last.Author.Login, firstLine(t.Last.Body), t.Last.URL))
	}
	section(w.out, "unresolved threads", threads)
	var news []string
	for _, r := range p.Reviews {
		if r.Author.Login != p.Viewer && r.Submitted.After(w.opts.since) && strings.TrimSpace(r.Body) != "" {
			news = append(news, fmt.Sprintf("%s review: %s %s", r.Author.Login, firstLine(r.Body), r.URL))
		}
	}
	for _, c := range p.Comments {
		if c.Author.Login != p.Viewer && c.Created.After(w.opts.since) {
			news = append(news, fmt.Sprintf("%s: %s %s", c.Author.Login, firstLine(c.Body), c.URL))
		}
	}
	section(w.out, "new since "+w.opts.since.UTC().Format(time.RFC3339), news)
	fmt.Fprintln(w.out, "since:", polled.UTC().Format(time.RFC3339))
}

func (w *watcher) reportRuns(p pr, sha, state string, runs []workflowRun, jobs map[int64][]job) {
	w.header(state, p, "merged as "+sha)
	var lines []string
	for _, r := range runs {
		status := r.Conclusion
		if r.Status != "completed" {
			status = r.Status
		}
		lines = append(lines, fmt.Sprintf("%s (%s) %s %s", r.Name, r.Event, status, r.URL))
		for _, j := range jobs[r.ID] {
			lines = append(lines, fmt.Sprintf("  job %s %s %s", j.Name, j.Conclusion, j.URL))
		}
	}
	section(w.out, "runs", lines)
	if state == "no-runs" {
		fmt.Fprintf(w.out, "  - no runs started for %s within %s\n", short(sha), w.opts.appear)
	}
}

func section(out io.Writer, title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(out, "%s:\n", title)
	for _, l := range lines {
		fmt.Fprintln(out, "  "+l)
	}
}

func firstLine(body string) string {
	for _, l := range strings.Split(body, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			if r := []rune(l); len(r) > 120 {
				return string(r[:119]) + "…"
			}
			return l
		}
	}
	return ""
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func dedupe(items []string) []string {
	seen := map[string]bool{}
	out := items[:0]
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

// gh.

type ghForge struct{}

type ghError struct {
	args   []string
	stderr string
	err    error
}

func (e *ghError) Error() string {
	msg := strings.TrimSpace(e.stderr)
	if msg == "" {
		msg = e.err.Error()
	}
	return fmt.Sprintf("gh %s: %s", strings.Join(e.args[:min(2, len(e.args))], " "), msg)
}

func (e *ghError) Unwrap() error { return e.err }

func isRateLimit(err error) bool {
	var ge *ghError
	return errors.As(err, &ge) && strings.Contains(strings.ToLower(ge.stderr), "rate limit")
}

func gh(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return nil, &ghError{args: args, stderr: stderr.String(), err: err}
	}
	return stdout.Bytes(), nil
}

func (ghForge) CurrentRepo(ctx context.Context) (string, error) {
	out, err := gh(ctx, "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	return strings.TrimSpace(string(out)), err
}

const prQuery = `query($owner: String!, $repo: String!, $number: Int!, $checks: String, $threads: String) {
  viewer { login }
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      url state isDraft mergeStateStatus headRefOid
      mergeCommit { oid }
      commits(last: 1) { nodes { commit { committedDate statusCheckRollup {
        contexts(first: 100, after: $checks) {
          pageInfo { hasNextPage endCursor }
          nodes {
            __typename
            ... on CheckRun { name status conclusion detailsUrl isRequired(pullRequestNumber: $number) }
            ... on StatusContext { context state targetUrl isRequired(pullRequestNumber: $number) }
          }
        }
      } } } }
      reviewRequests(first: 50) { nodes { requestedReviewer {
        __typename ... on User { login } ... on Bot { login } ... on Team { slug } ... on Mannequin { login }
      } } }
      reviews(last: 100) { nodes { author { __typename login } state submittedAt url body commit { oid } } }
      comments(last: 100) { nodes { author { __typename login } createdAt url body } }
      reviewThreads(first: 100, after: $threads) {
        pageInfo { hasNextPage endCursor }
        nodes { isResolved isOutdated path line originalLine
          comments(last: 1) { nodes { author { __typename login } createdAt url body } } }
      }
    }
  }
}`

type gqlAuthor struct {
	Typename string `json:"__typename"`
	Login    string `json:"login"`
}

func (a *gqlAuthor) actor() actor {
	if a == nil {
		return actor{Login: "ghost"}
	}
	return actor{Login: a.Login, Bot: a.Typename == "Bot"}
}

type gqlComment struct {
	Author    *gqlAuthor `json:"author"`
	CreatedAt time.Time  `json:"createdAt"`
	URL       string     `json:"url"`
	Body      string     `json:"body"`
}

func (c gqlComment) comment() comment {
	return comment{Author: c.Author.actor(), Created: c.CreatedAt, URL: c.URL, Body: c.Body}
}

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type prResponse struct {
	Data struct {
		Viewer     struct{ Login string } `json:"viewer"`
		Repository struct {
			PullRequest *struct {
				URL              string                `json:"url"`
				State            string                `json:"state"`
				IsDraft          bool                  `json:"isDraft"`
				MergeStateStatus string                `json:"mergeStateStatus"`
				HeadRefOid       string                `json:"headRefOid"`
				MergeCommit      *struct{ Oid string } `json:"mergeCommit"`
				Commits          struct {
					Nodes []struct {
						Commit struct {
							CommittedDate     time.Time `json:"committedDate"`
							StatusCheckRollup *struct {
								Contexts struct {
									PageInfo pageInfo `json:"pageInfo"`
									Nodes    []struct {
										Typename   string `json:"__typename"`
										Name       string `json:"name"`
										Status     string `json:"status"`
										Conclusion string `json:"conclusion"`
										DetailsURL string `json:"detailsUrl"`
										Context    string `json:"context"`
										State      string `json:"state"`
										TargetURL  string `json:"targetUrl"`
										IsRequired bool   `json:"isRequired"`
									} `json:"nodes"`
								} `json:"contexts"`
							} `json:"statusCheckRollup"`
						} `json:"commit"`
					} `json:"nodes"`
				} `json:"commits"`
				ReviewRequests struct {
					Nodes []struct {
						RequestedReviewer *struct {
							Typename string `json:"__typename"`
							Login    string `json:"login"`
							Slug     string `json:"slug"`
						} `json:"requestedReviewer"`
					} `json:"nodes"`
				} `json:"reviewRequests"`
				Reviews struct {
					Nodes []struct {
						Author      *gqlAuthor            `json:"author"`
						State       string                `json:"state"`
						SubmittedAt time.Time             `json:"submittedAt"`
						URL         string                `json:"url"`
						Body        string                `json:"body"`
						Commit      *struct{ Oid string } `json:"commit"`
					} `json:"nodes"`
				} `json:"reviews"`
				Comments struct {
					Nodes []gqlComment `json:"nodes"`
				} `json:"comments"`
				ReviewThreads struct {
					PageInfo pageInfo `json:"pageInfo"`
					Nodes    []struct {
						IsResolved   bool   `json:"isResolved"`
						IsOutdated   bool   `json:"isOutdated"`
						Path         string `json:"path"`
						Line         int    `json:"line"`
						OriginalLine int    `json:"originalLine"`
						Comments     struct {
							Nodes []gqlComment `json:"nodes"`
						} `json:"comments"`
					} `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct{ Message string } `json:"errors"`
}

func (ghForge) PullRequest(ctx context.Context, repo string, number int) (pr, error) {
	owner, name, _ := strings.Cut(repo, "/")
	var p pr
	var checksAfter, threadsAfter string
	first := true
	for {
		args := []string{"api", "graphql", "-f", "query=" + prQuery, "-F", "owner=" + owner, "-F", "repo=" + name, "-F", "number=" + strconv.Itoa(number)}
		if checksAfter != "" {
			args = append(args, "-f", "checks="+checksAfter)
		}
		if threadsAfter != "" {
			args = append(args, "-f", "threads="+threadsAfter)
		}
		out, err := gh(ctx, args...)
		if err != nil {
			return pr{}, err
		}
		var resp prResponse
		if err := json.Unmarshal(out, &resp); err != nil {
			return pr{}, fmt.Errorf("decode pull request: %w", err)
		}
		if len(resp.Errors) > 0 {
			return pr{}, fmt.Errorf("graphql: %s", resp.Errors[0].Message)
		}
		raw := resp.Data.Repository.PullRequest
		if raw == nil {
			return pr{}, fmt.Errorf("%s#%d not found", repo, number)
		}
		if first {
			p = pr{Viewer: resp.Data.Viewer.Login, URL: raw.URL, State: raw.State, Draft: raw.IsDraft,
				MergeState: raw.MergeStateStatus, Head: raw.HeadRefOid}
			if raw.MergeCommit != nil {
				p.MergeCommit = raw.MergeCommit.Oid
			}
			for _, n := range raw.ReviewRequests.Nodes {
				if r := n.RequestedReviewer; r != nil {
					login := r.Login
					if login == "" {
						login = r.Slug
					}
					p.ReviewRequests = append(p.ReviewRequests, actor{Login: login, Bot: r.Typename == "Bot"})
				}
			}
			for _, r := range raw.Reviews.Nodes {
				rv := review{Author: r.Author.actor(), State: r.State, Submitted: r.SubmittedAt, URL: r.URL, Body: r.Body}
				if r.Commit != nil {
					rv.Commit = r.Commit.Oid
				}
				p.Reviews = append(p.Reviews, rv)
			}
			for _, c := range raw.Comments.Nodes {
				p.Comments = append(p.Comments, c.comment())
			}
		}
		checksNext, threadsNext := false, false
		if (first || checksAfter != "") && len(raw.Commits.Nodes) > 0 {
			commit := raw.Commits.Nodes[0].Commit
			p.HeadCommitted = commit.CommittedDate
			if rollup := commit.StatusCheckRollup; rollup != nil {
				for _, n := range rollup.Contexts.Nodes {
					if n.Typename == "StatusContext" {
						status := "COMPLETED"
						if n.State == "PENDING" || n.State == "EXPECTED" {
							status = "PENDING"
						}
						p.Checks = append(p.Checks, check{Name: n.Context, Status: status, Conclusion: n.State, URL: n.TargetURL, Required: n.IsRequired})
					} else {
						p.Checks = append(p.Checks, check{Name: n.Name, Status: n.Status, Conclusion: n.Conclusion, URL: n.DetailsURL, Required: n.IsRequired})
					}
				}
				checksNext = rollup.Contexts.PageInfo.HasNextPage
				checksAfter = rollup.Contexts.PageInfo.EndCursor
			}
		}
		if first || threadsAfter != "" {
			for _, t := range raw.ReviewThreads.Nodes {
				if t.IsResolved || len(t.Comments.Nodes) == 0 {
					continue
				}
				line := t.Line
				if line == 0 {
					line = t.OriginalLine
				}
				p.Threads = append(p.Threads, thread{Path: t.Path, Line: line, Outdated: t.IsOutdated, Last: t.Comments.Nodes[0].comment()})
			}
			threadsNext = raw.ReviewThreads.PageInfo.HasNextPage
			threadsAfter = raw.ReviewThreads.PageInfo.EndCursor
		}
		if !checksNext {
			checksAfter = ""
		}
		if !threadsNext {
			threadsAfter = ""
		}
		if checksAfter == "" && threadsAfter == "" {
			return p, nil
		}
		first = false
	}
}

func (ghForge) BotEyes(ctx context.Context, repo string, number int) ([]string, error) {
	out, err := gh(ctx, "api", "--paginate", fmt.Sprintf("repos/%s/issues/%d/reactions?content=eyes&per_page=100", repo, number),
		"--jq", `.[] | select(.user.type == "Bot") | .user.login`)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

func (ghForge) Runs(ctx context.Context, repo, sha string) ([]workflowRun, error) {
	out, err := gh(ctx, "api", "--paginate", fmt.Sprintf("repos/%s/actions/runs?head_sha=%s&per_page=100", repo, sha),
		"--jq", `.workflow_runs[] | {id, name, event, status, conclusion: (.conclusion // ""), url: .html_url}`)
	if err != nil {
		return nil, err
	}
	var runs []workflowRun
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var r struct {
			ID                                   int64
			Name, Event, Status, Conclusion, URL string
		}
		if err := dec.Decode(&r); err != nil {
			return nil, fmt.Errorf("decode runs: %w", err)
		}
		runs = append(runs, workflowRun(r))
	}
	return runs, nil
}

func (ghForge) FailedJobs(ctx context.Context, repo string, runID int64) ([]job, error) {
	out, err := gh(ctx, "api", "--paginate", fmt.Sprintf("repos/%s/actions/runs/%d/jobs?filter=latest&per_page=100", repo, runID),
		"--jq", `.jobs[] | select(.conclusion != "success" and .conclusion != "skipped" and .conclusion != null) | {name, conclusion, url: .html_url}`)
	if err != nil {
		return nil, err
	}
	var jobs []job
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var j job
		if err := dec.Decode(&j); err != nil {
			return nil, fmt.Errorf("decode jobs: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}
