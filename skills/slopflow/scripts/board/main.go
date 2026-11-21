// Command board draws a slopflow board: one self-contained HTML page with the
// nodes that need the user on top and the issue graph below, grouped by epic.
//
//	go run <skill dir>/scripts/board/main.go github <owner/repo> <root issue> <out.html | out.json>
//	go run <skill dir>/scripts/board/main.go render <board.json | -> <out.html>
//
// github reads the root issue's sub-issues in any repository through `gh`. A
// sub-issue with sub-issues of its own is an epic; the other sub-issues group
// under the root. Each issue brings its "blocked by" dependencies, the
// needs-user label, the last question in its comments, and the pull requests
// linked to it. Write out.json to get the board in the render contract.
//
// render takes the same board from any tracker:
//
//	{
//	  "title": "Lanes",
//	  "epics": [{"id": "search", "title": "Search", "url": "https://…"}],
//	  "nodes": [{
//	    "id": "app#12", "epic": "search", "kind": "issue", "state": "needs-you",
//	    "title": "Index attachments", "ref": "#12", "repo": "acme/app",
//	    "url": "https://…", "question": "On device or on the server?",
//	    "merged": ["#20"]
//	  }, {
//	    "id": "app#30", "epic": "search", "kind": "pr", "state": "review",
//	    "title": "Index on device", "ref": "#30", "url": "https://…", "issue": "app#12"
//	  }],
//	  "edges": [{"kind": "blocked-by", "from": "app#11", "to": "app#12"}]
//	}
//
// kind is issue or pr. state is running, needs-you, blocked, review, or done.
// An epic's url, a node's repo and question, an issue's merged refs, and a
// pull request's issue are optional. A blocked-by edge points from the blocker
// to the node it blocks; a stacked edge from a base pull request to the one
// stacked on it. Edges must not form a cycle.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

type Board struct {
	Title string `json:"title"`
	Epics []Epic `json:"epics"`
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Epic struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
}

type Node struct {
	ID       string   `json:"id"`
	Epic     string   `json:"epic"`
	Kind     string   `json:"kind"`
	State    string   `json:"state"`
	Title    string   `json:"title"`
	Ref      string   `json:"ref"`
	Repo     string   `json:"repo,omitempty"`
	URL      string   `json:"url"`
	Question string   `json:"question,omitempty"`
	Merged   []string `json:"merged,omitempty"`
	Issue    string   `json:"issue,omitempty"`
}

type Edge struct {
	Kind string `json:"kind"`
	From string `json:"from"`
	To   string `json:"to"`
}

var (
	states = []string{"needs-you", "blocked", "running", "review", "done"}
	kinds  = []string{"issue", "pr"}
	links  = []string{"blocked-by", "stacked"}
)

const usage = `usage:
  board github <owner/repo> <root issue> <out.html | out.json>
  board render <board.json | -> <out.html>`

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	var board Board
	var out string
	var err error
	switch {
	case len(args) == 4 && args[0] == "github":
		out = args[3]
		board, err = fromGitHub(args[1], args[2])
	case len(args) == 3 && args[0] == "render":
		out = args[2]
		var data []byte
		if args[1] == "-" {
			data, err = io.ReadAll(stdin)
		} else {
			data, err = os.ReadFile(args[1])
		}
		if err != nil {
			return err
		}
		board, err = parseBoard(data)
	default:
		return errors.New(usage)
	}
	if err != nil {
		return err
	}
	var page []byte
	if strings.HasSuffix(out, ".json") {
		page, err = json.MarshalIndent(board, "", "  ")
	} else {
		page, err = renderBoard(board)
	}
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, page, 0o644); err != nil {
		return err
	}
	for _, n := range sortedByState(board.Nodes) {
		fmt.Fprintf(stdout, "%-9s %-4s %s %s  %s\n", n.State, n.Kind, n.Repo, n.Ref, n.Title)
	}
	fmt.Fprintf(stdout, "board: %s\n", out)
	return nil
}

func parseBoard(data []byte) (Board, error) {
	var board Board
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&board); err != nil {
		return Board{}, fmt.Errorf("board JSON: %w", err)
	}
	if dec.More() {
		return Board{}, errors.New("board JSON: trailing data after the board")
	}
	if err := validate(board); err != nil {
		return Board{}, fmt.Errorf("board JSON: %w", err)
	}
	return board, nil
}

