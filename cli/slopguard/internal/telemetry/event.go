package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sync"
	"time"

	"github.com/uinaf/ffss/cli/slopguard/internal/phase"
	"github.com/uinaf/ffss/cli/slopguard/internal/protocol"
)

const SchemaVersion = "1"

var releaseVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

var eventFields = map[string]bool{
	"schema_version": true, "cli_version": true, "report_schema_version": true,
	"provider": false, "target_mode": false, "web_access": true,
	"outcome": true, "failure_class": false, "attempt_outcomes": true,
	"protocol_recovery_applied": true, "protocol_recovery_strategy": false,
	"bundle_bucket": true, "finding_count_bucket": true, "phase_duration_buckets": true,
}

type Event struct {
	SchemaVersion            string            `json:"schema_version"`
	CLIVersion               string            `json:"cli_version"`
	ReportSchemaVersion      string            `json:"report_schema_version"`
	Provider                 string            `json:"provider,omitempty"`
	TargetMode               string            `json:"target_mode,omitempty"`
	WebAccess                bool              `json:"web_access"`
	Outcome                  string            `json:"outcome"`
	FailureClass             string            `json:"failure_class,omitempty"`
	AttemptOutcomes          []string          `json:"attempt_outcomes"`
	ProtocolRecoveryApplied  bool              `json:"protocol_recovery_applied"`
	ProtocolRecoveryStrategy string            `json:"protocol_recovery_strategy,omitempty"`
	BundleBucket             string            `json:"bundle_bucket"`
	FindingCountBucket       string            `json:"finding_count_bucket"`
	PhaseDurationBuckets     map[string]string `json:"phase_duration_buckets"`
}

type Metrics struct {
	mu          sync.Mutex
	bundleBytes int64
	bundleSet   bool
	phases      map[phase.Name]time.Duration
}

func NewMetrics() *Metrics {
	return &Metrics{phases: make(map[phase.Name]time.Duration)}
}

func (metrics *Metrics) Observe(measurement phase.Measurement) {
	if metrics == nil {
		return
	}
	metrics.mu.Lock()
	metrics.phases[measurement.Name] += measurement.Duration
	metrics.mu.Unlock()
}

func (metrics *Metrics) SetBundleBytes(value int64) {
	if metrics == nil {
		return
	}
	metrics.mu.Lock()
	metrics.bundleBytes = max(value, 0)
	metrics.bundleSet = true
	metrics.mu.Unlock()
}

func (metrics *Metrics) Event(cliVersion string, report protocol.Report) Event {
	event := Event{
		SchemaVersion:           SchemaVersion,
		CLIVersion:              cliVersion,
		ReportSchemaVersion:     report.SchemaVersion,
		WebAccess:               report.Metadata.WebAccess,
		Outcome:                 string(report.Status),
		AttemptOutcomes:         make([]string, 0, len(report.Metadata.Attempts)),
		FindingCountBucket:      findingCountBucket(report),
		PhaseDurationBuckets:    make(map[string]string),
		BundleBucket:            "unavailable",
		ProtocolRecoveryApplied: report.Metadata.ProtocolRecovery.Applied,
	}
	if report.Metadata.Provider != nil {
		event.Provider = string(report.Metadata.Provider.Name)
	}
	if report.Metadata.Target != nil {
		event.TargetMode = string(report.Metadata.Target.Mode)
	}
	if report.Failure != nil {
		event.FailureClass = string(report.Failure.Class)
	}
	for _, attempt := range report.Metadata.Attempts {
		event.AttemptOutcomes = append(event.AttemptOutcomes, string(attempt.Outcome))
	}
	if report.Metadata.ProtocolRecovery.Strategy != nil {
		event.ProtocolRecoveryStrategy = string(*report.Metadata.ProtocolRecovery.Strategy)
	}
	if metrics != nil {
		metrics.mu.Lock()
		if metrics.bundleSet {
			event.BundleBucket = byteBucket(metrics.bundleBytes)
		}
		for name, duration := range metrics.phases {
			event.PhaseDurationBuckets[string(name)] = durationBucket(duration)
		}
		metrics.mu.Unlock()
	}
	return event
}

