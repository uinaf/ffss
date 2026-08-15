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
	Type                 string                `json:"type,omitempty"`
	Description          string                `json:"description,omitempty"`
	Properties           map[string]jsonSchema `json:"properties,omitempty"`
	Required             []string              `json:"required,omitempty"`
	Enum                 []string              `json:"enum,omitempty"`
	Items                *jsonSchema           `json:"items,omitempty"`
	Minimum              *int                  `json:"minimum,omitempty"`
	AdditionalProperties *bool                 `json:"additionalProperties,omitempty"`
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
	reviewer := stringSchema("Registered reviewer identity. Built-ins: autoreview, bugbot; register anything else (including a human sign-off identity) with slopshipper reviewers --add.")
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
		mutationSchema("init", "Create a run for the current repository.", objectSchema(map[string]jsonSchema{"run": run}), flags("run", "input")),
		mutationSchema("intake", "Load or update the released-work contract.", intake, flags("file", "run", "input")),
		mutationSchema("release", "Latch human approval for an intake revision.", objectSchema(map[string]jsonSchema{
			"run": run, "revision": integerSchema("Exact intake revision to release."),
		}, "revision"), flags("revision", "run", "input")),
		mutationSchema("build", "Claim the next ready unit.", objectSchema(map[string]jsonSchema{"run": run}), flags("run", "input")),
		mutationSchema("verify", "Record verification evidence; convenience --cmd executes a shell command.", objectSchema(map[string]jsonSchema{
			"run": run, "command": stringSchema("Verification command represented by the evidence."),
			"exit_code": integerSchema("Verification exit code."), "output_digest": stringSchema("Optional output digest."),
		}, "command", "exit_code"), flags("cmd", "evidence", "run", "input")),
		mutationSchema("review", "Record strict independent-review evidence.", objectSchema(map[string]jsonSchema{
			"run": run, "reviewer": reviewer,
			"verdict":      enumSchema("Review outcome.", "clean", "findings", "ambiguous"),
			"artifact_ref": stringSchema("Stable review artifact reference."),
		}, "reviewer", "verdict", "artifact_ref"), flags("evidence", "run", "input")),
		mutationSchema("rework", "Return review work to the build loop.", objectSchema(map[string]jsonSchema{"run": run}), flags("run", "input")),
		mutationSchema("deliver", "Record delivery evidence and complete the current unit.", objectSchema(map[string]jsonSchema{
			"run": run, "delivery_mode": enumSchema("Must match intake when present.", "pr-hold", "pr-merge-when-ready", "direct-trunk"),
			"pr_url": stringSchema("Required for PR delivery modes."), "commit_sha": stringSchema("Required for direct-trunk delivery."),
		}), flags("evidence", "run", "input")),
		mutationSchema("ask", "Park the run for a human decision.", objectSchema(map[string]jsonSchema{
			"run": run, "question": stringSchema("Question requiring a human answer."),
		}, "question"), flags("question", "run", "input")),
		mutationSchema("decide", "Record a human answer and resume.", objectSchema(map[string]jsonSchema{
			"run": run, "answer": stringSchema("Human decision."),
		}, "answer"), flags("answer", "run", "input")),
		mutationSchema("retry", "Record recovery and resume blocked verification.", objectSchema(map[string]jsonSchema{
			"run": run, "reason": stringSchema("Confirmed recovery reason."),
		}, "reason"), flags("reason", "run", "input")),
		mutationSchema("block", "Record why active work cannot continue.", objectSchema(map[string]jsonSchema{
			"run": run, "reason": stringSchema("External blocker reason."),
		}, "reason"), flags("reason", "run", "input")),
		{Name: "status", Description: "Return compact state and the next allowed action.", Flags: flags("json", "run", "fields"), Output: "status"},
		{Name: "reviewers", Description: "List the reviewer registry, or register/unregister a custom identity.", Mutating: true, Flags: flags("add", "remove", "json"), Output: "reviewers"},
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
		case "add":
			description = "Register a custom reviewer identity; idempotent."
		case "remove":
			description = "Unregister a custom reviewer identity; idempotent."
		}
		result = append(result, flagSchema{Name: "--" + name, Type: typeName, Description: description})
	}
	return result
}

func objectSchema(properties map[string]jsonSchema, required ...string) jsonSchema {
	additional := false
	return jsonSchema{Type: "object", Properties: properties, Required: required, AdditionalProperties: &additional}
}

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

func boolSchema(description string) jsonSchema {
	return jsonSchema{Type: "boolean", Description: description}
}

func arraySchema(items jsonSchema, description string) jsonSchema {
	return jsonSchema{Type: "array", Description: description, Items: &items}
}