func validate(b Board) error {
	if b.Title == "" {
		return errors.New("title is required")
	}
	if len(b.Epics) == 0 {
		return errors.New("epics needs at least one epic")
	}
	epics := map[string]bool{}
	for i, e := range b.Epics {
		switch {
		case e.ID == "" || e.Title == "":
			return fmt.Errorf("epics[%d] needs id and title", i)
		case epics[e.ID]:
			return fmt.Errorf("epics[%d]: duplicate id %q", i, e.ID)
		case e.URL != "" && !isWebURL(e.URL):
			return fmt.Errorf("epics[%d] (%s): url must be an absolute http(s) URL", i, e.ID)
		}
		epics[e.ID] = true
	}
	nodes := map[string]Node{}
	for i, n := range b.Nodes {
		where := fmt.Sprintf("nodes[%d] (%s)", i, n.ID)
		switch {
		case n.ID == "" || n.Title == "" || n.Ref == "":
			return fmt.Errorf("%s needs id, title, and ref", where)
		case nodes[n.ID].ID != "":
			return fmt.Errorf("%s: duplicate id", where)
		case !epics[n.Epic]:
			return fmt.Errorf("%s: epic %q is not in epics", where, n.Epic)
		case !slices.Contains(kinds, n.Kind):
			return fmt.Errorf("%s: kind %q is not one of %s", where, n.Kind, strings.Join(kinds, ", "))
		case !slices.Contains(states, n.State):
			return fmt.Errorf("%s: state %q is not one of %s", where, n.State, strings.Join(states, ", "))
		case !isWebURL(n.URL):
			return fmt.Errorf("%s: url must be an absolute http(s) URL", where)
		case n.Kind == "pr" && len(n.Merged) > 0:
			return fmt.Errorf("%s: merged is for issues; a merged pull request is a ref on its issue", where)
		case n.Kind == "issue" && n.Issue != "":
			return fmt.Errorf("%s: issue is for pull requests", where)
		}
		nodes[n.ID] = n
	}
	for i, n := range b.Nodes {
		if n.Issue == "" {
			continue
		}
		if parent, ok := nodes[n.Issue]; !ok || parent.Kind != "issue" || parent.Epic != n.Epic {
			return fmt.Errorf("nodes[%d] (%s): issue %q must be an issue node in the same epic", i, n.ID, n.Issue)
		}
	}
	seen := map[Edge]bool{}
	for i, e := range b.Edges {
		where := fmt.Sprintf("edges[%d] (%s → %s)", i, e.From, e.To)
		from, okFrom := nodes[e.From]
		to, okTo := nodes[e.To]
		switch {
		case !slices.Contains(links, e.Kind):
			return fmt.Errorf("%s: kind %q is not one of %s", where, e.Kind, strings.Join(links, ", "))
		case !okFrom || !okTo:
			return fmt.Errorf("%s: from and to must be node ids", where)
		case e.From == e.To:
			return fmt.Errorf("%s: a node cannot point at itself", where)
		case e.Kind == "stacked" && (from.Kind != "pr" || to.Kind != "pr"):
			return fmt.Errorf("%s: stacked edges join two pr nodes", where)
		case seen[e]:
			return fmt.Errorf("%s: duplicate edge", where)
		}
		seen[e] = true
	}
	if cycle := findCycle(b); cycle != nil {
		return fmt.Errorf("edges form a cycle: %s", strings.Join(cycle, " → "))
	}
	return nil
}

func isWebURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

func successors(b Board) map[string][]string {
	next := map[string][]string{}
	for _, e := range b.Edges {
		next[e.From] = append(next[e.From], e.To)
	}
	for _, n := range b.Nodes {
		if n.Issue != "" {
			next[n.Issue] = append(next[n.Issue], n.ID)
		}
	}
	return next
}

