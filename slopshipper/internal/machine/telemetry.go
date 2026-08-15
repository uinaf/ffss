package machine

import "fmt"

// Route records which execution stack actually ran a transition's work.
// Recorded input for the future router; the machine never selects routes.
type Route struct {
	Venue   string            `json:"venue,omitempty"`
	Harness string            `json:"harness,omitempty"`
	Models  map[string]string `json:"models,omitempty"` // role → model
}

// Telemetry carries optional per-transition cost signals as recorded input.
// Absent telemetry is always valid; the machine never blocks on it and
// never enforces spend.
type Telemetry struct {
	DurationMS int64  `json:"duration_ms,omitempty"`
	Tokens     int    `json:"tokens,omitempty"`
	CostCents  int    `json:"cost_cents,omitempty"`
	Route      *Route `json:"route,omitempty"`
}

// IsZero reports whether no telemetry dimension is recorded.
func (t Telemetry) IsZero() bool {
	return t.DurationMS == 0 && t.Tokens == 0 && t.CostCents == 0 && t.Route == nil
}

// maxTelemetryValue bounds every recorded dimension: large enough for any
// real run (over 31 years of milliseconds), small enough that summing a
// whole ledger can never overflow int64.
const maxTelemetryValue = int64(1) << 50

// ValidateTelemetry checks recorded telemetry fail-closed.
func ValidateTelemetry(t *Telemetry) error {
	if t == nil {
		return nil
	}
	if t.DurationMS < 0 || t.DurationMS > maxTelemetryValue {
		return fmt.Errorf("%w: telemetry.duration_ms must be between 0 and %d", ErrBadArgs, maxTelemetryValue)
	}
	if t.Tokens < 0 || int64(t.Tokens) > maxTelemetryValue {
		return fmt.Errorf("%w: telemetry.tokens must be between 0 and %d", ErrBadArgs, maxTelemetryValue)
	}
	if t.CostCents < 0 || int64(t.CostCents) > maxTelemetryValue {
		return fmt.Errorf("%w: telemetry.cost_cents must be between 0 and %d", ErrBadArgs, maxTelemetryValue)
	}
	if t.Route == nil {
		return nil
	}
	if t.Route.Venue == "" && t.Route.Harness == "" && len(t.Route.Models) == 0 {
		return fmt.Errorf("%w: telemetry.route must record a venue, harness, or model map; omit it instead", ErrBadArgs)
	}
	if t.Route.Venue != "" {
		if err := ValidateResourceID("route venue", t.Route.Venue); err != nil {
			return err
		}
	}
	if t.Route.Harness != "" {
		if err := ValidateResourceID("route harness", t.Route.Harness); err != nil {
			return err
		}
	}
	for role, model := range t.Route.Models {
		if err := ValidateResourceID("route model role", role); err != nil {
			return err
		}
		if err := ValidateResourceID("route model", model); err != nil {
			return err
		}
	}
	return nil
}
