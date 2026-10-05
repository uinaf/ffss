package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var defaults = limits{prose: 75, total: 150, bullets: 3}

const template = `## Change

<!-- One sentence, then one visual aid. -->

## Validation

- [ ] Checks
`

const good = `## Change

Search retries after a cancelled retry instead of showing a stale failure.

| Case | Before | After |
| --- | --- | --- |
| Retry cancelled, tab reappears | failure banner, nothing in flight | searches again |

- Risk: one extra request when the tab returns during a retry.

Closes #31

## Validation

- [x] Unit and journey suites; the regression fails on the old code
- Unverified: a real network loss.

---
Written by an agent (Claude Code, Opus 5.5)
`

func problemsFor(t *testing.T, body string) []string {
	t.Helper()
	return check(body, template, "", "", defaults)
}

func requireProblem(t *testing.T, problems []string, fragment string) {
	t.Helper()
	for _, problem := range problems {
		if strings.Contains(problem, fragment) {
			return
		}
	}
	t.Fatalf("no problem containing %q in %q", fragment, problems)
}

func TestAcceptsShortBodyWithVisualAid(t *testing.T) {
	if problems := check(good, template, "Fixes #31", "Written by an agent (Claude Code, Opus 5.5)", defaults); len(problems) > 0 {
		t.Fatalf("check(good) = %q", problems)
	}
}

func TestRequiresVisualAid(t *testing.T) {
	body := "## Change\n\nSearch retries.\n\n## Validation\n\n- [x] Tests\n"
	requireProblem(t, problemsFor(t, body), "no visual aid")

	for name, aid := range map[string]string{
		"mermaid": "```mermaid\nflowchart LR\n  A --> B\n```",
		"image":   `<img src="https://github.com/user-attachments/assets/abc" width="300">`,
		"video":   "https://github.com/user-attachments/assets/0f3c2a",
		"code":    "~~~go\nfunc New() {}\n~~~",
	} {
		withAid := strings.Replace(body, "Search retries.", "Search retries.\n\n"+aid, 1)
		if problems := problemsFor(t, withAid); len(problems) > 0 {
			t.Fatalf("%s: %q", name, problems)
		}
	}
}

func TestProseLimitIgnoresTablesCodeAndDetails(t *testing.T) {
	words := strings.Repeat("word ", 70)
	rows := strings.Repeat("| "+strings.Repeat("cell ", 10)+"|\n", 3)
	body := "## Change\n\n" + words + "\n\n| a |\n| --- |\n" + rows +
		"\n```diff\n" + strings.Repeat("- removed line of code\n", 40) + "```\n" +
		"\n<details><summary>More</summary>\n\n" + strings.Repeat("hidden ", 200) + "\n\n</details>\n\n## Validation\n\n- [x] Checks\n"
	if problems := problemsFor(t, body); len(problems) > 0 {
		t.Fatalf("check = %q", problems)
	}

	longer := strings.Replace(body, words, words+"one two three four five six", 1)
	requireProblem(t, problemsFor(t, longer), "words outside tables and code")
}

func TestTotalLimitCountsTables(t *testing.T) {
	rows := strings.Repeat("| "+strings.Repeat("cell ", 20)+"|\n", 8)
	body := "## Change\n\nShort.\n\n| a |\n| --- |\n" + rows + "\n## Validation\n\n- [x] Checks\n"
	requireProblem(t, problemsFor(t, body), "words including tables")
}

func TestCountsBulletsOutsideValidation(t *testing.T) {
	body := "## Change\n\nOutcome.\n\n| a |\n| --- |\n| b |\n\n- one\n- two\n- three\n\n```diff\n- old\n- older\n```\n\n## Validation\n\n- [x] a\n- b\n- c\n- d\n"
	if problems := problemsFor(t, body); len(problems) > 0 {
		t.Fatalf("check = %q", problems)
	}
	requireProblem(t, problemsFor(t, strings.Replace(body, "- three\n", "- three\n- four\n", 1)), "4 bullets")
}

func TestFlagsTestCountsInProseOnly(t *testing.T) {
	inTable := strings.Replace(good, "| searches again |", "| 500 of 500 passed |", 1)
	if problems := problemsFor(t, inTable); len(problems) > 0 {
		t.Fatalf("table count flagged: %q", problems)
	}
	inProse := strings.Replace(good, "Unit and journey suites", "106 passed", 1)
	requireProblem(t, problemsFor(t, inProse), `test count "106 passed"`)
}

