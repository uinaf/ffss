package reviewpolicy

import "testing"

func TestGrokReviewIsIncomplete(t *testing.T) {
	t.Parallel()

	for _, explanation := range []string{
		"Starting an independent review of the complete target. I will inspect every changed file.",
		"Initial pass only records the frozen file set before the next complete review.",
		"The analysis is still being checked before a final answer.",
		"This placeholder result will be replaced after review.",
	} {
		if !GrokReviewIsIncomplete(explanation) {
			t.Errorf("did not reject explicit progress explanation %q", explanation)
		}
	}

	for _, explanation := range []string{
		"The complete target was inspected and no actionable defect was found in the changed control flow.",
		"The new branch will return the cached value after validation, which preserves the documented contract.",
		"The finding identifies a TODO placeholder in the changed UI because it is rendered to production users.",
	} {
		if GrokReviewIsIncomplete(explanation) {
			t.Errorf("rejected completed explanation %q", explanation)
		}
	}
}
