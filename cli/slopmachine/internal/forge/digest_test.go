package forge

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestThreadsDigestCoversUnsampledThreads pins that the observation digest
// fingerprints every unresolved thread, not just the bounded sample: a new
// comment on the 11th thread must change the digest.
func TestThreadsDigestCoversUnsampledThreads(t *testing.T) {
	view := `{"headRefOid":"aaa1111","state":"OPEN","mergeable":"MERGEABLE","statusCheckRollup":[]}`
	buildThreads := func(lastCommentID string) string {
		nodes := make([]string, 0, 11)
		for i := 0; i < 11; i++ {
			comment := fmt.Sprintf("c%d", i)
			if i == 10 {
				comment = lastCommentID
			}
			nodes = append(nodes, fmt.Sprintf(
				`{"id":"t%d","isResolved":false,"path":"f%d.go","line":%d,"comments":{"nodes":[{"id":%q,"author":{"login":"rev"},"body":"x"}]}}`,
				i, i, i+1, comment))
		}
		return `{"data":{"repository":{"pullRequest":{"reviewThreads":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[` +
			strings.Join(nodes, ",") + `]}}}}}`
	}

	observe := func(threads string) Observation {
		t.Helper()
		g := NewGitHub(fakeRunner(view, threads, nil))
		obs, err := g.Observe(context.Background(), ChangeRequestRef{Owner: "o", Repo: "r", Number: 1})
		if err != nil {
			t.Fatal(err)
		}
		return obs
	}
	before := observe(buildThreads("c10"))
	if before.UnresolvedThreads != 11 || len(before.Threads) != 10 {
		t.Fatalf("sample must stay bounded while the count covers all: %d/%d", before.UnresolvedThreads, len(before.Threads))
	}
	if before.ThreadsDigest == "" {
		t.Fatal("observation must carry a digest over every unresolved thread")
	}
	after := observe(buildThreads("c10-new-reply"))
	if after.ThreadsDigest == before.ThreadsDigest {
		t.Fatal("a new comment beyond the sample bound must change the digest")
	}
	same := observe(buildThreads("c10"))
	if same.ThreadsDigest != before.ThreadsDigest {
		t.Fatal("unchanged threads must keep an identical digest")
	}
}
