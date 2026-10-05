// Command bodycheck lints a change-request body against slopcourier's rules
// before it is posted. Run it from the repository the change request targets:
//
//	go run <skill dir>/scripts/bodycheck/main.go [flags] <body.md | ->
//
// It prints one line per problem and exits 1, or exits 0 silently.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type limits struct {
	prose, total, bullets int
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("bodycheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	templatePath := flags.String("template", "", "template whose headings the body keeps (default: the repository's change-request template)")
	beforePath := flags.String("before", "", "body this one replaces; flags closing references it drops")
	footer := flags.String("footer", "", "line the body must end with, such as an attribution")
	var lim limits
	flags.IntVar(&lim.prose, "prose", 75, "maximum words outside tables, code, and collapsed details")
	flags.IntVar(&lim.total, "total", 150, "maximum words including tables")
	flags.IntVar(&lim.bullets, "bullets", 3, "maximum top-level bullets outside the validation section")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: bodycheck [flags] <body.md | ->")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}

	body, err := readInput(flags.Arg(0), stdin)
	if err != nil {
		fmt.Fprintln(stderr, "bodycheck:", err)
		return 2
	}
	var template, before string
	if *templatePath != "" {
		template, err = readInput(*templatePath, nil)
	} else {
		var dir string
		if dir, err = os.Getwd(); err == nil {
			template, err = findTemplate(dir)
		}
	}
	if err == nil && *beforePath != "" {
		before, err = readInput(*beforePath, nil)
	}
	if err != nil {
		fmt.Fprintln(stderr, "bodycheck:", err)
		return 2
	}

	problems := check(body, template, before, *footer, lim)
	for _, problem := range problems {
		fmt.Fprintln(stdout, problem)
	}
	if len(problems) > 0 {
		return 1
	}
	return 0
}

func readInput(path string, stdin io.Reader) (string, error) {
	if path == "-" && stdin != nil {
		data, err := io.ReadAll(stdin)
		return string(data), err
	}
	data, err := os.ReadFile(path)
	return string(data), err
}

// templatePaths lists where GitHub and GitLab look for a default template,
// relative to the repository root.
var templatePaths = []string{
	".github/pull_request_template.md",
	".github/PULL_REQUEST_TEMPLATE.md",
	"pull_request_template.md",
	"PULL_REQUEST_TEMPLATE.md",
	"docs/pull_request_template.md",
	"docs/PULL_REQUEST_TEMPLATE.md",
	".gitlab/merge_request_templates/Default.md",
}

// findTemplate returns the template of the repository containing dir, or ""
// when dir is outside a repository or the repository has none.
func findTemplate(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
	for _, path := range templatePaths {
		data, err := os.ReadFile(filepath.Join(dir, path))
		if err == nil {
			return string(data), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return "", nil
}

type lineKind int

const (
	proseLine lineKind = iota
	tableLine
	codeLine
	detailsLine
	headingKind
)

type line struct {
	text    string
	kind    lineKind
	heading string
}

var (
	fenceStart   = regexp.MustCompile("^\\s*(```+|~~~+)")
	tableRow     = regexp.MustCompile(`^\s*\|.*\|\s*$`)
	tableDivider = regexp.MustCompile(`(?m)^\s*\|\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|\s*$`)
	headingLine  = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*#*\s*$`)
	bulletLine   = regexp.MustCompile(`^[-*+]\s+`)
	taskItem     = regexp.MustCompile(`^\s*[-*+]\s+\[[ xX]\]`)
	uncheckedBox = regexp.MustCompile(`^\s*[-*+]\s+\[ \]`)
	proofHeading = regexp.MustCompile(`(?i)validat|verif|test|proof|check`)
	mediaEmbed   = regexp.MustCompile(`(?i)<img\s|<video\s|!\[[^\]]*\]\(|github\.com/user-attachments/assets/|/uploads/[0-9a-f]{32}/`)
	imageRef     = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	linkRef      = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	bareURL      = regexp.MustCompile(`https?://\S+`)
	htmlTag      = regexp.MustCompile(`<[^>]+>`)
	testCount    = regexp.MustCompile(`(?i)\b\d[\d,]*\s+(?:passed|passing|failed|failing|skipped|tests?|assertions)\b|\b\d+/\d+\s+(?:passed|tests?)\b`)
	fullSHA      = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
	closingRef   = regexp.MustCompile(`(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?):?\s+((?:[\w.-]+/[\w.-]+)?#\d+|https?://\S+/(?:issues|pull|merge_requests)/\d+)`)
)

// classify splits a body into lines tagged as prose, heading, table, code,
// or collapsed details, each with the heading it sits under.
func classify(body string) []line {
	var lines []line
	var fence, heading string
	inDetails := false
	for _, text := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		item := line{text: text, heading: heading}
		switch {
		case fence != "":
			item.kind = codeLine
			if strings.HasPrefix(strings.TrimSpace(text), fence) {
				fence = ""
			}
		case fenceStart.MatchString(text):
			item.kind = codeLine
			fence = fenceStart.FindStringSubmatch(text)[1]
		case inDetails || strings.Contains(text, "<details"):
			item.kind = detailsLine
			inDetails = !strings.Contains(text, "</details>")
		case tableRow.MatchString(text):
			item.kind = tableLine
		default:
			if match := headingLine.FindStringSubmatch(text); match != nil {
				heading = match[1]
				item.heading = heading
				item.kind = headingKind
			}
		}
		lines = append(lines, item)
	}
	return lines
}

func has(lines []line, kind lineKind) bool {
	for _, item := range lines {
		if item.kind == kind {
			return true
		}
	}
	return false
}

func joined(lines []line, kinds ...lineKind) string {
	var parts []string
	for _, item := range lines {
		for _, kind := range kinds {
			if item.kind == kind {
				parts = append(parts, item.text)
				break
			}
		}
	}
	return strings.Join(parts, "\n")
}

// readable drops markup so only the words a reader sees remain.
func readable(text string) string {
	text = imageRef.ReplaceAllString(text, " ")
	text = linkRef.ReplaceAllString(text, "$1")
	text = bareURL.ReplaceAllString(text, " ")
	text = htmlTag.ReplaceAllString(text, " ")
	var lines []string
	for _, item := range strings.Split(text, "\n") {
		lines = append(lines, taskItem.ReplaceAllString(item, ""))
	}
	return strings.Join(lines, "\n")
}

func countWords(text string) int {
	count := 0
	for _, field := range strings.Fields(text) {
		if strings.IndexFunc(field, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			count++
		}
	}
	return count
}

func headings(text string) []string {
	var found []string
	for _, item := range classify(text) {
		if item.kind == headingKind {
			found = append(found, strings.TrimSpace(item.text))
		}
	}
	return found
}

func closingRefs(text string) map[string]string {
	refs := map[string]string{}
	for _, match := range closingRef.FindAllStringSubmatch(text, -1) {
		refs[strings.ToLower(match[1])] = match[0]
	}
	return refs
}

func check(body, template, before, footer string, lim limits) []string {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}
	counted := body
	if footer != "" {
		if lines := strings.Split(strings.TrimRight(body, "\n\t "), "\n"); strings.TrimSpace(lines[len(lines)-1]) != footer {
			add("does not end with %q", footer)
		}
		counted = strings.Replace(body, footer, "", 1)
	}

	lines := classify(counted)
	prose := readable(joined(lines, proseLine))
	if !has(lines, codeLine) && !tableDivider.MatchString(joined(lines, tableLine)) && !mediaEmbed.MatchString(body) {
		add("no visual aid: add a table, Mermaid diagram, short code sample, screenshot, or recording")
	}
	if n := countWords(prose); n > lim.prose {
		add("%d words outside tables and code (limit %d): let the visual aid carry the facts", n, lim.prose)
	}
	if n := countWords(readable(joined(lines, proseLine, tableLine))); n > lim.total {
		add("%d words including tables (limit %d)", n, lim.total)
	}
	bullets := 0
	for _, item := range lines {
		if item.kind == proseLine && bulletLine.MatchString(item.text) && !taskItem.MatchString(item.text) && !proofHeading.MatchString(item.heading) {
			bullets++
		}
	}
	if bullets > lim.bullets {
		add("%d bullets outside the validation section (limit %d): keep only risks the visual aid doesn't show", bullets, lim.bullets)
	}
	if match := testCount.FindString(prose); match != "" {
		add("test count %q: CI shows results; name what the proof covers instead", match)
	}
	if sha := fullSHA.FindString(readable(joined(lines, proseLine, tableLine))); sha != "" {
		add("full commit SHA %s in the text: link the commit or drop it", sha[:12])
	}
	for _, item := range lines {
		if item.kind == proseLine && uncheckedBox.MatchString(item.text) {
			add("unchecked task item %q: tick what ran and say what was skipped", strings.TrimSpace(item.text))
		}
	}
	if strings.Contains(joined(lines, proseLine, tableLine, detailsLine), "<!--") {
		add("template comment left in the body")
	}
	have := map[string]bool{}
	for _, heading := range headings(body) {
		have[strings.ToLower(heading)] = true
	}
	for _, heading := range headings(template) {
		if !have[strings.ToLower(heading)] {
			add("missing template heading %q", heading)
		}
	}
	kept, previous := closingRefs(body), closingRefs(before)
	refs := make([]string, 0, len(previous))
	for ref := range previous {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	for _, ref := range refs {
		if _, ok := kept[ref]; !ok {
			add("drops %q from the previous body", previous[ref])
		}
	}
	return problems
}
