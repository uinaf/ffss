package forge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Runner executes one forge CLI invocation and returns its stdout. The
// GitHub adapter shells out to the installed, authenticated `gh` so the
// binary never holds forge credentials itself.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

func ghRunner(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args[:min(2, len(args))], " "), detail)
	}
	return stdout.Bytes(), nil
}

// GitHub observes pull requests through the gh CLI.
type GitHub struct {
	run Runner
}

// NewGitHub returns the GitHub adapter; a nil runner uses the installed gh.
func NewGitHub(run Runner) *GitHub {
	if run == nil {
		run = ghRunner
	}
	return &GitHub{run: run}
}

func (g *GitHub) Kind() Kind { return KindGitHub }

var pullURLPattern = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9][A-Za-z0-9-]*)/([A-Za-z0-9._-]+)/pull/([0-9]+)$`)

func (g *GitHub) ParseChangeRequestURL(url string) (ChangeRequestRef, error) {
	match := pullURLPattern.FindStringSubmatch(strings.TrimSuffix(url, "/"))
	if match == nil {
		return ChangeRequestRef{}, &Error{Kind: ErrorNotFound, Err: fmt.Errorf("not a GitHub pull request URL: %q", url)}
	}
	number, err := strconv.Atoi(match[3])
	if err != nil || number < 1 {
		return ChangeRequestRef{}, &Error{Kind: ErrorNotFound, Err: fmt.Errorf("invalid pull request number in %q", url)}
	}
	return ChangeRequestRef{Owner: match[1], Repo: match[2], Number: number}, nil
}

func (g *GitHub) Observe(ctx context.Context, ref ChangeRequestRef) (Observation, error) {
	view, err := g.run(ctx, "pr", "view", strconv.Itoa(ref.Number),
		"--repo", ref.Owner+"/"+ref.Repo,
		"--json", "headRefOid,state,mergeable,statusCheckRollup")
	if err != nil {
		return Observation{}, classify(err)
	}
	var pr struct {
		HeadRefOid        string `json:"headRefOid"`
		State             string `json:"state"`
		Mergeable         string `json:"mergeable"`
		StatusCheckRollup []struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			State      string `json:"state"` // classic status contexts
		} `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal(view, &pr); err != nil {
		return Observation{}, &Error{Kind: ErrorTransient, Err: fmt.Errorf("decode pr view for %s: %w", ref, err)}
	}

	observation := Observation{
		Ref:          ref,
		HeadSHA:      pr.HeadRefOid,
		Checks:       ChecksNone,
		Mergeability: mergeability(pr.State, pr.Mergeable),
	}
	observation.Checks = rollupChecks(len(pr.StatusCheckRollup), func(i int) (string, string) {
		entry := pr.StatusCheckRollup[i]
		conclusion := entry.Conclusion
		if conclusion == "" {
			conclusion = entry.State
		}
		return entry.Status, conclusion
	})

	threads, unresolved, digest, err := g.reviewThreads(ctx, ref)
	if err != nil {
		return Observation{}, err
	}
	observation.Threads = threads
	observation.UnresolvedThreads = unresolved
	observation.ThreadsDigest = digest
	return observation, nil
}

func (g *GitHub) Head(ctx context.Context, ref ChangeRequestRef) (HeadState, error) {
	raw, err := g.run(ctx, "pr", "view", strconv.Itoa(ref.Number),
		"--repo", ref.Owner+"/"+ref.Repo,
		"--json", "headRefOid,state")
	if err != nil {
		return HeadState{}, classify(err)
	}
	var pr struct {
		HeadRefOid string `json:"headRefOid"`
		State      string `json:"state"`
	}
	if err := json.Unmarshal(raw, &pr); err != nil {
		return HeadState{}, &Error{Kind: ErrorTransient, Err: fmt.Errorf("decode pr head for %s: %w", ref, err)}
	}
	return HeadState{
		SHA:    pr.HeadRefOid,
		Merged: pr.State == "MERGED",
		Closed: pr.State == "CLOSED",
	}, nil
}

func (g *GitHub) Reviews(ctx context.Context, ref ChangeRequestRef) ([]Review, error) {
	raw, err := g.run(ctx, "pr", "view", strconv.Itoa(ref.Number),
		"--repo", ref.Owner+"/"+ref.Repo,
		"--json", "reviews")
	if err != nil {
		return nil, classify(err)
	}
	var pr struct {
		Reviews []struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			State       string `json:"state"`
			SubmittedAt string `json:"submittedAt"`
		} `json:"reviews"`
	}
	if err := json.Unmarshal(raw, &pr); err != nil {
		return nil, &Error{Kind: ErrorTransient, Err: fmt.Errorf("decode pr reviews for %s: %w", ref, err)}
	}
	reviews := make([]Review, 0, len(pr.Reviews))
	for _, review := range pr.Reviews {
		reviews = append(reviews, Review{Author: review.Author.Login, State: review.State, SubmittedAt: review.SubmittedAt})
	}
	return reviews, nil
}

const (
	maxThreadSample  = 10
	maxThreadSnippet = 200
	maxThreadPages   = 10
)

