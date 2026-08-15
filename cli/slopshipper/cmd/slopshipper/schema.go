package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/uinaf/slopshipper/internal/status"
)

type introspectionDocument struct {
	SchemaVersion           int             `json:"schema_version"`
	CLI                     string          `json:"cli"`
	GlobalFlags             []flagSchema    `json:"global_flags"`
	Commands                []commandSchema `json:"commands"`
	StatusFields            []string        `json:"status_fields"`
	RecommendedStatusFields []string        `json:"recommended_status_fields"`
	ErrorSchema             jsonSchema      `json:"error_schema"`
}

type commandSchema struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Mutating    bool         `json:"mutating"`
	Flags       []flagSchema `json:"flags"`
	Input       *jsonSchema  `json:"input_schema,omitempty"`
	Output      string       `json:"output"`
}

type flagSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type jsonSchema struct {
	Type          string                `json:"type,omitempty"`
	Description   string                `json:"description,omitempty"`
	Properties    map[string]jsonSchema `json:"properties,omitempty"`
	Required      []string              `json:"required,omitempty"`
	Enum          []string              `json:"enum,omitempty"`
	Items         *jsonSchema           `json:"items,omitempty"`
	Minimum       *int                  `json:"minimum,omitempty"`
	Maximum       *int                  `json:"maximum,omitempty"`
	MinLength     *int                  `json:"minLength,omitempty"`
	MinProperties *int                  `json:"minProperties,omitempty"`
	AnyOf         []jsonSchema          `json:"anyOf,omitempty"`
	// AdditionalProperties is false for closed objects or a schema for
	// typed map values.
	AdditionalProperties any `json:"additionalProperties,omitempty"`
}

func schemaDocument(command string) (introspectionDocument, error) {
	commands := allCommandSchemas()
	if command != "" {
		found := false
		for _, spec := range commands {
			if spec.Name == command {
				commands = []commandSchema{spec}
				found = true
				break
			}
		}
		if !found {
			return introspectionDocument{}, fmt.Errorf("unknown command %q", command)
		}
	}
	return introspectionDocument{
		SchemaVersion: 1,
		CLI:           "slopshipper",
		GlobalFlags: []flagSchema{
			{Name: "--json", Type: "boolean", Description: "Emit structured JSON success and error output."},
			{Name: "--dry-run", Type: "boolean", Description: "Validate a mutating command without shell execution or persistence."},
		},
		Commands:                commands,
		StatusFields:            status.FieldNames(),
		RecommendedStatusFields: strings.Split(status.AgentFieldMask, ","),
		ErrorSchema: objectSchema(map[string]jsonSchema{
			"schema_version": integerSchema("Error response schema version."),
			"ok":             boolSchema("Always false."),
			"error": objectSchema(map[string]jsonSchema{
				"kind":      stringSchema("Stable error classification."),
				"message":   stringSchema("Actionable error message."),
				"exit_code": integerSchema("Process exit code."),
			}, "kind", "message", "exit_code"),
		}, "schema_version", "ok", "error"),
	}, nil
}

