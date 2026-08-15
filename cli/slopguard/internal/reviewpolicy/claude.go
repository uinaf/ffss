package reviewpolicy

var claudeReviewProtocol = `
SLOPGUARD-TRUSTED-REVIEW-POLICY-V1
` + reviewPolicy + `
`

func ClaudeReviewProtocol() string {
	return claudeReviewProtocol
}
