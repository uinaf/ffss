package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDirectTelemetryRecording drives telemetry in-process: verify measures
// its own duration, other transitions accept caller-supplied telemetry, and
// invalid shapes fail closed without blocking telemetry-free transitions.
func TestDirectTelemetryRecording(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })
	t.Setenv("SLOPSHIPPER_DB", filepath.Join(t.TempDir(), "telemetry.sqlite"))

	if code := run([]string{"init", "--run", "tm"}); code != 0 {
		t.Fatal("init")
	}
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"required_reviewers":["slopguard"],"series_bound":1,"units":[{"id":"u1","title":"one"}]}`)
	if code := run([]string{"intake", "--file", intake, "--run", "tm"}); code != 0 {
		t.Fatal("intake")
	}
	if code := run([]string{"release", "--revision", "2", "--run", "tm"}); code != 0 {
		t.Fatal("release")
	}

	buildTel := filepath.Join(t.TempDir(), "tel.json")
	mustWrite(t, buildTel, `{"tokens":4000,"cost_cents":12,"route":{"venue":"local","harness":"claude-code","models":{"build":"claude-fable-5"}}}`)
	if code := run([]string{"build", "--run", "tm", "--telemetry", buildTel}); code != 0 {
		t.Fatal("build with telemetry")
	}

	// Fail-closed shapes: negative values, all-zero objects, junk route.
	for name, body := range map[string]string{
		"negative": `{"tokens":-1}`,
		"allzero":  `{}`,
		"badroute": `{"route":{"venue":"not a venue"}}`,
	} {
		bad := filepath.Join(t.TempDir(), name+".json")
		mustWrite(t, bad, body)
		if code := run([]string{"verify", "--cmd", "true", "--run", "tm", "--telemetry", bad}); code == 0 {
			t.Fatalf("%s telemetry must fail closed", name)
		}
	}

	// verify measures wall clock itself; the caller supplies the rest.
	verifyTel := filepath.Join(t.TempDir(), "vtel.json")
	mustWrite(t, verifyTel, `{"tokens":100}`)
	if code := run([]string{"verify", "--cmd", "true", "--run", "tm", "--telemetry", verifyTel}); code != 0 {
		t.Fatal("verify")
	}
	if code := run([]string{"--dry-run", "build", "--run", "tm"}); code == 0 {
		t.Log("dry-run build allowed (state-dependent); fine")
	}
}

func TestTelemetryTotalsSurfaceInStatus(t *testing.T) {
	h := newCLIHarness(t)
	h.must("init", "--run", "tt")
	intake := filepath.Join(t.TempDir(), "intake.json")
	mustWrite(t, intake, `{"required_reviewers":["slopguard"],"series_bound":1,"units":[{"id":"u1","title":"one"}]}`)
	h.must("intake", "--file", intake, "--run", "tt")
	h.must("release", "--revision", "2", "--run", "tt")
	tel := filepath.Join(t.TempDir(), "tel.json")
	mustWrite(t, tel, `{"tokens":4000,"cost_cents":12}`)
	h.must("build", "--run", "tt", "--telemetry", tel)
	h.must("verify", "--cmd", "true", "--run", "tt")

	out := h.must("status", "--json", "--run", "tt")
	var st struct {
		TotalDurationMS int64 `json:"total_duration_ms"`
		TotalTokens     int   `json:"total_tokens"`
		TotalCostCents  int   `json:"total_cost_cents"`
		TelemetryEvents int   `json:"telemetry_events"`
	}
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatal(err)
	}
	if st.TotalTokens != 4000 || st.TotalCostCents != 12 || st.TelemetryEvents != 2 || st.TotalDurationMS < 1 {
		t.Fatalf("totals must aggregate caller telemetry and measured verify duration: %s", out)
	}

	// Raw --input transitions accept a telemetry object.
	review := `{"run":"tt","reviewer":"slopguard","verdict":"clean","artifact_ref":"test://1","telemetry":{"tokens":500}}`
	h.mustInput(review, "review", "--input", "-")
	out = h.must("status", "--json", "--run", "tt")
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatal(err)
	}
	if st.TotalTokens != 4500 || st.TelemetryEvents != 3 {
		t.Fatalf("raw-input telemetry must aggregate: %s", out)
	}
}
