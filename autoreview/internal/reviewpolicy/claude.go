package reviewpolicy

var claudeReviewProtocol = `
AUTOREVIEW-TRUSTED-REVIEW-POLICY-V1
` + reviewPolicy + `
`

func ClaudeReviewProtocol() string {
	return claudeReviewProtocol
}
