package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitAcceptsTelemetryAndDryRunTotalsProject(t *testing.T) {
	h := newCLIHarness(t)
	tel := filepath.Join(t.TempDir(), "tel.json")
	mustWrite(t, tel, `{"tokens":1000}`)
	h.must("init", "--run", "it", "--telemetry", tel)
	out := h.must("status", "--json", "--run", "it")
	var st struct {
		TotalTokens     int `json:"total_tokens"`
		TelemetryEvents int `json:"telemetry_events"`
	}
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatal(err)
	}
	if st.TotalTokens != 1000 || st.TelemetryEvents != 1 {
		t.Fatalf("init telemetry must ride the init event: %s", out)
	}

	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"required_reviewers":["slopguard"],"series_bound":1,"units":[{"id":"u1","title":"one"}]}`)
	h.must("intake", "--file", intake, "--run", "it")

	// Dry-run totals include the proposed telemetry without persisting it.
	tel2 := filepath.Join(t.TempDir(), "tel2.json")
	mustWrite(t, tel2, `{"tokens":500}`)
	out = h.must("--json", "--dry-run", "release", "--revision", "2", "--run", "it", "--telemetry", tel2)
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatal(err)
	}
	if st.TotalTokens != 1500 || st.TelemetryEvents != 2 {
		t.Fatalf("dry-run totals must project the proposed telemetry: %s", out)
	}
	out = h.must("status", "--json", "--run", "it")
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatal(err)
	}
	if st.TotalTokens != 1000 || st.TelemetryEvents != 1 {
		t.Fatalf("dry run must not persist telemetry: %s", out)
	}
}

func TestTelemetryStdinCannotBeShared(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "sh")
	out, code := h.runInput(`{"series_bound":1}`, "intake", "--file", "-", "--telemetry", "-", "--run", "sh")
	if code != 2 || !strings.Contains(out, "one JSON source may read stdin") {
		t.Fatalf("two stdin sources must be rejected: exit %d %s", code, out)
	}
}

func TestTelemetrySchemaEncodesNonZeroRule(t *testing.T) {
	h := newCLIHarness(t)
	out := h.must("schema", "--command", "build", "--json")
	var doc struct {
		Commands []struct {
			Input struct {
				Properties map[string]struct {
					AnyOf []struct {
						Required []string `json:"required"`
					} `json:"anyOf"`
				} `json:"properties"`
			} `json:"input_schema"`
		} `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	telemetry := doc.Commands[0].Input.Properties["telemetry"]
	if len(telemetry.AnyOf) != 4 {
		t.Fatalf("published schema must encode the non-zero telemetry rule: %s", out)
	}
}

func TestVerifyRejectsBadTelemetryBeforeExecution(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "vt")
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"required_reviewers":["slopguard"],"series_bound":1,"units":[{"id":"u1","title":"one"}]}`)
	h.must("intake", "--file", intake, "--run", "vt")
	h.must("release", "--revision", "2", "--run", "vt")
	h.must("build", "--run", "vt")

	sentinel := filepath.Join(t.TempDir(), "executed")
	bad := filepath.Join(t.TempDir(), "bad.json")
	mustWrite(t, bad, `{"tokens":-5}`)
	out, code := h.run("verify", "--cmd", "touch "+sentinel, "--run", "vt", "--telemetry", bad)
	if code != 2 || !strings.Contains(out, "between 0 and") {
		t.Fatalf("bad telemetry must fail before execution: exit %d\n%s", code, out)
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Fatal("verify command must not run when telemetry is invalid")
	}
	// Dry-run with bad telemetry also fails instead of reporting success.
	out, code = h.run("--dry-run", "verify", "--cmd", "true", "--run", "vt", "--telemetry", bad)
	if code != 2 {
		t.Fatalf("dry-run must validate telemetry: exit %d\n%s", code, out)
	}
}
