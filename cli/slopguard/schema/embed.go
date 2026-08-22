package schema

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed review-v1.schema.json
var reviewV1 []byte

//go:embed result-v1.schema.json
var resultV1 []byte

const GrokMinimumOverallExplanationCharacters = 160
const GrokMinimumOverallConfidence = 0.7

func ReviewV1() []byte {
	return append([]byte(nil), reviewV1...)
}

func ResultV1() []byte {
	return append([]byte(nil), resultV1...)
}

func CodexReviewV1() ([]byte, error) {
	return providerReviewV1("Codex", false, false, 0)
}

func ClaudeReviewV1() ([]byte, error) {
	return providerReviewV1("Claude", true, false, 0)
}

func GrokReviewV1(fileCount int) ([]byte, error) {
	if fileCount < 0 {
		return nil, fmt.Errorf("Grok review schema received a negative target file count")
	}
	document, err := providerReviewDocument(true, true, GrokMinimumOverallExplanationCharacters)
	if err != nil {
		return nil, err
	}
	properties, ok := document["properties"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("review schema is missing properties")
	}
	overallConfidence, ok := properties["overall_confidence"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("review schema is missing overall_confidence")
	}
	overallConfidence["minimum"] = GrokMinimumOverallConfidence
	return grokCompletionReviewV1(document, fileCount)
}

func providerReviewV1(provider string, omitDraftURI, omitNonWhitespacePattern bool, minimumOverallExplanation int) ([]byte, error) {
	document, err := providerReviewDocument(omitDraftURI, omitNonWhitespacePattern, minimumOverallExplanation)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode %s review schema: %w", provider, err)
	}
	return encoded, nil
}

func providerReviewDocument(omitDraftURI, omitNonWhitespacePattern bool, minimumOverallExplanation int) (map[string]any, error) {
	var document map[string]any
	if err := json.Unmarshal(reviewV1, &document); err != nil {
		return nil, fmt.Errorf("decode embedded review schema: %w", err)
	}
	definitions, ok := document["$defs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("review schema is missing $defs")
	}
	relativePath, ok := definitions["relative_path"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("review schema is missing relative_path")
	}
	delete(relativePath, "not")
	if omitDraftURI {
		delete(document, "$schema")
	}
	if omitNonWhitespacePattern {
		deletePattern(document, `\S`)
	}
	if minimumOverallExplanation > 0 {
		properties, ok := document["properties"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("review schema is missing properties")
		}
		overallExplanation, ok := properties["overall_explanation"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("review schema is missing overall_explanation")
		}
		overallExplanation["minLength"] = minimumOverallExplanation
	}
	return document, nil
}

func grokCompletionReviewV1(document map[string]any, fileCount int) ([]byte, error) {
	review := map[string]any{
		"type":                 document["type"],
		"additionalProperties": document["additionalProperties"],
		"required":             document["required"],
		"properties":           document["properties"],
	}
	document["title"] = "slopguard Grok completed review v1"
	document["required"] = []string{"review", "completion"}
	rangeCount := 0
	lastFileIndex := 0
	if fileCount > 0 {
		rangeCount = 1
		lastFileIndex = fileCount - 1
	}
	document["properties"] = map[string]any{
		"review": review,
		"completion": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"status", "files"},
			"properties": map[string]any{
				"status": map[string]any{"enum": []string{"complete"}},
				"files": map[string]any{
					"type":     "array",
					"minItems": rangeCount,
					"maxItems": rangeCount,
					"items": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []string{"start_index", "end_index"},
						"properties": map[string]any{
							"start_index": map[string]any{"type": "integer", "minimum": 0, "maximum": 0},
							"end_index":   map[string]any{"type": "integer", "minimum": lastFileIndex, "maximum": lastFileIndex},
						},
					},
				},
			},
		},
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode Grok completed review schema: %w", err)
	}
	return encoded, nil
}

func deletePattern(value any, pattern string) {
	switch value := value.(type) {
	case map[string]any:
		if value["pattern"] == pattern {
			delete(value, "pattern")
		}
		for _, child := range value {
			deletePattern(child, pattern)
		}
	case []any:
		for _, child := range value {
			deletePattern(child, pattern)
		}
	}
}