func allCommandSchemas() []commandSchema {
	run := stringSchema("Optional run identifier. ASCII letters, digits, dot, underscore, and hyphen; must start alphanumeric.")
	unit := objectSchema(map[string]jsonSchema{
		"id":                  stringSchema("Hardened unit identifier."),
		"title":               stringSchema("Human-readable unit title."),
		"blockers":            arraySchema(stringSchema("ID of a prerequisite unit."), "Dependency-ordered blocker IDs."),
		"acceptance_criteria": arraySchema(stringSchema("One verifiable acceptance criterion."), "What must be true for the unit to count as done; at most 32 single-line criteria, each at most 500 bytes."),
		"complexity":          enumSchema("Expected difficulty of the unit.", "low", "medium", "high"),
	}, "id")
	venue := stringSchema("Where the work ran (for example local, crabbox, a cloud agent).")
	venue.MinLength = intPointer(1)
	harness := stringSchema("Driver harness identity.")
	harness.MinLength = intPointer(1)
	models := mapSchema(stringSchema("Model identity for the role."), "Role to model map.")
	route := objectSchema(map[string]jsonSchema{
		"venue":   venue,
		"harness": harness,
		"models":  models,
	})
	route.Description = "Execution stack actually used; at least one non-empty field when present."
	route.MinProperties = intPointer(1)
	route.AnyOf = []jsonSchema{
		{Required: []string{"venue"}},
		{Required: []string{"harness"}},
		{Required: []string{"models"}, Properties: map[string]jsonSchema{"models": {MinProperties: intPointer(1)}}},
	}
	telemetry := objectSchema(map[string]jsonSchema{
		"duration_ms": boundedIntegerSchema("Wall-clock duration in milliseconds.", 0, maxTelemetryDimension),
		"tokens":      boundedIntegerSchema("Estimated tokens spent.", 0, maxTelemetryDimension),
		"cost_cents":  boundedIntegerSchema("Estimated cost in cents.", 0, maxTelemetryDimension),
		"route":       route,
	})
	telemetry.Description = "Optional recorded telemetry for this transition; at least one non-zero dimension or a route when present."
	telemetry.MinProperties = intPointer(1)
	// All-zero telemetry is rejected at the boundary; the published contract
	// encodes the same rule so schema-driven agents never emit it.
	telemetry.AnyOf = []jsonSchema{
		{Required: []string{"duration_ms"}, Properties: map[string]jsonSchema{"duration_ms": minimumIntegerSchema("", 1)}},
		{Required: []string{"tokens"}, Properties: map[string]jsonSchema{"tokens": minimumIntegerSchema("", 1)}},
		{Required: []string{"cost_cents"}, Properties: map[string]jsonSchema{"cost_cents": minimumIntegerSchema("", 1)}},
		{Required: []string{"route"}},
	}
	reviewer := stringSchema("Registered reviewer identity. Built-ins: slopguard, bugbot; register anything else (including a human sign-off identity) with slopshipper reviewers --add.")
	intake := objectSchema(map[string]jsonSchema{
		"run":                run,
		"delivery_mode":      enumSchema("Delivery behavior.", "pr-hold", "pr-merge-when-ready", "direct-trunk"),
		"required_reviewers": arraySchema(reviewer, "Replacement set of required registered reviewers; at least one."),
		"risk_tier":          enumSchema("How much can go wrong when this work is wrong; consumed by routing and review-depth policy.", "low", "medium", "high"),
		"budget":             objectSchema(map[string]jsonSchema{"tokens": minimumIntegerSchema("Token ceiling for the run.", 1), "minutes": minimumIntegerSchema("Wall-clock ceiling in minutes.", 1)}),
		"series_bound":       minimumIntegerSchema("Maximum units this run may complete.", 1),
		"units":              arraySchema(unit, "Replacement dependency graph."),
	})
	commands := []commandSchema{
		withTelemetry(mutationSchema("init", "Create a run for the current repository.", objectSchema(map[string]jsonSchema{"run": run}), flags("run", "input")), telemetry),
		withTelemetry(mutationSchema("intake", "Load or update the released-work contract.", intake, flags("file", "run", "input")), telemetry),
		withTelemetry(mutationSchema("release", "Latch human approval for an intake revision.", objectSchema(map[string]jsonSchema{
			"run": run, "revision": integerSchema("Exact intake revision to release."),
		}, "revision"), flags("revision", "run", "input")), telemetry),
		withTelemetry(mutationSchema("build", "Claim the next ready unit.", objectSchema(map[string]jsonSchema{"run": run}), flags("run", "input")), telemetry),
		withTelemetry(mutationSchema("verify", "Record verification evidence; convenience --cmd executes a shell command.", objectSchema(map[string]jsonSchema{
			"run": run, "command": stringSchema("Verification command represented by the evidence."),
			"exit_code": integerSchema("Verification exit code."), "output_digest": stringSchema("Optional output digest."),
		}, "command", "exit_code"), flags("cmd", "evidence", "run", "input")), telemetry),
		withTelemetry(mutationSchema("review", "Record strict independent-review evidence; forge-corroborated reviewers are checked against the live change request.", objectSchema(map[string]jsonSchema{
			"run": run, "reviewer": reviewer,
			"verdict":           enumSchema("Review outcome.", "clean", "findings", "ambiguous"),
			"artifact_ref":      stringSchema("Stable review artifact reference; must be a change-request URL for forge-corroborated reviewers."),
			"unverified":        boolSchema("Bypass forge corroboration explicitly; requires unverified_reason."),
			"unverified_reason": stringSchema("Single-line reason recorded with an unverified override."),
		}, "reviewer", "verdict", "artifact_ref"), flags("evidence", "unverified", "reason", "run", "input")), telemetry),
		withTelemetry(mutationSchema("rework", "Return review work to the build loop.", objectSchema(map[string]jsonSchema{"run": run}), flags("run", "input")), telemetry),
		withTelemetry(mutationSchema("deliver", "Record delivery evidence and complete the current unit; forge-bound repos verify the change request exists and matches the delivered head.", objectSchema(map[string]jsonSchema{
			"run": run, "delivery_mode": enumSchema("Must match intake when present.", "pr-hold", "pr-merge-when-ready", "direct-trunk"),
			"pr_url": stringSchema("Required for PR delivery modes."), "commit_sha": stringSchema("Required for direct-trunk delivery; optional for PR modes (verified deliveries adopt the observed head when omitted)."),
			"unverified":        boolSchema("Bypass forge verification explicitly; requires unverified_reason."),
			"unverified_reason": stringSchema("Single-line reason recorded with an unverified override."),
		}), flags("evidence", "unverified", "reason", "run", "input")), telemetry),
		withTelemetry(mutationSchema("observe", "Record an external signal for a delivered unit.", objectSchema(map[string]jsonSchema{
			"run": run, "unit": stringSchema("Delivered unit; optional when exactly one unit is delivered."),
			"signal":    enumSchema("What the forge showed.", "merged", "checks_failed", "review_feedback", "head_moved"),
			"reference": stringSchema("Optional link or check name backing the signal."),
		}, "signal"), flags("signal", "unit", "reference", "run", "input")), telemetry),
		withTelemetry(mutationSchema("ask", "Park the run for a human decision.", objectSchema(map[string]jsonSchema{
			"run": run, "question": stringSchema("Question requiring a human answer."),
		}, "question"), flags("question", "run", "input")), telemetry),
		withTelemetry(mutationSchema("decide", "Record a human answer and resume.", objectSchema(map[string]jsonSchema{
			"run": run, "answer": stringSchema("Human decision."),
		}, "answer"), flags("answer", "run", "input")), telemetry),
		withTelemetry(mutationSchema("retry", "Record recovery and resume blocked verification.", objectSchema(map[string]jsonSchema{
			"run": run, "reason": stringSchema("Confirmed recovery reason."),
		}, "reason"), flags("reason", "run", "input")), telemetry),
		withTelemetry(mutationSchema("block", "Record why active work cannot continue.", objectSchema(map[string]jsonSchema{
			"run": run, "reason": stringSchema("External blocker reason."),
		}, "reason"), flags("reason", "run", "input")), telemetry),
		{Name: "status", Description: "Return compact state and the next allowed action.", Flags: flags("json", "run", "fields"), Output: "status"},
		{Name: "watch", Description: "Observe delivered units on the forge and record signals as observe events; --once runs one pass, --interval polls bounded.", Mutating: true, Flags: flags("once", "interval", "iterations", "run", "json"), Output: "watch"},
		{Name: "reviewers", Description: "List the reviewer registry, or register/unregister a custom identity.", Mutating: true, Flags: flags("add", "remove", "json"), Output: "reviewers"},
		{Name: "repo", Description: "Show or declare the repo profile: role bindings (review, qa, venue, memory) and policy (forge kind, trust tier, verify command, delivery mode, readiness). Subcommands: show, register, update, unregister.", Mutating: true, Flags: flags("forge", "trust", "verify-cmd", "delivery", "readiness", "bind", "forge-reviewer", "json"), Output: "repo"},
		{Name: "schema", Description: "Describe commands, flags, raw inputs, enums, and outputs as JSON.", Flags: flags("json", "command"), Output: "schema"},
		{Name: "storage", Description: "Inspect database path resolution and Git safety without mutation.", Flags: flags("json"), Output: "storage"},
		{Name: "serve", Description: "Serve the read-only projector on loopback.", Flags: flags("addr"), Output: "long-running"},
		{Name: "version", Description: "Print build version and source revision.", Flags: flags("json"), Output: "version"},
	}
	sort.Slice(commands, func(i, j int) bool { return commands[i].Name < commands[j].Name })
	return commands
}