func (event Event) Validate() error {
	if event.SchemaVersion != SchemaVersion || event.ReportSchemaVersion != protocol.SchemaVersion {
		return fmt.Errorf("unsupported telemetry schema")
	}
	if event.CLIVersion != "development" && !releaseVersionPattern.MatchString(event.CLIVersion) {
		return fmt.Errorf("invalid telemetry CLI version")
	}
	switch protocol.Status(event.Outcome) {
	case protocol.StatusClean, protocol.StatusFindings, protocol.StatusFailure:
	default:
		return fmt.Errorf("invalid telemetry outcome")
	}
	if event.Provider != "" {
		switch protocol.ProviderName(event.Provider) {
		case protocol.ProviderCodex, protocol.ProviderClaude, protocol.ProviderCursor, protocol.ProviderGrok:
		default:
			return fmt.Errorf("invalid telemetry provider")
		}
	}
	if event.TargetMode != "" {
		switch protocol.TargetMode(event.TargetMode) {
		case protocol.TargetLocal, protocol.TargetBranch, protocol.TargetCommit:
		default:
			return fmt.Errorf("invalid telemetry target mode")
		}
	}
	if event.FailureClass != "" {
		switch protocol.FailureClass(event.FailureClass) {
		case protocol.FailureConfig, protocol.FailureTarget, protocol.FailureSecretScan, protocol.FailureCapability,
			protocol.FailureAuth, protocol.FailureTimeout, protocol.FailureCancelled, protocol.FailureProvider,
			protocol.FailureProtocol, protocol.FailureSourceChanged, protocol.FailureInternal:
		default:
			return fmt.Errorf("invalid telemetry failure class")
		}
	}
	if len(event.AttemptOutcomes) > 2 {
		return fmt.Errorf("too many telemetry attempt outcomes")
	}
	for _, outcome := range event.AttemptOutcomes {
		switch protocol.AttemptOutcome(outcome) {
		case protocol.AttemptValid, protocol.AttemptMalformed, protocol.AttemptFailed:
		default:
			return fmt.Errorf("invalid telemetry attempt outcome")
		}
	}
	if event.ProtocolRecoveryStrategy != "" {
		if !event.ProtocolRecoveryApplied || protocol.RecoveryStrategy(event.ProtocolRecoveryStrategy) != protocol.RecoveryCursorTrailingObject {
			return fmt.Errorf("invalid telemetry recovery strategy")
		}
	}
	if !oneOf(event.BundleBucket, "unavailable", "0", "1-64KiB", "64KiB-1MiB", "1-8MiB", "8MiB+") {
		return fmt.Errorf("invalid telemetry bundle bucket")
	}
	if !oneOf(event.FindingCountBucket, "0", "1", "2-5", "6-20", "21+") {
		return fmt.Errorf("invalid telemetry finding bucket")
	}
	if len(event.PhaseDurationBuckets) > 9 {
		return fmt.Errorf("too many telemetry phase buckets")
	}
	for name, bucket := range event.PhaseDurationBuckets {
		if !oneOf(name,
			string(phase.Config), string(phase.DependencyProbes), string(phase.TargetFreeze), string(phase.SecretScan),
			string(phase.ProviderPreparation), string(phase.ProviderProcess), string(phase.ProtocolDecode),
			string(phase.SourceRevalidation), string(phase.ReportWrite),
		) || !oneOf(bucket, "0", "under-10ms", "10-99ms", "100-999ms", "1-9s", "10-59s", "60s+") {
			return fmt.Errorf("invalid telemetry phase bucket")
		}
	}
	return nil
}