func findCycle(b Board) []string {
	next := successors(b)
	const (
		visiting = 1
		finished = 2
	)
	mark := map[string]int{}
	var path []string
	var visit func(string) []string
	visit = func(id string) []string {
		mark[id] = visiting
		path = append(path, id)
		for _, to := range next[id] {
			switch mark[to] {
			case visiting:
				start := slices.Index(path, to)
				return append(slices.Clone(path[start:]), to)
			case 0:
				if c := visit(to); c != nil {
					return c
				}
			}
		}
		path = path[:len(path)-1]
		mark[id] = finished
		return nil
	}
	for _, n := range b.Nodes {
		if mark[n.ID] == 0 {
			if c := visit(n.ID); c != nil {
				return c
			}
		}
	}
	return nil
}

func sortedByState(nodes []Node) []Node {
	out := slices.Clone(nodes)
	slices.SortStableFunc(out, func(a, b Node) int {
		return slices.Index(states, a.State) - slices.Index(states, b.State)
	})
	return out
}

const (
	cardW  = 200
	cardH  = 80
	colGap = 56
	rowGap = 16
)

type card struct {
	Node
	X, Y  int
	Word  string
	Dot   string
	Class string
	Meta  string
}

type section struct {
	Epic          Epic
	Count         int
	Width, Height int
	Cards         []card
	Loose         []card
	Edges         template.HTML
}

type needsRow struct {
	Text, Ref, URL string
}

var dots = map[string]string{
	"running":   "live",
	"review":    "live",
	"needs-you": "warn",
	"blocked":   "error",
	"done":      "ok",
}

func layout(b Board, epic Epic, idx int) section {
	var members []Node
	for _, n := range b.Nodes {
		if n.Epic == epic.ID {
			members = append(members, n)
		}
	}
	in := map[string]bool{}
	for _, n := range members {
		in[n.ID] = true
	}
	next := successors(b)
	prev := map[string][]string{}
	for _, n := range members {
		for _, to := range next[n.ID] {
			if in[to] {
				prev[to] = append(prev[to], n.ID)
			}
		}
	}
	linked := map[string]bool{}
	for to, from := range prev {
		linked[to] = true
		for _, f := range from {
			linked[f] = true
		}
	}
	byID := map[string]Node{}
	for _, n := range b.Nodes {
		byID[n.ID] = n
	}
	s := section{Epic: epic, Count: len(members)}
	var graph []Node
	for _, n := range members {
		if linked[n.ID] {
			graph = append(graph, n)
		} else {
			s.Loose = append(s.Loose, newCard(b, n, byID, in, 0, 0))
		}
	}
	members = graph
	depth := map[string]int{}
	var depthOf func(string) int
	depthOf = func(id string) int {
		if d, ok := depth[id]; ok {
			return d
		}
		d := 0
		for _, p := range prev[id] {
			d = max(d, depthOf(p)+1)
		}
		depth[id] = d
		return d
	}
	cols := 0
	for _, n := range members {
		cols = max(cols, depthOf(n.ID)+1)
	}
	row := map[string]float64{}
	columns := make([][]Node, cols)
	for c := range cols {
		for _, n := range members {
			if depth[n.ID] == c {
				columns[c] = append(columns[c], n)
			}
		}
		weight := func(n Node) float64 {
			if len(prev[n.ID]) == 0 {
				return float64(slices.IndexFunc(members, func(m Node) bool { return m.ID == n.ID }))
			}
			sum := 0.0
			for _, p := range prev[n.ID] {
				sum += row[p]
			}
			return sum / float64(len(prev[n.ID]))
		}
		slices.SortStableFunc(columns[c], func(a, b Node) int {
			wa, wb := weight(a), weight(b)
			switch {
			case wa < wb:
				return -1
			case wa > wb:
				return 1
			}
			return 0
		})
		for r, n := range columns[c] {
			row[n.ID] = float64(r)
		}
	}
	rows := 0
	for _, col := range columns {
		rows = max(rows, len(col))
	}
	if cols == 0 {
		return s
	}
	s.Width = cols*cardW + (cols-1)*colGap
	s.Height = rows*cardH + (rows-1)*rowGap
	pos := map[string][2]int{}
	for c, col := range columns {
		for r, n := range col {
			x, y := c*(cardW+colGap), r*(cardH+rowGap)
			pos[n.ID] = [2]int{x, y}
			s.Cards = append(s.Cards, newCard(b, n, byID, in, x, y))
		}
	}
	var svg strings.Builder
	fmt.Fprintf(&svg, `<svg class="edges" width="%d" height="%d" aria-hidden="true"><defs>`, s.Width, s.Height)
	for _, kind := range links {
		fmt.Fprintf(&svg, `<marker id="%s-%d" viewBox="0 0 6 6" refX="6" refY="3" markerWidth="6" markerHeight="6" markerUnits="userSpaceOnUse" orient="auto"><path d="M0,0 L6,3 L0,6 z" class="head %s"/></marker>`, kind, idx, kind)
	}
	svg.WriteString(`</defs>`)
	for _, e := range b.Edges {
		from, okFrom := pos[e.From]
		to, okTo := pos[e.To]
		if !okFrom || !okTo {
			continue
		}
		x1, y1 := from[0]+cardW, from[1]+cardH/2
		x2, y2 := to[0], to[1]+cardH/2
		dx := (x2 - x1) / 2
		fmt.Fprintf(&svg, `<path class="e %s" d="M%d,%d C%d,%d %d,%d %d,%d" marker-end="url(#%s-%d)"/>`,
			e.Kind, x1, y1, x1+dx, y1, x2-dx, y2, x2-1, y2, e.Kind, idx)
	}
	svg.WriteString(`</svg>`)
	s.Edges = template.HTML(svg.String())
	return s
}

