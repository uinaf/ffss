package forge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
)

func glabRunner(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "glab", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, &Error{Kind: ErrorTransient, Err: fmt.Errorf("run glab: %w", err)}
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("glab %s: %s", strings.Join(args[:min(2, len(args))], " "), detail)
	}
	return stdout.Bytes(), nil
}

// GitLab observes merge requests through the glab CLI. The host comes from
// the merge-request URL, so the same adapter supports GitLab.com and
// authenticated self-hosted GitLab instances.
type GitLab struct {
	run Runner
}

// NewGitLab returns the GitLab adapter; a nil runner uses the installed glab.
func NewGitLab(run Runner) *GitLab {
	if run == nil {
		run = glabRunner
	}
	return &GitLab{run: run}
}

func (g *GitLab) Kind() Kind { return KindGitLab }

func (g *GitLab) ParseChangeRequestURL(raw string) (ChangeRequestRef, error) {
	parsed, err := url.Parse(strings.TrimSuffix(raw, "/"))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ChangeRequestRef{}, &Error{Kind: ErrorNotFound, Err: fmt.Errorf("not a GitLab merge request URL: %q", raw)}
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 5 || segments[len(segments)-3] != "-" || segments[len(segments)-2] != "merge_requests" {
		return ChangeRequestRef{}, &Error{Kind: ErrorNotFound, Err: fmt.Errorf("not a GitLab merge request URL: %q", raw)}
	}
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return ChangeRequestRef{}, &Error{Kind: ErrorNotFound, Err: fmt.Errorf("invalid GitLab project path in %q", raw)}
		}
	}
	number, err := strconv.Atoi(segments[len(segments)-1])
	if err != nil || number < 1 {
		return ChangeRequestRef{}, &Error{Kind: ErrorNotFound, Err: fmt.Errorf("invalid merge request number in %q", raw)}
	}
	project := segments[:len(segments)-3]
	return ChangeRequestRef{
		Host:   parsed.Host,
		Owner:  strings.Join(project[:len(project)-1], "/"),
		Repo:   project[len(project)-1],
		Number: number,
	}, nil
}

type gitLabMergeRequest struct {
	SHA                 string `json:"sha"`
	State               string `json:"state"`
	MergeStatus         string `json:"merge_status"`
	DetailedMergeStatus string `json:"detailed_merge_status"`
	HeadPipeline        *struct {
		Status string `json:"status"`
	} `json:"head_pipeline"`
}

func (g *GitLab) Observe(ctx context.Context, ref ChangeRequestRef) (Observation, error) {
	mr, err := g.mergeRequest(ctx, ref)
	if err != nil {
		return Observation{}, err
	}
	threads, unresolved, digest, err := g.reviewThreads(ctx, ref)
	if err != nil {
		return Observation{}, err
	}
	checks := ChecksNone
	if mr.HeadPipeline != nil {
		checks = gitLabPipelineChecks(mr.HeadPipeline.Status)
	}
	return Observation{
		Ref:               ref,
		HeadSHA:           mr.SHA,
		Checks:            checks,
		Mergeability:      gitLabMergeability(mr.State, mr.DetailedMergeStatus, mr.MergeStatus),
		UnresolvedThreads: unresolved,
		Threads:           threads,
		ThreadsDigest:     digest,
	}, nil
}

func (g *GitLab) Head(ctx context.Context, ref ChangeRequestRef) (HeadState, error) {
	mr, err := g.mergeRequest(ctx, ref)
	if err != nil {
		return HeadState{}, err
	}
	return HeadState{SHA: mr.SHA, Merged: mr.State == "merged", Closed: mr.State == "closed"}, nil
}

func (g *GitLab) Reviews(ctx context.Context, ref ChangeRequestRef) ([]Review, error) {
	approved, err := g.approvals(ctx, ref)
	if err != nil {
		return nil, err
	}
	notes, err := g.reviewNotes(ctx, ref)
	if err != nil {
		return nil, err
	}
	return append(approved, notes...), nil
}

func (g *GitLab) mergeRequest(ctx context.Context, ref ChangeRequestRef) (gitLabMergeRequest, error) {
	raw, err := g.api(ctx, ref, "")
	if err != nil {
		return gitLabMergeRequest{}, err
	}
	var mr gitLabMergeRequest
	if err := json.Unmarshal(raw, &mr); err != nil {
		return gitLabMergeRequest{}, &Error{Kind: ErrorTransient, Err: fmt.Errorf("decode merge request for %s: %w", ref, err)}
	}
	return mr, nil
}

