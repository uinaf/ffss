package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
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
		"not JSON":            {`{"title":`, "unexpected EOF"},
		"unknown field":       {`{"title":"Lanes",` + epic + `,"lanes":[]}`, `unknown field "lanes"`},
		"no title":            {`{` + epic + `}`, "title is required"},
		"no epics":            {`{"title":"Lanes"}`, "at least one epic"},
		"node in no epic":     {board(strings.Replace(issue("#1", ""), `"epic":"e"`, `"epic":"x"`, 1), ""), `epic "x" is not in epics`},
		"unknown kind":        {board(strings.Replace(issue("#1", ""), `"issue"`, `"task"`, 1), ""), `kind "task" is not one of issue, pr`},
		"unknown state":       {board(strings.Replace(issue("#1", ""), `"running"`, `"waiting"`, 1), ""), `state "waiting" is not one of`},
		"relative url":        {board(strings.Replace(issue("#1", ""), "https://example.com/", "/", 1), ""), "url must be an absolute http(s) URL"},
		"script url":          {board(strings.Replace(issue("#1", ""), "https://example.com/", "javascript:alert(1)//", 1), ""), "url must be an absolute http(s) URL"},
		"duplicate node":      {board(issue("#1", "")+","+issue("#1", ""), ""), "duplicate id"},
		"merged on a pr":      {board(strings.Replace(pr("#2"), `}`, `,"merged":["#3"]}`, 1), ""), "merged is for issues"},
		"pr for a missing":    {board(strings.Replace(pr("#2"), `}`, `,"issue":"#9"}`, 1), ""), `issue "#9" must be an issue node`},
		"edge to nowhere":     {board(issue("#1", ""), `{"kind":"blocked-by","from":"#1","to":"#9"}`), "from and to must be node ids"},
		"unknown edge":        {board(issue("#1", "")+","+issue("#2", ""), `{"kind":"parent","from":"#1","to":"#2"}`), `kind "parent" is not one of blocked-by, stacked`},
		"stacked issues":      {board(issue("#1", "")+","+issue("#2", ""), `{"kind":"stacked","from":"#1","to":"#2"}`), "stacked edges join two pr nodes"},
		"self edge":           {board(issue("#1", ""), `{"kind":"blocked-by","from":"#1","to":"#1"}`), "cannot point at itself"},
		"cycle":               {board(issue("#1", "")+","+issue("#2", ""), `{"kind":"blocked-by","from":"#1","to":"#2"},{"kind":"blocked-by","from":"#2","to":"#1"}`), "edges form a cycle: #1 → #2 → #1"},
		"trailing data":       {board(issue("#1", ""), "") + `{}`, "trailing data"},
		"needs-you no need":   {board(strings.Replace(issue("#1", ""), `"running"`, `"needs-you"`, 1), ""), "a needs-you node needs need"},
		"need when running":   {board(issue("#1", `,"need":{"kind":"decision","since":"2026-11-20T09:30:00Z"}`), ""), "need is for needs-you nodes"},
		"unknown need kind":   {board(strings.Replace(issue("#1", `,"need":{"kind":"later","since":"2026-11-20T09:30:00Z"}`), `"running"`, `"needs-you"`, 1), ""), `need.kind "later" is not one of blocked, approval, decision`},
		"need since no time":  {board(strings.Replace(issue("#1", `,"need":{"kind":"decision","since":"yesterday"}`), `"running"`, `"needs-you"`, 1), ""), `need.since "yesterday" is not an RFC 3339 time`},
		"options no question": {board(strings.Replace(issue("#1", `,"need":{"kind":"decision","since":"2026-11-20T09:30:00Z","options":["A","B"]}`), `"running"`, `"needs-you"`, 1), ""), "need.options answer need.question"},
		"recommend elsewhere": {board(strings.Replace(issue("#1", `,"need":{"kind":"decision","since":"2026-11-20T09:30:00Z","question":"A or B?","options":["A","B"],"recommend":"C"}`), `"running"`, `"needs-you"`, 1), ""), `need.recommend "C" is not one of need.options`},
		"negative cap":        {strings.Replace(board(issue("#1", ""), ""), `"title":"Lanes",`, `"title":"Lanes","cap":-1,`, 1), "cap must not be negative"},
		"stall no duration":   {strings.Replace(board(issue("#1", ""), ""), `"title":"Lanes",`, `"title":"Lanes","stall":"soon",`, 1), `stall "soon" is not a positive duration`},
		"active no time":      {strings.Replace(board(issue("#1", ""), ""), `"title":"Search"}`, `"title":"Search","active":"yesterday"}`, 1), `active "yesterday" is not an RFC 3339 time`},
		"delivery on a pr":    {board(strings.Replace(pr("#2"), `}`, `,"delivery":{"stage":"landed","since":"2026-11-20T09:30:00Z"}}`, 1), ""), "delivery is for issues"},
		"unknown stage":       {board(issue("#1", `,"delivery":{"stage":"merged","since":"2026-11-20T09:30:00Z"}`), ""), `delivery.stage "merged" is not one of landed, deployed, proven, waived`},
		"stage since no time": {board(issue("#1", `,"delivery":{"stage":"landed","since":"today"}`), ""), `delivery.since "today" is not an RFC 3339 time`},
		"done when landed":    {board(strings.Replace(issue("#1", `,"delivery":{"stage":"landed","since":"2026-11-20T09:30:00Z"}`), `"running"`, `"done"`, 1), ""), "done exactly when its delivery is proven or waived"},
		"running when proven": {board(issue("#1", `,"delivery":{"stage":"proven","since":"2026-11-20T09:30:00Z"}`), ""), "done exactly when its delivery is proven or waived"},
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

