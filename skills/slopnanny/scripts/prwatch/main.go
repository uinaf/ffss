// Command prwatch waits on a GitHub pull request or GitLab merge request until
// slopnanny has something to act on, then prints what it saw and exits. Run it
// where `gh` or `glab` is signed in:
//
//	go run <skill dir>/scripts/prwatch/main.go [flags] <number | url>
//
// An open change request is watched until its checks fail, someone else posts
// a review or comment, its head moves, it merges or closes, or its checks
// settle and it is ready or blocked. A merged one has the runs or pipelines
// its merge commit started watched until they finish.
//
// Exit codes: 0 ready, merged, or runs passed; 1 needs attention; 2 usage or
// forge CLI failure; 3 deadline reached.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
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
	kind, host, repo          string
	number                    int
	timeout, interval, settle time.Duration
	appear, botWait           time.Duration
	since                     time.Time
	maxErrors                 int
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], deps{forge: newForge, origin: originURL, clock: realClock{}}, os.Stdout, os.Stderr))
}

type deps struct {
	forge  func(kind, host string) forge
	origin func(ctx context.Context) (string, error)
	clock  clock
}

func newForge(kind, host string) forge {
	if kind == "gitlab" {
		return &glabForge{host: host}
	}
	return ghForge{host: host}
}

func originURL(ctx context.Context) (string, error) {
	out, err := cli(ctx, "git", "remote", "get-url", "origin")
	return strings.TrimSpace(string(out)), err
}

func run(ctx context.Context, args []string, d deps, stdout, stderr io.Writer) int {
	clock := d.clock
	flags := flag.NewFlagSet("prwatch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var opts options
	var since string
	flags.StringVar(&opts.repo, "R", "", "owner/repo or GitLab group/project (default: the URL's, or the origin remote's)")
	flags.StringVar(&opts.kind, "forge", "", "github or gitlab (default: from the URL or the origin remote's host)")
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
	if opts.kind != "" && opts.kind != "github" && opts.kind != "gitlab" {
		fmt.Fprintln(stderr, "prwatch: -forge must be github or gitlab")
		return exitError
	}
	if err := parseTarget(flags.Arg(0), &opts); err != nil {
		fmt.Fprintln(stderr, "prwatch:", err)
		return exitError
	}
	opts.since = clock.Now().Truncate(time.Second)
	if since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			fmt.Fprintln(stderr, "prwatch: -since:", err)
			return exitError
		}
		opts.since = t
	}
	if opts.repo == "" || opts.host == "" {
		remote, err := d.origin(ctx)
		host, repo, ok := parseRemote(remote)
		switch {
		case ok:
			if opts.repo == "" {
				opts.repo = repo
			}
			if opts.host == "" {
				opts.host = host
			}
		case opts.repo == "":
			fmt.Fprintln(stderr, "prwatch: pass -R or a URL; no origin remote to read:", err)
			return exitError
		}
	}
	if opts.kind == "" {
		opts.kind = forgeOf(opts.host)
		if opts.kind == "" {
			fmt.Fprintf(stderr, "prwatch: can't tell which forge %s is; pass -forge\n", opts.host)
			return exitError
		}
	}
	w := &watcher{f: d.forge(opts.kind, opts.host), clock: clock, opts: opts, out: stdout, log: stderr}
	return w.watch(ctx)
}

var (
	githubURL = regexp.MustCompile(`^https?://([^/]+)/([^/]+/[^/]+)/pull/(\d+)`)
	gitlabURL = regexp.MustCompile(`^https?://([^/]+)/(.+?)/-/merge_requests/(\d+)`)
	remoteURL = regexp.MustCompile(`^(?:[a-z+]+://)?(?:[^@/]+@)?([^/:]+)(?::\d+)?[:/](.+?)(?:\.git)?/?$`)
)

func parseTarget(arg string, opts *options) error {
	for kind, re := range map[string]*regexp.Regexp{"github": githubURL, "gitlab": gitlabURL} {
		if m := re.FindStringSubmatch(arg); m != nil {
			if opts.kind != "" && opts.kind != kind {
				return fmt.Errorf("-forge %s contradicts the %s URL", opts.kind, kind)
			}
			opts.kind = kind
			if opts.repo == "" {
				opts.repo = m[2]
			}
			opts.host, arg = m[1], m[3]
		}
	}
	n, err := strconv.Atoi(strings.TrimLeft(arg, "#!"))
	if err != nil || n <= 0 {
		return fmt.Errorf("want a change request number or URL, got %q", arg)
	}
	opts.number = n
	return nil
}

