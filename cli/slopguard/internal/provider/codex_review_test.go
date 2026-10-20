package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

func TestDecodeCodexReviewText(t *testing.T) {
	t.Parallel()

	repository := t.TempDir()
	resolved, err := filepath.EvalSymlinks(repository)
	if err != nil {
		t.Fatal(err)
	}
	findings := "Full-context reads bypass the byte budget.\n\nFull review comments:\n\n" +
		"- [P1] Charge reads to the budget — " + filepath.Join(resolved, "cli/redact.go") + ":129-131\n" +
		"  Each read gets the whole allowance.\n\n  Charge it to the shared budget.\n\n" +
		"- [P3] Name the limit — or not — " + filepath.Join(repository, "docs/a — b.md") + ":7\n" +
		"  The doc says otherwise.\n"
	output, err := decodeCodexReviewText(findings, repository)
	if err != nil {
		t.Fatalf("decode findings: %v", err)
	}
	review, err := protocol.DecodeReview(output)
	if err != nil {
		t.Fatalf("DecodeReview: %v\n%s", err, output)
	}
	if review.OverallExplanation != "Full-context reads bypass the byte budget." || len(review.Findings) != 2 {
		t.Fatalf("review = %+v", review)
	}
	first, second := review.Findings[0], review.Findings[1]
	if first.Priority != protocol.PriorityP1 || first.Title != "Charge reads to the budget" || first.Location != (protocol.Location{FilePath: "cli/redact.go", StartLine: 129, EndLine: 131}) {
		t.Errorf("first finding = %+v", first)
	}
	if first.Body != "Each read gets the whole allowance.\n\nCharge it to the shared budget." {
		t.Errorf("first body = %q", first.Body)
	}
	if second.Title != "Name the limit — or not" || second.Location != (protocol.Location{FilePath: "docs/a — b.md", StartLine: 7, EndLine: 7}) {
		t.Errorf("second location = %+v", second.Location)
	}

	clean, err := decodeCodexReviewText("No actionable regressions were found.", repository)
	if err != nil {
		t.Fatalf("decode clean: %v", err)
	}
	if review, err := protocol.DecodeReview(clean); err != nil || len(review.Findings) != 0 {
		t.Fatalf("clean review = %+v, %v", review, err)
	}

	outside := filepath.Join(filepath.Dir(repository), "elsewhere.go")
	for name, text := range map[string]string{
		"finding before heading":  "Summary.\n- [P1] Bug — a.go:1\n  Body.",
		"heading without finding": "Summary.\n\nReview comment:\n\n",
		"unrecognized line":       "Summary.\n\nReview comment:\n\n- [P1] Bug — a.go:1\n  Body.\nstray text",
		"path outside repository": "Summary.\n\nReview comment:\n\n- [P1] Bug — " + outside + ":1\n  Body.",
		"empty summary":           "Review comment:\n\n- [P1] Bug — /a.go:1\n  Body.",
		"relative path":           "Summary.\n\nReview comment:\n\n- [P1] Bug — a.go:1\n  Body.",
	} {
		if _, err := decodeCodexReviewText(text, repository); err == nil {
			t.Errorf("%s: decode succeeded", name)
		}
	}
}

func TestCodexReviewArgumentsStayReadOnlyWithoutProjectInstructions(t *testing.T) {
	t.Parallel()

	arguments := strings.Join(codexReviewArguments(codexConfig(false, 0), filepath.Join(os.TempDir(), "out"), "test-model"), " ")
	for _, want := range []string{`sandbox_mode="read-only"`, "project_doc_max_bytes=0", "exec review"} {
		if !strings.Contains(arguments, want) {
			t.Errorf("arguments %q missing %q", arguments, want)
		}
	}
}