func stubNow(t *testing.T) {
	now = func() time.Time { return time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { now = time.Now })
}

func needsStrip(page string) string {
	start := strings.Index(page, `<div class="needs">`)
	end := strings.Index(page, `<div class="band">`)
	if start < 0 || end < start {
		return ""
	}
	return page[start:end]
}

func TestRenderDrawsEveryNodeAsALink(t *testing.T) {
	stubNow(t)
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
	rows := []string{
		`<span class="k">blocked</span><span class="q">Index attachments on device or on the server?</span><span class="r">26h · app #241</span>`,
		`<span class="k">approval</span><span class="q">feat(ui): member list</span><span class="r">45h · app #236</span>`,
		`<span class="k">approval</span><span class="q">feat(ui): member roles</span><span class="r">25h · app #237</span>`,
		`<span class="k">decision</span><span class="q">Prorate upgrades to the day, or bill the full month?</span><span class="r">4d · app #251</span>`,
	}
	strip, last := needsStrip(page), -1
	for _, row := range rows {
		at := strings.Index(strip, row)
		if at <= last {
			t.Fatalf("needs-you strip lacks %s after the rows ranked above it:\n%s", row, strip)
		}
		last = at
	}
	flow := "WIP 5/5 · landed, not live: app #261 40m · oldest in flight 40m · last proven 35m ago · stalled: Search 3h"
	if !strings.Contains(page, `<div class="flow">`+flow+`</div>`) {
		t.Errorf("the flow line is missing from the page header, want %q", flow)
	}
	if first, rest, _ := strings.Cut(stdout.String(), "\n"); first != flow || !strings.HasPrefix(rest, "needs-you") {
		t.Fatalf("stdout should open with the flow line, then needs-you nodes; got %q", stdout.String())
	}
	if !strings.Contains(page, `<span class="n">3</span> <span class="stall">stalled 3h</span>`) {
		t.Error("the silent search lane is not marked stalled")
	}
	if !strings.Contains(page, `<span class="u-dot u-dot--live"></span>landed</span><span class="t">Sharing guide</span><span class="m">app #261 · merged #262 · publish pending</span>`) {
		t.Error("a landed issue does not show its stage and evidence")
	}
}