func parseRemote(remote string) (host, repo string, ok bool) {
	m := remoteURL.FindStringSubmatch(strings.TrimSpace(remote))
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

func forgeOf(host string) string {
	switch {
	case host == "" || strings.Contains(host, "github"):
		return "github"
	case strings.Contains(host, "gitlab"):
		return "gitlab"
	}
	return ""
}

// Forge data, as prwatch needs it.

type pr struct {
	Viewer, URL, State, MergeState string
	// MergeDetail is the forge's own reason a merge is blocked, when it says.
	MergeDetail       string
	Base              string
	Draft             bool
	Head, MergeCommit string
	HeadCommitted     time.Time
	Checks            []check
	ReviewRequests    []actor
	// Reviews are review events, for activity; Verdicts are each reviewer's
	// standing approval or change request.
	Reviews  []review
	Verdicts []review
	Comments []comment
	// ThreadComments holds every recent comment in every review thread,
	// resolved or not; Threads only the unresolved ones.
	ThreadComments []comment
	Threads        []thread
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
	PullRequest(ctx context.Context, repo string, number int) (pr, error)
	BotEyes(ctx context.Context, repo string, number int) ([]string, error)
	Runs(ctx context.Context, repo, sha, branch string) ([]workflowRun, error)
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
			last, lastPolled = p, polled
			if p.State == "MERGED" {
				return w.watchRuns(ctx, p)
			}
			v, err = w.judge(ctx, p, polled)
			if err == nil {
				errorsInRow = 0
				if v != nil {
					w.report(p, *v, polled)
					return v.code
				}
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

// failed reports a forge CLI failure and says whether to give up. Rate limits and
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
	if p.MergeState == "UNKNOWN" {
		w.note("waiting for the forge to compute the merge state")
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

// since counts an event at exactly the since second as new: forges stamp
// events to the second, and the printed since line is truncated to it.
func isNew(at, since time.Time) bool { return !at.Before(since) }

// newActivity lists reviews and comments, threaded or not, resolved or not,
// from anyone but the viewer at or after since.
func newActivity(p pr, since time.Time) []string {
	var news []string
	for _, r := range p.Reviews {
		if r.Author.Login != p.Viewer && isNew(r.Submitted, since) {
			news = append(news, fmt.Sprintf("%s reviewed: %s", r.Author.Login, r.State))
		}
	}
	for _, c := range p.Comments {
		if c.Author.Login != p.Viewer && isNew(c.Created, since) {
			news = append(news, c.Author.Login+" commented")
		}
	}
	for _, c := range p.ThreadComments {
		if c.Author.Login != p.Viewer && isNew(c.Created, since) {
			news = append(news, c.Author.Login+" commented in a thread")
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
	for _, r := range p.Verdicts {
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
		if p.MergeDetail != "" {
			out = append(out, "merge blocked: "+p.MergeDetail)
		} else {
			out = append(out, "merge blocked by repository rules (a required review or check)")
		}
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
	if p.MergeState == "UNKNOWN" {
		reasons = append(reasons, "merge state still unknown")
	}
	return append(reasons, blockers(p)...)
}

// After merge.

var skippedEvents = map[string]bool{"schedule": true, "workflow_dispatch": true}

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
		all, err := w.f.Runs(ctx, w.opts.repo, sha, p.Base)
		if err != nil {
			if code, stop := w.failed(err, &errorsInRow); stop {
				return code
			}
		} else {
			runs = runs[:0]
			for _, r := range all {
				if !skippedEvents[r.Event] {
					runs = append(runs, r)
				}
			}
			code, done, err := w.judgeRuns(ctx, p, sha, runs, now)
			if err != nil {
				if code, stop := w.failed(err, &errorsInRow); stop {
					return code
				}
			} else if errorsInRow = 0; done {
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

func (w *watcher) judgeRuns(ctx context.Context, p pr, sha string, runs []workflowRun, now time.Time) (int, bool, error) {
	if len(runs) == 0 {
		if now.Sub(w.start) < w.opts.appear {
			w.note("merged as %s; no runs yet", short(sha))
			return 0, false, nil
		}
		w.reportRuns(p, sha, "no-runs", nil, nil)
		return exitDone, true, nil
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
		return 0, false, nil
	}
	if !w.stable(strings.Join(parts, ";"), now) {
		w.note("runs finished; settling for %s in case they trigger more", w.opts.settle)
		return 0, false, nil
	}
	if len(failed) == 0 {
		w.reportRuns(p, sha, "runs-passed", runs, nil)
		return exitDone, true, nil
	}
	jobs := map[int64][]job{}
	for _, r := range runs {
		if r.Status == "completed" && !passing[r.Conclusion] {
			js, err := w.f.FailedJobs(ctx, w.opts.repo, r.ID)
			if err != nil {
				return 0, false, err
			}
			jobs[r.ID] = js
		}
	}
	w.reportRuns(p, sha, "runs-failed", runs, jobs)
	return exitAttention, true, nil
}

// Output.

func (w *watcher) header(state string, p pr, at string) {
	sep := "#"
	if w.opts.kind == "gitlab" {
		sep = "!"
	}
	fmt.Fprintf(w.out, "%s: %s%s%d %s (%s)\n", state, w.opts.repo, sep, w.opts.number, at, w.clock.Now().Sub(w.start).Round(time.Second))
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
	for _, r := range p.Verdicts {
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
		if r.Author.Login != p.Viewer && isNew(r.Submitted, w.opts.since) && strings.TrimSpace(r.Body) != "" {
			news = append(news, fmt.Sprintf("%s review: %s %s", r.Author.Login, firstLine(r.Body), r.URL))
		}
	}
	for _, c := range append(append([]comment{}, p.Comments...), p.ThreadComments...) {
		if c.Author.Login != p.Viewer && isNew(c.Created, w.opts.since) {
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

// Forge CLIs.

type cliError struct {
	name   string
	args   []string
	stderr string
	err    error
}

func (e *cliError) Error() string {
	msg := strings.TrimSpace(e.stderr)
	if msg == "" {
		msg = e.err.Error()
	}
	return fmt.Sprintf("%s %s: %s", e.name, strings.Join(e.args[:min(2, len(e.args))], " "), msg)
}

func (e *cliError) Unwrap() error { return e.err }

func isRateLimit(err error) bool {
	var ce *cliError
	if !errors.As(err, &ce) {
		return false
	}
	msg := strings.ToLower(ce.stderr)
	return strings.Contains(msg, "rate limit") || strings.Contains(msg, "429")
}

// cliTimeout bounds one forge CLI call so a hung process can't outlive the
// watch deadline by more than this.
const cliTimeout = 2 * time.Minute

func cli(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return nil, &cliError{name: name, args: args, stderr: stderr.String(), err: err}
	}
	return stdout.Bytes(), nil
}

// GitHub.

type ghForge struct{ host string }

func (g ghForge) call(ctx context.Context, args ...string) ([]byte, error) {
	if g.host != "" {
		args = append(args, "--hostname", g.host)
	}
	return cli(ctx, "gh", args...)
}

const prQuery = `query($owner: String!, $repo: String!, $number: Int!, $checks: String, $threads: String) {
  viewer { login }
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      url state isDraft mergeStateStatus headRefOid baseRefName
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
      latestOpinionatedReviews(first: 100) { nodes { author { __typename login } state submittedAt url body commit { oid } } }
      comments(last: 100) { nodes { author { __typename login } createdAt url body } }
      reviewThreads(first: 100, after: $threads) {
        pageInfo { hasNextPage endCursor }
        nodes { isResolved isOutdated path line originalLine
          comments(last: 50) { nodes { author { __typename login } createdAt url body } } }
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

type gqlReview struct {
	Author      *gqlAuthor            `json:"author"`
	State       string                `json:"state"`
	SubmittedAt time.Time             `json:"submittedAt"`
	URL         string                `json:"url"`
	Body        string                `json:"body"`
	Commit      *struct{ Oid string } `json:"commit"`
}

func (r gqlReview) review() review {
	rv := review{Author: r.Author.actor(), State: r.State, Submitted: r.SubmittedAt, URL: r.URL, Body: r.Body}
	if r.Commit != nil {
		rv.Commit = r.Commit.Oid
	}
	return rv
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
				BaseRefName      string                `json:"baseRefName"`
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
					Nodes []gqlReview `json:"nodes"`
				} `json:"reviews"`
				LatestOpinionatedReviews struct {
					Nodes []gqlReview `json:"nodes"`
				} `json:"latestOpinionatedReviews"`
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

func (g ghForge) PullRequest(ctx context.Context, repo string, number int) (pr, error) {
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
		out, err := g.call(ctx, args...)
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
				MergeState: raw.MergeStateStatus, Head: raw.HeadRefOid, Base: raw.BaseRefName}
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
				p.Reviews = append(p.Reviews, r.review())
			}
			for _, r := range raw.LatestOpinionatedReviews.Nodes {
				if r.State == "CHANGES_REQUESTED" || r.State == "APPROVED" {
					p.Verdicts = append(p.Verdicts, r.review())
				}
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
				for _, c := range t.Comments.Nodes {
					p.ThreadComments = append(p.ThreadComments, c.comment())
				}
				if t.IsResolved || len(t.Comments.Nodes) == 0 {
					continue
				}
				line := t.Line
				if line == 0 {
					line = t.OriginalLine
				}
				p.Threads = append(p.Threads, thread{Path: t.Path, Line: line, Outdated: t.IsOutdated, Last: t.Comments.Nodes[len(t.Comments.Nodes)-1].comment()})
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

func (g ghForge) BotEyes(ctx context.Context, repo string, number int) ([]string, error) {
	out, err := g.call(ctx, "api", "--paginate", fmt.Sprintf("repos/%s/issues/%d/reactions?content=eyes&per_page=100", repo, number),
		"--jq", `.[] | select(.user.type == "Bot") | .user.login`)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

func (g ghForge) Runs(ctx context.Context, repo, sha, _ string) ([]workflowRun, error) {
	out, err := g.call(ctx, "api", "--paginate", fmt.Sprintf("repos/%s/actions/runs?head_sha=%s&per_page=100", repo, sha),
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

func (g ghForge) FailedJobs(ctx context.Context, repo string, runID int64) ([]job, error) {
	out, err := g.call(ctx, "api", "--paginate", fmt.Sprintf("repos/%s/actions/runs/%d/jobs?filter=latest&per_page=100", repo, runID),
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

// GitLab.

type glabForge struct {
	host   string
	viewer string
}

func (g *glabForge) get(ctx context.Context, path string, out any) error {
	args := []string{"api", path}
	if g.host != "" {
		args = append(args, "--hostname", g.host)
	}
	data, err := cli(ctx, "glab", args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// list reads every page of a GitLab list endpoint; --paginate prints one JSON
// array per page.
func list[T any](ctx context.Context, g *glabForge, path string) ([]T, error) {
	args := []string{"api", "--paginate", path}
	if g.host != "" {
		args = append(args, "--hostname", g.host)
	}
	data, err := cli(ctx, "glab", args...)
	if err != nil {
		return nil, err
	}
	var all []T
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var page []T
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		all = append(all, page...)
	}
	return all, nil
}

func project(repo string) string { return "projects/" + url.PathEscape(repo) }

type glUser struct {
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

var botName = regexp.MustCompile(`(?i)(^|[-_\[])bot($|[-_\]\d])|^gitlabduo$`)

func (u glUser) actor() actor {
	return actor{Login: u.Username, Bot: u.Bot || botName.MatchString(u.Username)}
}

type glNote struct {
	ID         int64     `json:"id"`
	Body       string    `json:"body"`
	Author     glUser    `json:"author"`
	CreatedAt  time.Time `json:"created_at"`
	System     bool      `json:"system"`
	Resolvable bool      `json:"resolvable"`
	Resolved   bool      `json:"resolved"`
	Position   *struct {
		NewPath string `json:"new_path"`
		NewLine int    `json:"new_line"`
		OldPath string `json:"old_path"`
		OldLine int    `json:"old_line"`
	} `json:"position"`
}

// glStatus maps a GitLab job or pipeline status onto GitHub check vocabulary.
// Manual jobs never start on their own: an optional one counts as finished,
// a blocking one (or a pipeline waiting on one) needs someone to act.
func glStatus(status string, optional bool) (state, conclusion string) {
	switch status {
	case "success":
		return "COMPLETED", "SUCCESS"
	case "failed":
		return "COMPLETED", "FAILURE"
	case "canceled", "canceling":
		return "COMPLETED", "CANCELLED"
	case "skipped":
		return "COMPLETED", "SKIPPED"
	case "manual":
		if optional {
			return "COMPLETED", "NEUTRAL"
		}
		return "COMPLETED", "ACTION_REQUIRED"
	}
	return "IN_PROGRESS", ""
}

// Approving or requesting changes without a comment leaves only a system note.
var glReviewNote = regexp.MustCompile(`(?i)^(approved|unapproved|requested changes)\b`)

var glReviewStates = map[string]string{"approved": "APPROVED", "unapproved": "DISMISSED", "requested changes": "CHANGES_REQUESTED"}

var glMergeStates = map[string]string{
	"unchecked": "UNKNOWN", "checking": "UNKNOWN", "preparing": "UNKNOWN", "approvals_syncing": "UNKNOWN",
	"ci_still_running": "UNKNOWN",
	"mergeable":        "CLEAN", "draft_status": "CLEAN",
	"discussions_not_resolved": "CLEAN", "requested_changes": "CLEAN",
	"conflict": "DIRTY", "broken_status": "DIRTY", "need_rebase": "BEHIND",
}

func (g *glabForge) PullRequest(ctx context.Context, repo string, number int) (pr, error) {
	base := fmt.Sprintf("%s/merge_requests/%d", project(repo), number)
	if g.viewer == "" {
		var me glUser
		if err := g.get(ctx, "user", &me); err != nil {
			return pr{}, err
		}
		g.viewer = me.Username
	}
	var mr struct {
		State               string `json:"state"`
		Draft               bool   `json:"draft"`
		WebURL              string `json:"web_url"`
		SHA                 string `json:"sha"`
		MergeCommitSHA      string `json:"merge_commit_sha"`
		SquashCommitSHA     string `json:"squash_commit_sha"`
		TargetBranch        string `json:"target_branch"`
		DetailedMergeStatus string `json:"detailed_merge_status"`
		// GitLab only reports a head pipeline that ran for the current head.
		HeadPipeline *struct {
			ID        int64  `json:"id"`
			ProjectID int64  `json:"project_id"`
			Status    string `json:"status"`
			WebURL    string `json:"web_url"`
		} `json:"head_pipeline"`
	}
	if err := g.get(ctx, base, &mr); err != nil {
		return pr{}, err
	}
	p := pr{Viewer: g.viewer, URL: mr.WebURL, Draft: mr.Draft, Head: mr.SHA, Base: mr.TargetBranch}
	p.State = map[string]string{"opened": "OPEN", "merged": "MERGED"}[mr.State]
	if p.State == "" {
		p.State = "CLOSED"
	}
	p.MergeCommit = mr.MergeCommitSHA
	if p.MergeCommit == "" {
		p.MergeCommit = mr.SquashCommitSHA
	}
	if p.MergeState = glMergeStates[mr.DetailedMergeStatus]; p.MergeState == "" {
		p.MergeState, p.MergeDetail = "BLOCKED", mr.DetailedMergeStatus
	}
	if p.State != "OPEN" {
		return p, nil
	}

	if hp := mr.HeadPipeline; hp != nil {
		// A fork's merge request runs its pipeline in the fork.
		jobs, err := list[struct {
			Name         string `json:"name"`
			Status       string `json:"status"`
			AllowFailure bool   `json:"allow_failure"`
			WebURL       string `json:"web_url"`
		}](ctx, g, fmt.Sprintf("projects/%d/pipelines/%d/jobs?per_page=100", hp.ProjectID, hp.ID))
		if err != nil {
			return pr{}, err
		}
		for _, j := range jobs {
			state, conclusion := glStatus(j.Status, j.AllowFailure)
			p.Checks = append(p.Checks, check{Name: j.Name, Status: state, Conclusion: conclusion, URL: j.WebURL, Required: !j.AllowFailure})
		}
		// The pipeline's own status also covers bridge jobs and child pipelines.
		state, conclusion := glStatus(hp.Status, false)
		p.Checks = append(p.Checks, check{Name: fmt.Sprintf("pipeline %d", hp.ID), Status: state, Conclusion: conclusion, URL: hp.WebURL, Required: true})
	}

	reviewers, err := list[struct {
		User  glUser `json:"user"`
		State string `json:"state"`
	}](ctx, g, base+"/reviewers")
	if err != nil {
		return pr{}, err
	}
	for _, r := range reviewers {
		switch r.State {
		case "requested_changes":
			p.Verdicts = append(p.Verdicts, review{Author: r.User.actor(), State: "CHANGES_REQUESTED"})
		case "approved":
			p.Verdicts = append(p.Verdicts, review{Author: r.User.actor(), State: "APPROVED"})
		case "unreviewed", "review_started":
			p.ReviewRequests = append(p.ReviewRequests, r.User.actor())
		}
	}

	discussions, err := list[struct {
		IndividualNote bool     `json:"individual_note"`
		Notes          []glNote `json:"notes"`
	}](ctx, g, base+"/discussions?per_page=100")
	if err != nil {
		return pr{}, err
	}
	for _, d := range discussions {
		var notes []glNote
		open := false
		for _, n := range d.Notes {
			if !n.System {
				notes = append(notes, n)
			} else if m := glReviewNote.FindStringSubmatch(n.Body); m != nil {
				p.Reviews = append(p.Reviews, review{Author: n.Author.actor(), State: glReviewStates[strings.ToLower(m[1])], Submitted: n.CreatedAt})
			}
			open = open || (n.Resolvable && !n.Resolved)
		}
		if len(notes) == 0 {
			continue
		}
		toComment := func(n glNote) comment {
			return comment{Author: n.Author.actor(), Created: n.CreatedAt, URL: fmt.Sprintf("%s#note_%d", mr.WebURL, n.ID), Body: n.Body}
		}
		if d.IndividualNote {
			for _, n := range notes {
				p.Comments = append(p.Comments, toComment(n))
			}
			continue
		}
		for _, n := range notes {
			p.ThreadComments = append(p.ThreadComments, toComment(n))
		}
		if !open {
			continue
		}
		t := thread{Last: toComment(notes[len(notes)-1])}
		if pos := notes[0].Position; pos != nil {
			t.Path, t.Line = pos.NewPath, pos.NewLine
			if t.Path == "" {
				t.Path, t.Line = pos.OldPath, pos.OldLine
			}
		}
		p.Threads = append(p.Threads, t)
	}
	return p, nil
}

func (g *glabForge) BotEyes(ctx context.Context, repo string, number int) ([]string, error) {
	awards, err := list[struct {
		Name string `json:"name"`
		User glUser `json:"user"`
	}](ctx, g, fmt.Sprintf("%s/merge_requests/%d/award_emoji?per_page=100", project(repo), number))
	if err != nil {
		return nil, err
	}
	var bots []string
	for _, a := range awards {
		if a.Name == "eyes" && a.User.actor().Bot {
			bots = append(bots, a.User.Username)
		}
	}
	return bots, nil
}

func (g *glabForge) Runs(ctx context.Context, repo, sha, branch string) ([]workflowRun, error) {
	type pipeline struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
		Source string `json:"source"`
		Ref    string `json:"ref"`
		WebURL string `json:"web_url"`
	}
	query := fmt.Sprintf("%s/pipelines?sha=%s&ref=%s&per_page=100", project(repo), sha, url.QueryEscape(branch))
	pipelines, err := list[pipeline](ctx, g, query)
	if err != nil {
		return nil, err
	}
	// The list omits child pipelines unless asked for them by source.
	children, err := list[pipeline](ctx, g, query+"&source=parent_pipeline")
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, children...)
	var runs []workflowRun
	for _, pl := range pipelines {
		event := pl.Source
		if event == "web" {
			event = "workflow_dispatch"
		}
		r := workflowRun{ID: pl.ID, Name: fmt.Sprintf("pipeline %d", pl.ID), Event: event, URL: pl.WebURL, Status: "in_progress"}
		if state, conclusion := glStatus(pl.Status, false); state == "COMPLETED" {
			r.Status, r.Conclusion = "completed", strings.ToLower(conclusion)
		}
		runs = append(runs, r)
	}
	return runs, nil
}

func (g *glabForge) FailedJobs(ctx context.Context, repo string, runID int64) ([]job, error) {
	jobs, err := list[struct {
		Name         string `json:"name"`
		Status       string `json:"status"`
		AllowFailure bool   `json:"allow_failure"`
		WebURL       string `json:"web_url"`
	}](ctx, g, fmt.Sprintf("%s/pipelines/%d/jobs?scope[]=failed&per_page=100", project(repo), runID))
	if err != nil {
		return nil, err
	}
	var out []job
	for _, j := range jobs {
		if !j.AllowFailure {
			out = append(out, job{Name: j.Name, Conclusion: j.Status, URL: j.WebURL})
		}
	}
	return out, nil
}
