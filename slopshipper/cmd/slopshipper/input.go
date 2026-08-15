package main

import (
	"fmt"
)

type initInput struct {
	Run string `json:"run,omitempty"`
}

type intakeInput struct {
	Run               string          `json:"run,omitempty"`
	DeliveryMode      *string         `json:"delivery_mode,omitempty"`
	RequiredReviewers []string        `json:"required_reviewers,omitempty"`
	RiskTier          *string         `json:"risk_tier,omitempty"`
	Budget            *budgetDTO      `json:"budget,omitempty"`
	SeriesBound       *int            `json:"series_bound,omitempty"`
	Units             []intakeUnitDTO `json:"units,omitempty"`
}

type releaseInput struct {
	Run      string `json:"run,omitempty"`
	Revision *int64 `json:"revision"`
}

type runInput struct {
	Run string `json:"run,omitempty"`
}

type verifyInput struct {
	Run          string `json:"run,omitempty"`
	Command      string `json:"command"`
	ExitCode     *int   `json:"exit_code"`
	OutputDigest string `json:"output_digest,omitempty"`
}

type reviewInput struct {
	Run         string `json:"run,omitempty"`
	Reviewer    string `json:"reviewer"`
	Verdict     string `json:"verdict"`
	ArtifactRef string `json:"artifact_ref"`
}

type deliverInput struct {
	Run          string `json:"run,omitempty"`
	DeliveryMode string `json:"delivery_mode,omitempty"`
	PRURL        string `json:"pr_url,omitempty"`
	CommitSHA    string `json:"commit_sha,omitempty"`
}

type questionInput struct {
	Run      string `json:"run,omitempty"`
	Question string `json:"question"`
}

type answerInput struct {
	Run    string `json:"run,omitempty"`
	Answer string `json:"answer"`
}

type reasonInput struct {
	Run    string `json:"run,omitempty"`
	Reason string `json:"reason"`
}

func decodeMutationInput(flags map[string]string, dest any) (bool, error) {
	path, present := flags["input"]
	if !present {
		return false, nil
	}
	if path == "" {
		return false, fmt.Errorf("--input requires a path or - for stdin")
	}
	for name := range flags {
		if name != "input" {
			return false, fmt.Errorf("--input cannot be combined with --%s", name)
		}
	}
	if err := readJSON(path, dest); err != nil {
		return false, fmt.Errorf("invalid command input: %w", err)
	}
	return true, nil
}