func mutationSchema(name, description string, input jsonSchema, commandFlags []flagSchema) commandSchema {
	return commandSchema{Name: name, Description: description, Mutating: true, Flags: commandFlags, Input: &input, Output: "status"}
}

// withTelemetry adds the shared optional telemetry object to a transition's
// input schema and its --telemetry flag.
func withTelemetry(spec commandSchema, telemetry jsonSchema) commandSchema {
	if spec.Input != nil {
		spec.Input.Properties["telemetry"] = telemetry
	}
	spec.Flags = append(spec.Flags, flagSchema{Name: "--telemetry", Type: "string", Description: "Strict JSON telemetry path; use - for stdin."})
	return spec
}

func flags(names ...string) []flagSchema {
	result := make([]flagSchema, 0, len(names))
	for _, name := range names {
		typeName := "string"
		description := "Command option."
		switch name {
		case "json":
			typeName, description = "boolean", "Emit JSON. May also be placed before the command."
		case "input":
			description = "Strict raw JSON command payload path; use - for stdin. Cannot be mixed with convenience flags."
		case "file", "evidence":
			description = "Strict JSON path; use - for stdin."
		case "fields":
			description = "Comma-separated status field mask; requires --json."
		case "revision":
			typeName, description = "integer", "Exact intake revision."
		case "run":
			description = "Hardened run identifier."
		case "command":
			description = "Limit schema output to one command."
		case "signal":
			description = "Observed signal: merged, checks_failed, review_feedback, or head_moved."
		case "unit":
			description = "Delivered unit identifier; optional when unambiguous."
		case "reference":
			description = "Optional link or check name backing the signal."
		case "add":
			description = "Register a custom reviewer identity; idempotent."
		case "remove":
			description = "Unregister a custom reviewer identity; idempotent."
		case "once":
			typeName, description = "boolean", "Run exactly one observation pass (the default)."
		case "interval":
			typeName, description = "integer", "Poll every N seconds (5-3600) until bounds are reached."
		case "iterations":
			typeName, description = "integer", "Maximum polling passes for --interval (default 20, max 1000)."
		case "forge":
			description = "Forge kind hosting this repo's change requests: github."
		case "trust":
			description = "Earned autonomy tier: low, medium, or high."
		case "verify-cmd":
			description = "Canonical single-line verification command for this repo."
		case "delivery":
			description = "Default delivery mode for new runs: pr-hold, pr-merge-when-ready, or direct-trunk."
		case "readiness":
			description = "Recorded agent-readiness verdict: ready or not_ready."
		case "bind":
			description = "Replace role bindings as comma-separated role=name pairs; roles: review, qa, venue, memory."
		case "forge-reviewer":
			description = "Replace forge-resident reviewer mappings as comma-separated identity=login pairs; their review evidence is corroborated against the forge."
		case "unverified":
			typeName, description = "boolean", "Bypass forge verification of this evidence explicitly; requires --reason."
		case "reason":
			description = "Reason text recorded with the action."
		}
		result = append(result, flagSchema{Name: "--" + name, Type: typeName, Description: description})
	}
	return result
}

