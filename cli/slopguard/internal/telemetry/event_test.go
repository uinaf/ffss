package telemetry

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/phase"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

func TestEventUsesPrivacyAllowlist(t *testing.T) {
	const private = "PRIVATE-SENTINEL-path-revision-prompt-model-prose"
	recovery := protocol.RecoveryCursorTrailingObject
	report := protocol.Report{
		SchemaVersion: protocol.SchemaVersion,
		Status:        protocol.StatusFindings,
		Review: &protocol.Review{
			Findings: []protocol.Finding{{
				Title: private, Body: private, Priority: protocol.PriorityP1, Confidence: 0.9,
				Category: protocol.CategoryBug, Location: protocol.Location{FilePath: private, StartLine: 1, EndLine: 1},
			}},
			OverallExplanation: private,
			OverallConfidence:  0.9,
		},
		Metadata: protocol.Metadata{
			Target: &protocol.Target{
				Mode: protocol.TargetBranch, SnapshotHash: private, HeadRevision: private, BaseRevision: private,
				Files: []protocol.ReviewedFile{{FilePath: private, LineRanges: []protocol.LineRange{{StartLine: 1, EndLine: 1}}}},
			},
			Provider:         &protocol.Provider{Name: protocol.ProviderCursor, Model: private, Version: private},
			Attempts:         []protocol.Attempt{{Number: 1, Outcome: protocol.AttemptMalformed}, {Number: 2, Outcome: protocol.AttemptValid}},
			WebAccess:        true,
			ProtocolRecovery: protocol.ProtocolRecovery{Applied: true, Strategy: &recovery},
		},
	}
	metrics := NewMetrics()
	metrics.SetBundleBytes(70 << 10)
	metrics.Observe(phase.Measurement{Name: phase.ProviderProcess, Duration: 2 * time.Second})
	event := metrics.Event("v1.2.3", report)
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), private) {
		t.Fatalf("private report data entered telemetry: %s", encoded)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"attempt_outcomes", "bundle_bucket", "cli_version", "finding_count_bucket", "outcome",
		"phase_duration_buckets", "protocol_recovery_applied", "protocol_recovery_strategy", "provider",
		"report_schema_version", "schema_version", "target_mode", "web_access",
	}
	got := make([]string, 0, len(fields))
	for field := range fields {
		got = append(got, field)
	}
	slices.Sort(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("event fields = %v, want %v", got, want)
	}
	if event.BundleBucket != "64KiB-1MiB" || event.FindingCountBucket != "1" || event.PhaseDurationBuckets[string(phase.ProviderProcess)] != "1-9s" {
		t.Fatalf("event buckets = %+v", event)
	}
	if err := event.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestFailureEventOmitsFailureProse(t *testing.T) {
	const private = "PRIVATE-FAILURE-PROSE"
	report := protocol.Report{
		SchemaVersion: protocol.SchemaVersion,
		Status:        protocol.StatusFailure,
		Failure:       &protocol.Failure{Class: protocol.FailureAuth, Message: private},
		Metadata:      protocol.Metadata{Attempts: []protocol.Attempt{}},
	}
	encoded, err := json.Marshal(NewMetrics().Event("development", report))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), private) || !strings.Contains(string(encoded), `"failure_class":"authentication"`) {
		t.Fatalf("failure event = %s", encoded)
	}
}

func TestDecodeEventRejectsUnknownDuplicateAndMissingFields(t *testing.T) {
	valid, err := json.Marshal(validEvent())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeEvent(valid); err != nil {
		t.Fatalf("valid event: %v", err)
	}
	unknown := append([]byte(`{"prompt":"PRIVATE",`), valid[1:]...)
	duplicate := append([]byte(`{"outcome":"clean",`), valid[1:]...)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(valid, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "bundle_bucket")
	missing, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	nullBoolean := bytes.Replace(valid, []byte(`"web_access":false`), []byte(`"web_access":null`), 1)
	nullArray := bytes.Replace(valid, []byte(`"attempt_outcomes":["valid"]`), []byte(`"attempt_outcomes":null`), 1)
	nullMap := bytes.Replace(valid, []byte(`"phase_duration_buckets":{}`), []byte(`"phase_duration_buckets":null`), 1)
	duplicateMap := bytes.Replace(valid, []byte(`"phase_duration_buckets":{}`), []byte(`"phase_duration_buckets":{"config":"0","config":"0"}`), 1)
	for name, data := range map[string][]byte{
		"unknown": unknown, "duplicate": duplicate, "missing": missing,
		"null boolean": nullBoolean, "null array": nullArray, "null map": nullMap, "duplicate map": duplicateMap,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeEvent(data); err == nil {
				t.Fatalf("decodeEvent(%s) accepted %s", name, data)
			}
		})
	}
}
