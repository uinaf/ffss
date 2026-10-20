package reviewpolicy

import "strings"

var claudeReviewProtocol = `
SLOPGUARD-TRUSTED-REVIEW-POLICY-V1
` + reviewPolicy + `
`

func ClaudeReviewProtocol() string {
	return claudeReviewProtocol
}

// ClaudeRepositoryReviewProtocol lifts the no-tools rule for the read-only
// snapshot tools and appends the snapshot policy.
func ClaudeRepositoryReviewProtocol() string {
	return strings.Replace(claudeReviewProtocol, noToolsRule, "", 1) + RepositorySnapshotPolicy()
}