func newCard(b Board, n Node, byID map[string]Node, inEpic map[string]bool, x, y int) card {
	meta := []string{strings.TrimSpace(shortRepo(n.Repo) + " " + n.Ref)}
	if len(n.Merged) > 0 {
		meta = append(meta, "merged "+strings.Join(n.Merged, ", "))
	}
	for _, e := range b.Edges {
		if e.Kind == "blocked-by" && e.To == n.ID && !inEpic[e.From] {
			from := byID[e.From]
			meta = append(meta, "after "+strings.TrimSpace(shortRepo(from.Repo)+" "+from.Ref))
		}
	}
	return card{
		Node:  n,
		X:     x,
		Y:     y,
		Word:  strings.ReplaceAll(n.State, "-", " "),
		Dot:   dots[n.State],
		Class: n.Kind + " " + n.State,
		Meta:  strings.Join(meta, " · "),
	}
}

func shortRepo(repo string) string {
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		return repo[i+1:]
	}
	return repo
}

var page = template.Must(template.New("board").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<link rel="stylesheet" href="https://design.uinaf.dev/tokens.css">
<style>
body{margin:0}
.board{padding:20px;border-radius:var(--radius-md);font-size:13px;line-height:1.3}
.board a{color:inherit;text-decoration:none}
.lbl{display:flex;gap:8px;margin:0 0 12px;font-size:10px;letter-spacing:var(--tracking-caps);text-transform:uppercase;color:var(--fg-subtle)}
.lbl .n{color:var(--fg-faint)}
.lbl a:hover{color:var(--fg)}
.needs a{display:flex;align-items:center;gap:10px;padding:8px 0;border-top:1px solid var(--border-soft)}
.needs a:first-of-type{border-top:0;padding-top:0}
.needs .q{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.needs .r{font-size:11px;color:var(--fg-subtle);white-space:nowrap}
.needs a:hover .q{color:var(--fg-strong)}
.band{display:flex;flex-wrap:wrap;gap:0 32px;padding-top:20px;margin-top:24px;border-top:1px solid var(--border-soft)}
.band:first-child{margin-top:0}
.epic{max-width:100%;overflow-x:auto}
.canvas{position:relative}
.canvas .card{position:absolute}
.loose{display:flex;flex-wrap:wrap;gap:16px}
.canvas+.loose{margin-top:16px}
.edges{position:absolute;inset:0;overflow:visible;pointer-events:none}
.e{fill:none;stroke:var(--neutral-700);stroke-width:1}
.e.stacked{stroke:var(--neutral-500);stroke-width:1.5}
.head{fill:var(--neutral-700)}
.head.stacked{fill:var(--neutral-500)}
.card{flex:none;box-sizing:border-box;width:200px;height:80px;padding:10px 12px;display:flex;flex-direction:column;justify-content:space-between;background:var(--bg-raised);border:1px solid var(--border);border-radius:5px;transition:border-color var(--duration-fast) var(--ease-snappy)}
.card.pr{border-style:dashed}
.card.needs-you{border-color:color-mix(in srgb,var(--warn) 35%,var(--border))}
.card.done{opacity:.42}
.card:hover{border-color:var(--border-hover)}
.st{display:flex;align-items:center;gap:6px;font-size:10px;color:var(--fg-subtle)}
.t,.m{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.t{color:var(--fg)}
.m{font-size:11px;color:var(--fg-subtle)}
</style></head>
<body><div class="uinaf board">
{{- if .Needs}}
<div class="needs"><div class="lbl">needs you <span class="n">{{len .Needs}}</span></div>
{{- range .Needs}}
<a href="{{.URL}}"><span class="u-dot u-dot--warn"></span><span class="q">{{.Text}}</span><span class="r">{{.Ref}}</span></a>
{{- end}}
</div>
{{- end}}
{{- range .Bands}}
<div class="band">
{{- range .}}
<section class="epic">
<div class="lbl">{{if .Epic.URL}}<a href="{{.Epic.URL}}">{{.Epic.Title}}</a>{{else}}{{.Epic.Title}}{{end}} <span class="n">{{.Count}}</span></div>
{{- if .Cards}}
<div class="canvas" style="width:{{.Width}}px;height:{{.Height}}px">{{.Edges}}
{{- range .Cards}}
<a class="card {{.Class}}" href="{{.URL}}" title="{{.Title}}" style="left:{{.X}}px;top:{{.Y}}px">{{template "card" .}}</a>
{{- end}}
</div>
{{- end}}
{{- if .Loose}}
<div class="loose">
{{- range .Loose}}
<a class="card {{.Class}}" href="{{.URL}}" title="{{.Title}}">{{template "card" .}}</a>
{{- end}}
</div>
{{- end}}
</section>
{{- end}}
</div>
{{- end}}
</div></body></html>
{{- define "card"}}<span class="st"><span class="u-dot u-dot--{{.Dot}}"></span>{{.Word}}</span><span class="t">{{.Title}}</span><span class="m">{{.Meta}}</span>{{end}}
`))

func renderBoard(b Board) ([]byte, error) {
	data := struct {
		Title string
		Needs []needsRow
		Bands [][]section
	}{Title: b.Title}
	for _, n := range b.Nodes {
		if n.State != "needs-you" {
			continue
		}
		text := n.Question
		if text == "" {
			text = n.Title
		}
		data.Needs = append(data.Needs, needsRow{Text: text, Ref: strings.TrimSpace(shortRepo(n.Repo) + " " + n.Ref), URL: n.URL})
	}
	single := false
	for i, e := range b.Epics {
		s := layout(b, e, i)
		if s.Count == 0 {
			continue
		}
		if s.Count == 1 && single {
			last := len(data.Bands) - 1
			data.Bands[last] = append(data.Bands[last], s)
			continue
		}
		data.Bands = append(data.Bands, []section{s})
		single = s.Count == 1
	}
	var buf bytes.Buffer
	if err := page.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type ghIssue struct {
	ID         string `json:"id"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	State      string `json:"state"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	SubIssues struct {
		TotalCount int `json:"totalCount"`
	} `json:"subIssues"`
	BlockedBy struct {
		TotalCount int `json:"totalCount"`
		Nodes      []struct {
			Number     int    `json:"number"`
			State      string `json:"state"`
			Repository struct {
				NameWithOwner string `json:"nameWithOwner"`
			} `json:"repository"`
		} `json:"nodes"`
	} `json:"blockedBy"`
	PullRequests struct {
		TotalCount int    `json:"totalCount"`
		Nodes      []ghPR `json:"nodes"`
	} `json:"closedByPullRequestsReferences"`
	Comments struct {
		Nodes []struct {
			Body string `json:"body"`
		} `json:"nodes"`
	} `json:"comments"`
}

type ghPR struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	State       string `json:"state"`
	IsDraft     bool   `json:"isDraft"`
	BaseRefName string `json:"baseRefName"`
	HeadRefName string `json:"headRefName"`
	Repository  struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
}

type ghChildren struct {
	PageInfo struct {
		HasNextPage bool `json:"hasNextPage"`
	} `json:"pageInfo"`
	Nodes []ghIssue `json:"nodes"`
}

const issueFields = `
fragment fields on Issue {
  id number title url state
  repository { nameWithOwner }
  labels(first: 50) { nodes { name } }
  subIssues(first: 1) { totalCount }
  blockedBy(first: 50) { totalCount nodes { number state repository { nameWithOwner } } }
  closedByPullRequestsReferences(first: 50, includeClosedPrs: true) {
    totalCount
    nodes { number title url state isDraft baseRefName headRefName repository { nameWithOwner } }
  }
  comments(last: 10) { nodes { body } }
}`

const rootQuery = `query($owner: String!, $name: String!, $number: Int!) {
  repository(owner: $owner, name: $name) {
    issue(number: $number) { title url subIssues(first: 100) { pageInfo { hasNextPage } nodes { ...fields } } }
  }
}` + issueFields

const epicQuery = `query($id: ID!) {
  node(id: $id) { ... on Issue { subIssues(first: 100) { pageInfo { hasNextPage } nodes { ...fields } } } }
}` + issueFields

func graphql(query string, vars map[string]string, into any) error {
	args := []string{"api", "graphql", "-f", "query=" + query}
	for k, v := range vars {
		flag := "-f"
		if k == "number" {
			flag = "-F"
		}
		args = append(args, flag, k+"="+v)
	}
	var stderr bytes.Buffer
	cmd := exec.Command("gh", args...)
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("gh api graphql: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("gh api graphql: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("gh api graphql: %s", envelope.Errors[0].Message)
	}
	if err := json.Unmarshal(envelope.Data, into); err != nil {
		return fmt.Errorf("gh api graphql: %w", err)
	}
	return nil
}

func fromGitHub(repo, number string) (Board, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return Board{}, fmt.Errorf("repository %q is not owner/repo", repo)
	}
	if n, err := strconv.Atoi(number); err != nil || n <= 0 {
		return Board{}, fmt.Errorf("root issue %q is not an issue number", number)
	}
	var root struct {
		Repository struct {
			Issue *struct {
				Title     string     `json:"title"`
				URL       string     `json:"url"`
				SubIssues ghChildren `json:"subIssues"`
			} `json:"issue"`
		} `json:"repository"`
	}
	if err := graphql(rootQuery, map[string]string{"owner": owner, "name": name, "number": number}, &root); err != nil {
		return Board{}, err
	}
	top := root.Repository.Issue
	if top == nil {
		return Board{}, fmt.Errorf("%s#%s is not an issue", repo, number)
	}
	if top.SubIssues.PageInfo.HasNextPage {
		return Board{}, fmt.Errorf("%s#%s has more than 100 sub-issues", repo, number)
	}
	b := Board{Title: top.Title, Edges: []Edge{}}
	rootEpic := Epic{ID: fmt.Sprintf("%s#%s", repo, number), Title: top.Title, URL: top.URL}
	var leaves []ghIssue
	type group struct {
		epic   Epic
		issues []ghIssue
	}
	var groups []group
	for _, sub := range top.SubIssues.Nodes {
		if sub.SubIssues.TotalCount == 0 {
			leaves = append(leaves, sub)
			continue
		}
		var epic struct {
			Node struct {
				SubIssues ghChildren `json:"subIssues"`
			} `json:"node"`
		}
		if err := graphql(epicQuery, map[string]string{"id": sub.ID}, &epic); err != nil {
			return Board{}, err
		}
		if epic.Node.SubIssues.PageInfo.HasNextPage {
			return Board{}, fmt.Errorf("%s has more than 100 sub-issues", issueRef(sub.Repository.NameWithOwner, sub.Number))
		}
		groups = append(groups, group{Epic{ID: issueRef(sub.Repository.NameWithOwner, sub.Number), Title: sub.Title, URL: sub.URL}, epic.Node.SubIssues.Nodes})
	}
	if len(leaves) > 0 {
		groups = append([]group{{rootEpic, leaves}}, groups...)
	}
	if len(groups) == 0 {
		groups = []group{{rootEpic, nil}}
	}
	var blockers [][2]string
	var prs []Node
	prRefs := map[string]ghPR{}
	for _, g := range groups {
		b.Epics = append(b.Epics, g.epic)
		for _, is := range g.issues {
			id := issueRef(is.Repository.NameWithOwner, is.Number)
			if is.BlockedBy.TotalCount > len(is.BlockedBy.Nodes) || is.PullRequests.TotalCount > len(is.PullRequests.Nodes) {
				return Board{}, fmt.Errorf("%s has more blockers or pull requests than one page holds", id)
			}
			node := Node{ID: id, Epic: g.epic.ID, Kind: "issue", Title: is.Title, Ref: fmt.Sprintf("#%d", is.Number), Repo: is.Repository.NameWithOwner, URL: is.URL}
			openBlocker := false
			for _, bl := range is.BlockedBy.Nodes {
				blockers = append(blockers, [2]string{issueRef(bl.Repository.NameWithOwner, bl.Number), id})
				openBlocker = openBlocker || bl.State == "OPEN"
			}
			needsUser := slices.ContainsFunc(is.Labels.Nodes, func(l struct {
				Name string `json:"name"`
			}) bool {
				return l.Name == "needs-user"
			})
			switch {
			case is.State != "OPEN":
				node.State = "done"
			case needsUser:
				node.State = "needs-you"
				for i := len(is.Comments.Nodes) - 1; i >= 0 && node.Question == ""; i-- {
					node.Question = lastQuestion(is.Comments.Nodes[i].Body)
				}
			case openBlocker:
				node.State = "blocked"
			default:
				node.State = "running"
			}
			for _, pr := range is.PullRequests.Nodes {
				ref := fmt.Sprintf("#%d", pr.Number)
				if pr.Repository.NameWithOwner != node.Repo {
					ref = shortRepo(pr.Repository.NameWithOwner) + ref
				}
				switch pr.State {
				case "MERGED":
					node.Merged = append(node.Merged, ref)
				case "OPEN":
					prID := issueRef(pr.Repository.NameWithOwner, pr.Number)
					if _, dup := prRefs[prID]; dup {
						continue
					}
					prRefs[prID] = pr
					state := "review"
					if pr.IsDraft {
						state = "running"
					}
					prs = append(prs, Node{ID: prID, Epic: g.epic.ID, Kind: "pr", State: state, Title: pr.Title, Ref: fmt.Sprintf("#%d", pr.Number), Repo: pr.Repository.NameWithOwner, URL: pr.URL, Issue: id})
				}
			}
			b.Nodes = append(b.Nodes, node)
		}
	}
	b.Nodes = append(b.Nodes, prs...)
	onBoard := map[string]bool{}
	for _, n := range b.Nodes {
		onBoard[n.ID] = true
	}
	for _, bl := range blockers {
		edge := Edge{Kind: "blocked-by", From: bl[0], To: bl[1]}
		if onBoard[bl[0]] && !slices.Contains(b.Edges, edge) {
			b.Edges = append(b.Edges, edge)
		}
	}
	for baseID, base := range prRefs {
		for nextID, next := range prRefs {
			if baseID != nextID && base.Repository.NameWithOwner == next.Repository.NameWithOwner && next.BaseRefName == base.HeadRefName {
				b.Edges = append(b.Edges, Edge{Kind: "stacked", From: baseID, To: nextID})
			}
		}
	}
	slices.SortStableFunc(b.Edges, func(x, y Edge) int { return strings.Compare(x.Kind+x.From+x.To, y.Kind+y.From+y.To) })
	if err := validate(b); err != nil {
		return Board{}, fmt.Errorf("board from %s#%s: %w", repo, number, err)
	}
	return b, nil
}

func issueRef(repo string, number int) string {
	return fmt.Sprintf("%s#%d", repo, number)
}

func lastQuestion(body string) string {
	lines := strings.Split(body, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(lines[i]), "#>-* "))
		line = strings.Trim(line, "*_")
		if strings.HasSuffix(line, "?") {
			return line
		}
	}
	return ""
}