func decodeEvent(data []byte) (Event, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return Event{}, fmt.Errorf("telemetry event must be one JSON object")
	}
	seen := make(map[string]bool, len(eventFields))
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return Event{}, err
		}
		field, ok := token.(string)
		if !ok {
			return Event{}, fmt.Errorf("telemetry event field must be a string")
		}
		if _, known := eventFields[field]; !known {
			return Event{}, fmt.Errorf("unknown telemetry event field %q", field)
		}
		if seen[field] {
			return Event{}, fmt.Errorf("duplicate telemetry event field %q", field)
		}
		seen[field] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return Event{}, err
		}
		if err := validateEventField(field, value); err != nil {
			return Event{}, err
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return Event{}, fmt.Errorf("telemetry event object is incomplete")
	}
	if token, err := decoder.Token(); err != io.EOF || token != nil {
		return Event{}, fmt.Errorf("telemetry event has trailing content")
	}
	for field, required := range eventFields {
		if required && !seen[field] {
			return Event{}, fmt.Errorf("telemetry event field %q is required", field)
		}
	}
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return Event{}, err
	}
	if err := event.Validate(); err != nil {
		return Event{}, err
	}
	return event, nil
}

func validateEventField(field string, data json.RawMessage) error {
	switch field {
	case "web_access", "protocol_recovery_applied":
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("telemetry event field %q must be a boolean", field)
		}
	case "attempt_outcomes":
		return validateStringArray(field, data)
	case "phase_duration_buckets":
		return validateStringMap(field, data)
	default:
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		if _, ok := value.(string); !ok {
			return fmt.Errorf("telemetry event field %q must be a string", field)
		}
	}
	return nil
}

func validateStringArray(field string, data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('[') {
		return fmt.Errorf("telemetry event field %q must be a string array", field)
	}
	for decoder.More() {
		value, err := decoder.Token()
		if err != nil {
			return err
		}
		if _, ok := value.(string); !ok {
			return fmt.Errorf("telemetry event field %q must contain strings", field)
		}
	}
	return validateCollectionEnd(decoder, json.Delim(']'))
}

func validateStringMap(field string, data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return fmt.Errorf("telemetry event field %q must be a string map", field)
	}
	seen := make(map[string]bool)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok || seen[key] {
			return fmt.Errorf("telemetry event field %q has an invalid or duplicate key", field)
		}
		seen[key] = true
		value, err := decoder.Token()
		if err != nil {
			return err
		}
		if _, ok := value.(string); !ok {
			return fmt.Errorf("telemetry event field %q must contain strings", field)
		}
	}
	return validateCollectionEnd(decoder, json.Delim('}'))
}

func validateCollectionEnd(decoder *json.Decoder, want json.Delim) error {
	closing, err := decoder.Token()
	if err != nil || closing != want {
		return fmt.Errorf("telemetry event collection is incomplete")
	}
	if token, err := decoder.Token(); err != io.EOF || token != nil {
		return fmt.Errorf("telemetry event collection has trailing content")
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func byteBucket(value int64) string {
	switch {
	case value == 0:
		return "0"
	case value <= 64<<10:
		return "1-64KiB"
	case value <= 1<<20:
		return "64KiB-1MiB"
	case value <= 8<<20:
		return "1-8MiB"
	default:
		return "8MiB+"
	}
}

func findingCountBucket(report protocol.Report) string {
	count := 0
	if report.Review != nil {
		count = len(report.Review.Findings)
	}
	switch {
	case count == 0:
		return "0"
	case count == 1:
		return "1"
	case count <= 5:
		return "2-5"
	case count <= 20:
		return "6-20"
	default:
		return "21+"
	}
}

func durationBucket(value time.Duration) string {
	switch {
	case value <= 0:
		return "0"
	case value < 10*time.Millisecond:
		return "under-10ms"
	case value < 100*time.Millisecond:
		return "10-99ms"
	case value < time.Second:
		return "100-999ms"
	case value < 10*time.Second:
		return "1-9s"
	case value < time.Minute:
		return "10-59s"
	default:
		return "60s+"
	}
}
