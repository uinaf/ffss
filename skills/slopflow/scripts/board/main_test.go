package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderRejectsBoardsOutsideTheContract(t *testing.T) {
	epic := `"epics":[{"id":"e","title":"Search"}]`
	issue := func(id, extra string) string {
		return `{"id":"` + id + `","epic":"e","kind":"issue","state":"running","title":"Search","ref":"` + id + `","url":"https://example.com/` + id + `"` + extra + `}`
	}
	pr := func(id string) string {
		return `{"id":"` + id + `","epic":"e","kind":"pr","state":"review","title":"Search","ref":"` + id + `","url":"https://example.com/` + id + `"}`
	}
	board := func(nodes, edges string) string {
		return `{"title":"Lanes",` + epic + `,"nodes":[` + nodes + `],"edges":[` + edges + `]}`
	}
	cases := map[string]struct{ input, want string }{
		"not JSON":          {`{"title":`, "unexpected EOF"},
		"unknown field":     {`{"title":"Lanes",` + epic + `,"lanes":[]}`, `unknown field "lanes"`},
		"no title":          {`{` + epic + `}`, "title is required"},
		"no epics":          {`{"title":"Lanes"}`, "at least one epic"},
		"node in no epic":   {board(strings.Replace(issue("#1", ""), `"epic":"e"`, `"epic":"x"`, 1), ""), `epic "x" is not in epics`},
		"unknown kind":      {board(strings.Replace(issue("#1", ""), `"issue"`, `"task"`, 1), ""), `kind "task" is not one of issue, pr`},
		"unknown state":     {board(strings.Replace(issue("#1", ""), `"running"`, `"waiting"`, 1), ""), `state "waiting" is not one of`},
		"relative url":      {board(strings.Replace(issue("#1", ""), "https://example.com/", "/", 1), ""), "url must be an absolute http(s) URL"},
		"script url":        {board(strings.Replace(issue("#1", ""), "https://example.com/", "javascript:alert(1)//", 1), ""), "url must be an absolute http(s) URL"},
		"duplicate node":    {board(issue("#1", "")+","+issue("#1", ""), ""), "duplicate id"},
		"merged on a pr":    {board(strings.Replace(pr("#2"), `}`, `,"merged":["#3"]}`, 1), ""), "merged is for issues"},
		"pr for a missing":  {board(strings.Replace(pr("#2"), `}`, `,"issue":"#9"}`, 1), ""), `issue "#9" must be an issue node`},
		"edge to nowhere":   {board(issue("#1", ""), `{"kind":"blocked-by","from":"#1","to":"#9"}`), "from and to must be node ids"},
		"unknown edge":      {board(issue("#1", "")+","+issue("#2", ""), `{"kind":"parent","from":"#1","to":"#2"}`), `kind "parent" is not one of blocked-by, stacked`},
		"stacked issues":    {board(issue("#1", "")+","+issue("#2", ""), `{"kind":"stacked","from":"#1","to":"#2"}`), "stacked edges join two pr nodes"},
		"self edge":         {board(issue("#1", ""), `{"kind":"blocked-by","from":"#1","to":"#1"}`), "cannot point at itself"},
		"cycle":             {board(issue("#1", "")+","+issue("#2", ""), `{"kind":"blocked-by","from":"#1","to":"#2"},{"kind":"blocked-by","from":"#2","to":"#1"}`), "edges form a cycle: #1 → #2 → #1"},
		"trailing data":     {board(issue("#1", ""), "") + `{}`, "trailing data"},
		"question not text": {board(issue("#1", `,"question":7`), ""), "cannot unmarshal number"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "board.html")
			err := run([]string{"render", "-", out}, strings.NewReader(c.input), &bytes.Buffer{})
			if err == nil || !strings.HasPrefix(err.Error(), "board JSON: ") || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want a board JSON error containing %q, got %v", c.want, err)
			}
			if _, statErr := os.Stat(out); statErr == nil {
				t.Fatal("wrote a board from input outside the contract")
			}
		})
	}
}