func TestDecidePutsEveryQuestionOnOnePage(t *testing.T) {
	stubNow(t)
	out := filepath.Join(t.TempDir(), "decide.html")
	var stdout bytes.Buffer
	if err := run([]string{"decide", "testdata/board.json", out}, nil, &stdout); err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	page := string(html)
	if strings.Contains(page, "<script") {
		t.Fatal("the decision page carries a script")
	}
	if got := strings.Count(page, `<section class="card">`); got != 2 {
		t.Fatalf("want a card for each of the 2 questions, got %d", got)
	}
	blocked := strings.Index(page, "Index attachments on device or on the server?")
	decision := strings.Index(page, "Prorate upgrades to the day, or bill the full month?")
	if blocked < 0 || decision < blocked {
		t.Error("want the blocking question first, then the decision")
	}
	if !strings.Contains(page, `<li>Prorate to the day</li><li class="rec">Bill the full month <span class="tag">recommended</span></li>`) {
		t.Error("options or the recommendation are missing from the billing card")
	}
	if !strings.Contains(page, "<pre>app #241: \napp #251: Bill the full month\n</pre>") {
		t.Errorf("the answer block should list each question, prefilled with its recommendation:\n%s", page)
	}
}

func TestGitHubBoardKeepsNestedIssuesReviewsAndCrossEpicStacks(t *testing.T) {
	stubNow(t)
	issue := func(id string, number, children int, labels, blockers, prs string) string {
		return fmt.Sprintf(`{"id":%q,"number":%d,"title":"Issue %d","url":"https://github.com/acme/app/issues/%d","state":"OPEN",
			"updatedAt":"2026-11-21T11:00:00Z","repository":{"nameWithOwner":"acme/app"},"labels":{"nodes":[%s]},"subIssues":{"totalCount":%d},
			"blockedBy":{"totalCount":%d,"nodes":[%s]},"closedByPullRequestsReferences":{"totalCount":%d,"nodes":[%s]},"comments":{"nodes":[]},
			"timelineItems":{"nodes":[{"createdAt":"2026-11-20T12:00:00Z","label":{"name":"needs-user"}},{"createdAt":"2026-11-20T13:00:00Z","label":{"name":"bug"}}]}}`,
			id, number, number, number, labels, children, strings.Count(blockers, `"number"`), blockers, strings.Count(prs, `"number"`), prs)
	}
	pr := func(number int, base, headRepo, head, reviewer string) string {
		return fmt.Sprintf(`{"number":%d,"title":"PR %d","url":"https://github.com/acme/app/pull/%d","state":"OPEN","isDraft":false,
			"baseRefName":%q,"headRepository":{"nameWithOwner":%q},"headRefName":%q,"repository":{"nameWithOwner":"acme/app"},"updatedAt":"2026-11-21T11:30:00Z",
			"reviewRequests":{"nodes":[{"requestedReviewer":{"login":%q}}]},
			"timelineItems":{"nodes":[{"createdAt":"2026-11-21T10:00:00Z","requestedReviewer":{"login":%q}}]}}`,
			number, number, number, base, headRepo, head, reviewer, reviewer)
	}
	children := func(nodes ...string) string {
		return `{"node":{"subIssues":{"pageInfo":{"hasNextPage":false},"nodes":[` + strings.Join(nodes, ",") + `]}}}`
	}
	revision := func(at, line string) string {
		return fmt.Sprintf(`{"editedAt":%q,"diff":%q}`, at, "## Lane t\n"+line+"\n- [ ] #12 needs-user\n")
	}
	ledger := `{"node":{"body":"## Lane t\n- [ ] #13 landed: #33 on staging, promote pending\n- [ ] #12 needs-user\n","createdAt":"2026-11-21T09:00:00Z",
		"userContentEdits":{"nodes":[` + strings.Join([]string{
		revision("2026-11-21T11:40:00Z", "- [ ] #13 landed: #33 on staging, promote pending"),
		revision("2026-11-21T11:20:00Z", "- [ ] #13 landed: #33"),
		revision("2026-11-21T10:00:00Z", "- [ ] #13 PR #33"),
		revision("2026-11-21T09:00:00Z", "- [ ] #13"),
	}, ",") + `]},`
	closed := strings.Replace(issue("U", 13, 0, "", "", ""), `"state":"OPEN"`, `"state":"CLOSED"`, 1)
	asked := strings.Replace(issue("S", 12, 0, `{"name":"needs-user"}`, "", ""), `"comments":{"nodes":[]}`,
		`"comments":{"nodes":[{"body":"Which store holds drafts?\n\n- SQLite (recommended)\n- IndexedDB\n\nThanks"}]}`, 1)
	responses := map[string]string{
		"root": `{"viewer":{"login":"me"},"repository":{"issue":{"title":"Lanes","url":"https://github.com/acme/app/issues/1",
			"subIssues":{"pageInfo":{"hasNextPage":false},"nodes":[` +
			issue("E", 10, 1, `{"name":"needs-user"}`, "", "") + "," +
			issue("L", 2, 0, "", `{"number":12,"state":"OPEN","repository":{"nameWithOwner":"acme/app"}}`, pr(32, "api", "acme/app", "ui", "someone")) + `]}}}}`,
		"E": strings.Replace(children(issue("T", 11, 1, "", "", pr(30, "main", "acme/app", "api", "me")+","+
			pr(40, "main", "fork-a/app", "main", "someone")+","+pr(41, "main", "fork-b/app", "main", "someone")), closed), `{"node":{`, ledger, 1),
		"T": children(asked),
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
	var stdout bytes.Buffer
	if err := run([]string{"github", "-stall", "20m", "acme/app", "1", out}, nil, &stdout); err != nil {
		t.Fatal(err)
	}
	flow := "WIP 2 · landed, not live: app #13 40m · oldest in flight 40m · stalled: Lanes 30m, Issue 10 30m"
	if first, _, _ := strings.Cut(stdout.String(), "\n"); first != flow {
		t.Errorf("want flow line %q, got %q", flow, first)
	}
	data := filepath.Join(t.TempDir(), "board.json")
	if err := run([]string{"github", "acme/app", "1", data}, nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(data)
	if err != nil {
		t.Fatal(err)
	}
	var board Board
	if err := json.Unmarshal(raw, &board); err != nil {
		t.Fatal(err)
	}
	for _, n := range board.Nodes {
		switch n.ID {
		case "acme/app#13":
			if n.State != "running" || n.Delivery == nil || *n.Delivery != (Delivery{Stage: "landed", Since: "2026-11-21T11:20:00Z", Evidence: "#33 on staging, promote pending"}) {
				t.Errorf("a closed issue the ledger has at landed should stay in flight since it landed, got %s %+v", n.State, n.Delivery)
			}
		case "acme/app#12":
			if n.Need == nil || n.Need.Question != "Which store holds drafts?" || !slices.Equal(n.Need.Options, []string{"SQLite", "IndexedDB"}) || n.Need.Recommend != "SQLite" {
				t.Errorf("the question's options and recommendation were not read, got %+v", n.Need)
			}
		}
	}
	html, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	page := string(html)
	strip := needsStrip(page)
	blocked := strings.Index(strip, `<span class="k">blocked</span><span class="q">Which store holds drafts?</span><span class="r">24h · app #12</span>`)
	approval := strings.Index(strip, `<span class="k">approval</span><span class="q">PR 30</span><span class="r">2h · app #30</span>`)
	if blocked < 0 || approval < blocked {
		t.Errorf("want the nested blocking decision, then the review requested from the viewer:\n%s", strip)
	}
	if !strings.Contains(strip, `<span class="q">Issue 10</span>`) {
		t.Errorf("the epic's own needs-user ask is missing:\n%s", strip)
	}
	if strings.Contains(strip, "PR 32") {
		t.Error("a review requested from someone else reached the needs-you strip")
	}
	if !strings.Contains(page, `app #32 · on app #30`) {
		t.Error("the cross-epic stack left no trace on its card")
	}
}
