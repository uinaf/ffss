package reviewpolicy

import (
	"regexp"
	"strings"
)

var grokIncompleteExplanationPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b(?:starting|beginning) (?:an? )?(?:independent )?review\b`),
	regexp.MustCompile(`\b(?:i|we) (?:will|need to) (?:inspect|review|analy[sz]e|check)\b`),
	regexp.MustCompile(`\b(?:review|analysis|inspection) (?:is )?still (?:in progress|being (?:checked|performed|completed))\b`),
	regexp.MustCompile(`\b(?:initial|preliminary|interim) (?:pass|review|response)\b`),
	regexp.MustCompile(`\bnext (?:complete|completed|final) review\b`),
	regexp.MustCompile(`\bplaceholder (?:review|response|result)\b`),
}

var grokReviewProtocol = `
AUTOREVIEW-TRUSTED-GROK-REVIEW-PROTOCOL-V1
` + reviewPolicy + `
This is a single-shot, non-interactive review. Analyze the complete frozen target internally before producing structured output. The structured result must be the completed final review, never progress, a plan, a placeholder, or a statement of future work.
For a clean result, explain concrete inspected behavior that satisfies the trusted task contract. For each finding, identify the concrete failure mechanism and affected input at the cited changed lines. If the review is not complete, do not claim a clean or findings result.
Return the provider schema's review and completion objects. Set completion.status to complete only after assessing every target file. Include exactly one completion.files entry per target file, explain the concrete assessment for that file, and link each review finding by its zero-based index to the matching file. The completion object is internal protocol evidence and is not part of autoreview's public result schema.
Overall confidence represents confidence that the entire frozen target has been reviewed to completion, not confidence in each individual finding. Keep every concrete finding even when its own confidence is low. Do not claim the schema's minimum overall confidence until the review is complete.
`

func GrokReviewProtocol() string {
	return grokReviewProtocol
}

func GrokReviewIsIncomplete(overallExplanation string) bool {
	explanation := strings.ToLower(overallExplanation)
	for _, pattern := range grokIncompleteExplanationPatterns {
		if pattern.MatchString(explanation) {
			return true
		}
	}
	return false
}
