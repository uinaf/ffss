package main

import (
	"fmt"

	"github.com/uinaf/ffsstack/cli/slopmachine/internal/machine"
)

type initInput struct {
	Run       string        `json:"run,omitempty"`
	Telemetry *telemetryDTO `json:"telemetry,omitempty"`
}

type intakeInput struct {
	Run               string          `json:"run,omitempty"`
	DeliveryMode      *string         `json:"delivery_mode,omitempty"`
	RequiredReviewers []string        `json:"required_reviewers,omitempty"`
	RiskTier          *string         `json:"risk_tier,omitempty"`
	Budget            *budgetDTO      `json:"budget,omitempty"`
	SeriesBound       *int            `json:"series_bound,omitempty"`
	Units             []intakeUnitDTO `json:"units,omitempty"`
	Telemetry         *telemetryDTO   `json:"telemetry,omitempty"`
}

type releaseInput struct {
	Run       string        `json:"run,omitempty"`
	Revision  *int64        `json:"revision"`
	Telemetry *telemetryDTO `json:"telemetry,omitempty"`
}

type runInput struct {
	Run       string        `json:"run,omitempty"`
	Telemetry *telemetryDTO `json:"telemetry,omitempty"`
}

type verifyInput struct {
	Run          string        `json:"run,omitempty"`
	Command      string        `json:"command"`
	ExitCode     *int          `json:"exit_code"`
	OutputDigest string        `json:"output_digest,omitempty"`
	Telemetry    *telemetryDTO `json:"telemetry,omitempty"`
}

type reviewInput struct {
	Run         string `json:"run,omitempty"`
	Reviewer    string `json:"reviewer"`
	Verdict     string `json:"verdict"`
	ArtifactRef string `json:"artifact_ref"`
	// Unverified requests an explicit verification bypass. Both fields keep
	// presence: an explicit false with a reason and an orphan reason field
	// (even empty) are rejected rather than ignored.
	Unverified       *bool         `json:"unverified,omitempty"`
	UnverifiedReason *string       `json:"unverified_reason,omitempty"`
	Telemetry        *telemetryDTO `json:"telemetry,omitempty"`
}

type deliverInput struct {
	Run              string        `json:"run,omitempty"`
	DeliveryMode     string        `json:"delivery_mode,omitempty"`
	PRURL            string        `json:"pr_url,omitempty"`
	CommitSHA        string        `json:"commit_sha,omitempty"`
	Unverified       *bool         `json:"unverified,omitempty"`
	UnverifiedReason *string       `json:"unverified_reason,omitempty"`
	Telemetry        *telemetryDTO `json:"telemetry,omitempty"`
}

type observeInput struct {
	Run       string        `json:"run,omitempty"`
	Unit      string        `json:"unit,omitempty"`
	Signal    string        `json:"signal"`
	Reference string        `json:"reference,omitempty"`
	Telemetry *telemetryDTO `json:"telemetry,omitempty"`
}

type questionInput struct {
	Run       string        `json:"run,omitempty"`
	Question  string        `json:"question"`
	Telemetry *telemetryDTO `json:"telemetry,omitempty"`
}

type answerInput struct {
	Run       string        `json:"run,omitempty"`
	Answer    string        `json:"answer"`
	Telemetry *telemetryDTO `json:"telemetry,omitempty"`
}

type reasonInput struct {
	Run       string        `json:"run,omitempty"`
	Reason    string        `json:"reason"`
	Telemetry *telemetryDTO `json:"telemetry,omitempty"`
}

// telemetryDTO preserves field presence so explicitly empty values are
// rejected instead of silently reading as absent.
type telemetryDTO struct {
	DurationMS *int64    `json:"duration_ms,omitempty"`
	Tokens     *int      `json:"tokens,omitempty"`
	CostCents  *int      `json:"cost_cents,omitempty"`
	Route      *routeDTO `json:"route,omitempty"`
}

type routeDTO struct {
	Venue   *string           `json:"venue,omitempty"`
	Harness *string           `json:"harness,omitempty"`
	Models  map[string]string `json:"models,omitempty"`
}

func (t *telemetryDTO) toTelemetry() (*machine.Telemetry, error) {
	if t == nil {
		return nil, nil
	}
	out := &machine.Telemetry{}
	if t.DurationMS != nil {
		out.DurationMS = *t.DurationMS
	}
	if t.Tokens != nil {
		out.Tokens = *t.Tokens
	}
	if t.CostCents != nil {
		out.CostCents = *t.CostCents
	}
	if t.Route != nil {
		route := &machine.Route{Models: t.Route.Models}
		if t.Route.Venue != nil {
			if *t.Route.Venue == "" {
				return nil, fmt.Errorf("telemetry.route.venue must be non-empty when set; omit the field instead")
			}
			route.Venue = *t.Route.Venue
		}
		if t.Route.Harness != nil {
			if *t.Route.Harness == "" {
				return nil, fmt.Errorf("telemetry.route.harness must be non-empty when set; omit the field instead")
			}
			route.Harness = *t.Route.Harness
		}
		out.Route = route
	}
	return out, nil
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