func TestFlagsFullSHAOutsideLinksAndCode(t *testing.T) {
	sha := "5df79196060f2c3b9a8e7d6c5b4a39281706f5e4"
	linked := strings.Replace(good, "Closes #31", "Closes #31, [fix](https://github.com/o/r/commit/"+sha+"), https://github.com/o/r/commit/"+sha, 1)
	if problems := problemsFor(t, linked); len(problems) > 0 {
		t.Fatalf("linked SHA flagged: %q", problems)
	}
	requireProblem(t, problemsFor(t, strings.Replace(good, "Closes #31", "Fixed in "+sha+".\n\nCloses #31", 1)), "full commit SHA 5df79196060f")
}

func TestFlagsTemplateLeftovers(t *testing.T) {
	body := strings.Replace(good, "- [x] Unit", "<!-- Link only proof CI cannot show. -->\n- [ ] Unit", 1)
	problems := problemsFor(t, body)
	requireProblem(t, problems, "unchecked task item")
	requireProblem(t, problems, "template comment")
}

func TestRequiresTemplateHeadings(t *testing.T) {
	body := strings.Replace(good, "## Validation", "## Testing", 1)
	requireProblem(t, problemsFor(t, body), `missing template heading "## Validation"`)
}

func TestAllowsDroppingTemplateSectionsMarkedDeletable(t *testing.T) {
	deletable := `## Problem

<!-- as the requester stated it; screenshots are optional -->

## Solution

### Risks

## Proof

<!-- only what CI cannot show.
     Delete this section when CI covers everything. -->

### Media

## Notes (optional)
`
	body := "## Problem\n\nStale failure.\n\n## Solution\n\n### Risks\n\n| a |\n| --- |\n"
	for _, problem := range check(body, deletable, "", "", defaults) {
		if strings.Contains(problem, "missing template heading") {
			t.Fatalf("deletable section required: %q", problem)
		}
	}

	problems := check(strings.Replace(body, "### Risks", "", 1), deletable, "", "", defaults)
	requireProblem(t, problems, `missing template heading "### Risks"`)
	problems = check(strings.Replace(body, "## Problem", "## Context", 1), deletable, "", "", defaults)
	requireProblem(t, problems, `missing template heading "## Problem"`)
}

func TestFlagsDroppedClosingReferences(t *testing.T) {
	before := "Closes #31\nFixes putdotio/putio-ios#5\nResolves: https://gitlab.com/g/p/-/issues/9\nRefs #2"
	problems := check(good, template, before, "", defaults)
	if len(problems) != 2 {
		t.Fatalf("check = %q", problems)
	}
	requireProblem(t, problems, `drops "Fixes putdotio/putio-ios#5"`)
	requireProblem(t, problems, `drops "Resolves: https://gitlab.com/g/p/-/issues/9"`)
}

func TestRequiresFooterAsLastLineWithoutCountingIt(t *testing.T) {
	footer := "Written by an agent (Claude Code, Opus 5.5)"
	requireProblem(t, check(good+"\nP.S.\n", template, "", footer, defaults), "does not end with")

	atLimit := "## Change\n\n" + strings.Repeat("word ", 75) + "\n\n| a |\n| --- |\n\n## Validation\n\n---\n" + footer + "\n"
	if problems := check(atLimit, template, "", footer, defaults); len(problems) > 0 {
		t.Fatalf("footer counted: %q", problems)
	}
}

func TestFindsTemplateAtRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	for _, dir := range []string{filepath.Join(root, ".git"), filepath.Join(root, ".github"), nested} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := findTemplate(nested); err != nil || got != "" {
		t.Fatalf("without a template: %q, %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github", "pull_request_template.md"), []byte(template), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := findTemplate(nested); err != nil || got != template {
		t.Fatalf("findTemplate = %q, %v", got, err)
	}
}

func TestRunExitCodes(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-"}, strings.NewReader(good), &stdout, &stderr); code != 0 {
		t.Fatalf("good body: exit %d, %q", code, stdout.String())
	}
	if code := run([]string{"-"}, strings.NewReader("Just words."), &stdout, &stderr); code != 1 || !strings.Contains(stdout.String(), "no visual aid") {
		t.Fatalf("bad body: exit %d, %q", code, stdout.String())
	}
	if code := run([]string{"missing.md"}, nil, &stdout, &stderr); code != 2 {
		t.Fatalf("missing file: exit %d", code)
	}
}