func objectSchema(properties map[string]jsonSchema, required ...string) jsonSchema {
	return jsonSchema{Type: "object", Properties: properties, Required: required, AdditionalProperties: false}
}

// mapSchema is an open-keyed object whose values share one schema.
func mapSchema(values jsonSchema, description string) jsonSchema {
	return jsonSchema{Type: "object", Description: description, AdditionalProperties: &values}
}

func intPointer(value int) *int { return &value }

func stringSchema(description string) jsonSchema {
	return jsonSchema{Type: "string", Description: description}
}

func enumSchema(description string, values ...string) jsonSchema {
	return jsonSchema{Type: "string", Description: description, Enum: values}
}

func integerSchema(description string) jsonSchema {
	return jsonSchema{Type: "integer", Description: description}
}

func minimumIntegerSchema(description string, minimum int) jsonSchema {
	return jsonSchema{Type: "integer", Description: description, Minimum: &minimum}
}

// maxTelemetryDimension mirrors the machine's per-dimension telemetry bound.
const maxTelemetryDimension = 1 << 50

func boundedIntegerSchema(description string, minimum, maximum int) jsonSchema {
	return jsonSchema{Type: "integer", Description: description, Minimum: &minimum, Maximum: &maximum}
}

func boolSchema(description string) jsonSchema {
	return jsonSchema{Type: "boolean", Description: description}
}

func arraySchema(items jsonSchema, description string) jsonSchema {
	return jsonSchema{Type: "array", Description: description, Items: &items}
}