func TestRenderDrawsEveryNodeAsALink(t *testing.T) {
	data, err := os.ReadFile("testdata/board.json")
	if err != nil {
		t.Fatal(err)
	}
	var board Board
	if err := json.Unmarshal(data, &board); err != nil {
		t.Fatal(err)
	}
	board.Nodes[0].Title = `<script>alert(1)</script>`
	data, err = json.Marshal(board)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "board.html")
	var stdout bytes.Buffer
	if err := run([]string{"render", "-", out}, bytes.NewReader(data), &stdout); err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	page := string(html)
	if strings.Contains(page, "<script") {
		t.Fatal("a node title reached the page unescaped")
	}
	if got := strings.Count(page, `<a class="card `); got != len(board.Nodes) {
		t.Fatalf("want %d cards, got %d", len(board.Nodes), got)
	}
	for _, n := range board.Nodes {
		if !strings.Contains(page, `href="`+n.URL+`"`) {
			t.Fatalf("%s does not link to %s", n.ID, n.URL)
		}
	}
	if !strings.Contains(page, "Index attachments on device or on the server?") {
		t.Fatal("the needs-you strip lost its question")
	}
	if !strings.HasPrefix(stdout.String(), "needs-you") {
		t.Fatalf("needs-you nodes should list first, got %q", stdout.String())
	}
}

func TestGitHubBoardKeepsNestedIssuesReviewsAndCrossEpicStacks(t *testing.T) {
	issue := func(id string, number, children int, labels, prs string) string {
		return fmt.Sprintf(`{"id":%q,"number":%d,"title":"Issue %d","url":"https://github.com/acme/app/issues/%d","state":"OPEN",
			"repository":{"nameWithOwner":"acme/app"},"labels":{"nodes":[%s]},"subIssues":{"totalCount":%d},
			"blockedBy":{"totalCount":0,"nodes":[]},"closedByPullRequestsReferences":{"totalCount":%d,"nodes":[%s]},"comments":{"nodes":[]}}`,
			id, number, number, number, labels, children, strings.Count(prs, `"number"`), prs)
	}
	pr := func(number int, base, head, reviewer string) string {
		return fmt.Sprintf(`{"number":%d,"title":"PR %d","url":"https://github.com/acme/app/pull/%d","state":"OPEN","isDraft":false,
			"baseRefName":%q,"headRefName":%q,"repository":{"nameWithOwner":"acme/app"},
			"reviewRequests":{"nodes":[{"requestedReviewer":{"login":%q}}]}}`, number, number, number, base, head, reviewer)
	}
	children := func(nodes ...string) string {
		return `{"node":{"subIssues":{"pageInfo":{"hasNextPage":false},"nodes":[` + strings.Join(nodes, ",") + `]}}}`
	}
	responses := map[string]string{
		"root": `{"viewer":{"login":"me"},"repository":{"issue":{"title":"Lanes","url":"https://github.com/acme/app/issues/1",
			"subIssues":{"pageInfo":{"hasNextPage":false},"nodes":[` +
			issue("E", 10, 1, "", "") + "," + issue("L", 2, 0, "", pr(32, "api", "ui", "someone")) + `]}}}}`,
		"E": children(issue("T", 11, 1, "", pr(30, "main", "api", "me"))),
		"T": children(issue("S", 12, 0, `{"name":"needs-user"}`, "")),
	}
	graphql = func(_ string, vars map[string]string, into any) error {
		key := vars["id"]
		if key == "" {
			key = "root"
		}
		return json.Unmarshal([]byte(responses[key]), into)
	}
	t.Cleanup(func() { graphql = ghGraphQL })

	out := filepath.Join(t.TempDir(), "board.html")
	if err := run([]string{"github", "acme/app", "1", out}, nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	page := string(html)
	for _, want := range []string{
		`href="https://github.com/acme/app/issues/12"`,
		`<span class="q">review: PR 30</span>`,
		`<span class="q">Issue 12</span>`,
		`app #32 · on app #30`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("board lacks %s", want)
		}
	}
	if strings.Contains(page, "review: PR 32") {
		t.Error("a review requested from someone else reached the needs-you strip")
	}
}