func (g *GitHub) reviewThreads(ctx context.Context, ref ChangeRequestRef) ([]ReviewThread, int, string, error) {
	const query = `query($owner:String!,$repo:String!,$number:Int!,$cursor:String){
  repository(owner:$owner,name:$repo){
    pullRequest(number:$number){
      reviewThreads(first:100, after:$cursor){
        pageInfo{hasNextPage endCursor}
        nodes{
          id isResolved path line
          comments(last:1){nodes{id updatedAt author{login} body}}
        }
      }
    }
  }
}`
	var sample []ReviewThread
	unresolved := 0
	digest := sha256.New()
	cursor := ""
	// Bounded pagination keeps the count honest on huge threads; beyond the
	// bound the observation fails closed instead of undercounting.
	for page := 0; page < maxThreadPages; page++ {
		args := []string{"api", "graphql",
			"-f", "query=" + query,
			"-F", "owner=" + ref.Owner,
			"-F", "repo=" + ref.Repo,
			"-F", "number=" + strconv.Itoa(ref.Number)}
		if cursor != "" {
			args = append(args, "-F", "cursor="+cursor)
		}
		raw, err := g.run(ctx, args...)
		if err != nil {
			return nil, 0, "", classify(err)
		}
		nodes, pageInfo, err := decodeThreadsPage(raw, ref)
		if err != nil {
			return nil, 0, "", err
		}
		for _, thread := range nodes {
			if thread.Resolved {
				continue
			}
			unresolved++
			// Every unresolved thread joins the digest, sampled or not, so a
			// new or edited comment beyond the sample still changes the
			// observation.
			fmt.Fprintf(digest, "|%s@%s@%s", thread.ID, thread.LastCommentID, thread.LastCommentEdited)
			if len(sample) < maxThreadSample {
				sample = append(sample, thread)
			}
		}
		if !pageInfo.HasNextPage {
			return sample, unresolved, fmt.Sprintf("%x", digest.Sum(nil)), nil
		}
		cursor = pageInfo.EndCursor
	}
	return nil, 0, "", &Error{Kind: ErrorTransient, Err: fmt.Errorf("%s has more than %d pages of review threads; observation incomplete", ref, maxThreadPages)}
}

type threadPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

func decodeThreadsPage(raw []byte, ref ChangeRequestRef) ([]ReviewThread, threadPageInfo, error) {
	var response struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ReviewThreads struct {
						PageInfo threadPageInfo `json:"pageInfo"`
						Nodes    []struct {
							ID         string `json:"id"`
							IsResolved bool   `json:"isResolved"`
							Path       string `json:"path"`
							Line       int    `json:"line"`
							Comments   struct {
								Nodes []struct {
									ID        string `json:"id"`
									UpdatedAt string `json:"updatedAt"`
									Author    struct {
										Login string `json:"login"`
									} `json:"author"`
									Body string `json:"body"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, threadPageInfo{}, &Error{Kind: ErrorTransient, Err: fmt.Errorf("decode review threads for %s: %w", ref, err)}
	}
	threads := response.Data.Repository.PullRequest.ReviewThreads
	out := make([]ReviewThread, 0, len(threads.Nodes))
	for _, node := range threads.Nodes {
		thread := ReviewThread{ID: node.ID, Path: node.Path, Line: node.Line, Resolved: node.IsResolved}
		if len(node.Comments.Nodes) > 0 {
			last := node.Comments.Nodes[len(node.Comments.Nodes)-1]
			thread.Author = last.Author.Login
			thread.Snippet = snippet(last.Body)
			thread.LastCommentID = last.ID
			thread.LastCommentEdited = last.UpdatedAt
		}
		out = append(out, thread)
	}
	return out, threads.PageInfo, nil
}

func snippet(body string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(body), "\n")
	if len(line) > maxThreadSnippet {
		return line[:maxThreadSnippet]
	}
	return line
}

func mergeability(state, mergeable string) Mergeability {
	switch state {
	case "MERGED":
		return MergeableMerged
	case "CLOSED":
		return MergeableClosed
	}
	switch mergeable {
	case "MERGEABLE":
		return MergeableClean
	case "CONFLICTING":
		return MergeableConflicting
	default:
		return MergeableUnknown
	}
}

func rollupChecks(count int, at func(int) (status, conclusion string)) ChecksState {
	if count == 0 {
		return ChecksNone
	}
	pending := false
	for i := 0; i < count; i++ {
		status, conclusion := at(i)
		switch strings.ToUpper(conclusion) {
		case "FAILURE", "ERROR", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED", "STARTUP_FAILURE", "STALE":
			return ChecksFailing
		case "SUCCESS", "NEUTRAL", "SKIPPED":
			continue
		}
		switch strings.ToUpper(status) {
		case "COMPLETED":
			// Completed with an unrecognized conclusion: treat as pending
			// rather than inventing a pass.
			pending = true
		default:
			pending = true
		}
	}
	if pending {
		return ChecksPending
	}
	return ChecksPassing
}

// classify maps gh failures onto the stable observation taxonomy. Message
// sniffing is confined to this boundary; callers branch on Error.Kind only.
func classify(err error) error {
	message := strings.ToLower(err.Error())
	kind := ErrorTransient
	switch {
	case strings.Contains(message, "rate limit"), strings.Contains(message, "http 429"),
		strings.Contains(message, "too many requests"):
		kind = ErrorRateLimit
	case strings.Contains(message, "http 401"), strings.Contains(message, "http 403"),
		strings.Contains(message, "authentication"), strings.Contains(message, "auth login"),
		strings.Contains(message, "bad credentials"):
		kind = ErrorAuth
	// Transport failures stay transient even when their wording overlaps
	// GitHub's object-resolution messages.
	case strings.Contains(message, "could not resolve host"), strings.Contains(message, "no such host"),
		strings.Contains(message, "dial tcp"), strings.Contains(message, "timeout"):
		kind = ErrorTransient
	case strings.Contains(message, "http 404"), strings.Contains(message, "not found"),
		strings.Contains(message, "could not resolve to"):
		kind = ErrorNotFound
	}
	return &Error{Kind: kind, Err: err}
}