func (g *GitLab) approvals(ctx context.Context, ref ChangeRequestRef) ([]Review, error) {
	raw, err := g.api(ctx, ref, "/approvals")
	if err != nil {
		return nil, err
	}
	var response struct {
		ApprovedBy []struct {
			User struct {
				Username string `json:"username"`
			} `json:"user"`
		} `json:"approved_by"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, &Error{Kind: ErrorTransient, Err: fmt.Errorf("decode approvals for %s: %w", ref, err)}
	}
	reviews := make([]Review, 0, len(response.ApprovedBy))
	for _, approval := range response.ApprovedBy {
		if approval.User.Username != "" {
			reviews = append(reviews, Review{Author: approval.User.Username, State: "APPROVED"})
		}
	}
	return reviews, nil
}

type gitLabNote struct {
	ID         int64  `json:"id"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	System     bool   `json:"system"`
	Resolvable bool   `json:"resolvable"`
	Resolved   bool   `json:"resolved"`
	Author     struct {
		Username string `json:"username"`
	} `json:"author"`
	Position *struct {
		NewPath string `json:"new_path"`
		OldPath string `json:"old_path"`
		NewLine int    `json:"new_line"`
		OldLine int    `json:"old_line"`
	} `json:"position"`
}

type gitLabDiscussion struct {
	ID    string       `json:"id"`
	Notes []gitLabNote `json:"notes"`
}

const gitLabPageSize = 100

func (g *GitLab) reviewThreads(ctx context.Context, ref ChangeRequestRef) ([]ReviewThread, int, string, error) {
	var sample []ReviewThread
	unresolved := 0
	digest := sha256.New()
	for page := 1; page <= maxThreadPages; page++ {
		raw, err := g.api(ctx, ref, fmt.Sprintf("/discussions?per_page=%d&page=%d", gitLabPageSize, page))
		if err != nil {
			return nil, 0, "", err
		}
		var discussions []gitLabDiscussion
		if err := json.Unmarshal(raw, &discussions); err != nil {
			return nil, 0, "", &Error{Kind: ErrorTransient, Err: fmt.Errorf("decode discussions for %s: %w", ref, err)}
		}
		for _, discussion := range discussions {
			thread, open := gitLabThread(discussion)
			if !open {
				continue
			}
			unresolved++
			fmt.Fprintf(digest, "|%s@%s@%s", thread.ID, thread.LastCommentID, thread.LastCommentEdited)
			if len(sample) < maxThreadSample {
				sample = append(sample, thread)
			}
		}
		if len(discussions) < gitLabPageSize {
			return sample, unresolved, fmt.Sprintf("%x", digest.Sum(nil)), nil
		}
	}
	return nil, 0, "", &Error{Kind: ErrorTransient, Err: fmt.Errorf("%s has more than %d pages of review discussions; observation incomplete", ref, maxThreadPages)}
}

func gitLabThread(discussion gitLabDiscussion) (ReviewThread, bool) {
	open := false
	for _, note := range discussion.Notes {
		if note.Resolvable && !note.Resolved {
			open = true
			break
		}
	}
	if !open || len(discussion.Notes) == 0 {
		return ReviewThread{}, false
	}
	last := discussion.Notes[len(discussion.Notes)-1]
	thread := ReviewThread{
		ID:                discussion.ID,
		LastCommentID:     strconv.FormatInt(last.ID, 10),
		LastCommentEdited: last.UpdatedAt,
		Author:            last.Author.Username,
		Snippet:           snippet(last.Body),
	}
	for i := len(discussion.Notes) - 1; i >= 0; i-- {
		position := discussion.Notes[i].Position
		if position == nil {
			continue
		}
		thread.Path, thread.Line = position.NewPath, position.NewLine
		if thread.Path == "" {
			thread.Path, thread.Line = position.OldPath, position.OldLine
		}
		if thread.Path != "" {
			break
		}
	}
	return thread, true
}

func (g *GitLab) reviewNotes(ctx context.Context, ref ChangeRequestRef) ([]Review, error) {
	var reviews []Review
	for page := 1; page <= maxThreadPages; page++ {
		raw, err := g.api(ctx, ref, fmt.Sprintf("/notes?sort=asc&order_by=created_at&per_page=%d&page=%d", gitLabPageSize, page))
		if err != nil {
			return nil, err
		}
		var notes []gitLabNote
		if err := json.Unmarshal(raw, &notes); err != nil {
			return nil, &Error{Kind: ErrorTransient, Err: fmt.Errorf("decode notes for %s: %w", ref, err)}
		}
		for _, note := range notes {
			if !note.System && note.Author.Username != "" {
				reviews = append(reviews, Review{Author: note.Author.Username, State: "COMMENTED", SubmittedAt: note.CreatedAt})
			}
		}
		if len(notes) < gitLabPageSize {
			return reviews, nil
		}
	}
	return nil, &Error{Kind: ErrorTransient, Err: fmt.Errorf("%s has more than %d pages of merge request notes; review observation incomplete", ref, maxThreadPages)}
}

func (g *GitLab) api(ctx context.Context, ref ChangeRequestRef, suffix string) ([]byte, error) {
	host := ref.Host
	if host == "" {
		host = "gitlab.com"
	}
	project := url.PathEscape(ref.Owner + "/" + ref.Repo)
	endpoint := fmt.Sprintf("projects/%s/merge_requests/%d%s", project, ref.Number, suffix)
	raw, err := g.run(ctx, "api", endpoint, "--hostname", host)
	if err != nil {
		return nil, classify(err)
	}
	return raw, nil
}

func gitLabPipelineChecks(status string) ChecksState {
	switch strings.ToLower(status) {
	case "success", "skipped":
		return ChecksPassing
	case "failed", "canceled":
		return ChecksFailing
	case "":
		return ChecksNone
	default:
		return ChecksPending
	}
}

func gitLabMergeability(state, detailed, fallback string) Mergeability {
	switch strings.ToLower(state) {
	case "merged":
		return MergeableMerged
	case "closed":
		return MergeableClosed
	}
	switch strings.ToLower(detailed) {
	case "mergeable":
		return MergeableClean
	case "conflict":
		return MergeableConflicting
	}
	switch strings.ToLower(fallback) {
	case "can_be_merged":
		return MergeableClean
	case "cannot_be_merged":
		return MergeableConflicting
	default:
		return MergeableUnknown
	}
}
