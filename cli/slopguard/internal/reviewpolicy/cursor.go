package reviewpolicy

import contractschema "github.com/uinaf/ffss/cli/slopguard/schema"

var cursorReviewProtocol = `
SLOPGUARD-TRUSTED-REVIEW-PROTOCOL-V1
` + reviewPolicy + `
Your entire final assistant response must be exactly one JSON object matching the schema below. Do not include markdown fences, prose before or after the object, or any additional JSON values.
BEGIN SLOPGUARD-TRUSTED-REVIEW-SCHEMA-V1
` + string(contractschema.ReviewV1()) + `
END SLOPGUARD-TRUSTED-REVIEW-SCHEMA-V1
Return only the review JSON object now.
`

func CursorReviewProtocol() string {
	return cursorReviewProtocol
}
