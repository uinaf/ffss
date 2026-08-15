package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"

	"github.com/uinaf/ffsstack/cli/slopmachine/internal/machine"
)

// Event is one persisted machine transition for a run.
type Event struct {
	Seq           int
	At            string
	Command       string
	FromState     string
	ToState       string
	EvidenceJSON  string
	TelemetryJSON string
}

// RunSummary is a compact projector row for listing runs.
type RunSummary struct {
	ID             string
	State          string
	Released       bool
	CurrentUnitID  string
	CompletedUnits int
	SeriesBound    int
	UpdatedAt      string
	Open           bool
}

// ListRuns returns runs for repoKey newest-first (open and closed).
func (s *Store) ListRuns(repoKey string) ([]RunSummary, error) {
	rows, err := s.db.Query(`SELECT
		id, state, intake_revision, released_revision, current_unit_id, completed_units,
		series_bound, updated_at, open
	FROM runs WHERE repo_key = ?
	ORDER BY updated_at DESC, created_at DESC, rowid DESC`, repoKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []RunSummary
	for rows.Next() {
		var r RunSummary
		var intake int64
		var released sql.NullInt64
		var open int
		if err := rows.Scan(
			&r.ID, &r.State, &intake, &released, &r.CurrentUnitID, &r.CompletedUnits,
			&r.SeriesBound, &r.UpdatedAt, &open,
		); err != nil {
			return nil, err
		}
		run := machine.Run{IntakeRevision: intake}
		if released.Valid {
			v := released.Int64
			run.ReleasedRevision = &v
		}
		r.Released = run.Released()
		r.Open = open == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListEvents returns the transition timeline for a run in repoKey, oldest-first.
// runs.id is the global primary key, so run_id uniquely identifies one repo's run.
// Reads stay outside Begin() so _txlock=immediate does not take a write lock.
func (s *Store) ListEvents(repoKey, runID string) ([]Event, error) {
	if _, _, err := s.getRunForRepo(repoKey, runID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT e.seq, e.at, e.command, e.from_state, e.to_state, e.evidence_json, e.telemetry_json
		FROM events e JOIN runs r ON r.id = e.run_id
		WHERE e.run_id = ? AND r.repo_key = ?
		ORDER BY e.seq ASC`, runID, repoKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Seq, &e.At, &e.Command, &e.FromState, &e.ToState, &e.EvidenceJSON, &e.TelemetryJSON); err != nil {
			return nil, err
		}
		if e.EvidenceJSON == "" {
			e.EvidenceJSON = "{}"
		}
		if e.TelemetryJSON == "" {
			e.TelemetryJSON = "{}"
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Delivery is one persisted delivery event for a unit. Seq identifies the
// exact event, so an ABA re-delivery with identical evidence still reads as
// a different delivery.
type Delivery struct {
	Seq      int
	Evidence machine.DeliverEvidence
}

// LatestDeliveries returns the newest delivery per unit for a run.
// Events older than the unit-stamping change carry no unit and are skipped;
// their units stay observable through manual observe.
func (s *Store) LatestDeliveries(runID string) (map[string]Delivery, error) {
	rows, err := s.db.Query(`SELECT seq, evidence_json FROM events
		WHERE run_id = ? AND command = ? ORDER BY seq ASC`, runID, string(machine.CmdDeliver))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]Delivery)
	for rows.Next() {
		var seq int
		var raw string
		if err := rows.Scan(&seq, &raw); err != nil {
			return nil, err
		}
		var evidence machine.DeliverEvidence
		if err := json.Unmarshal([]byte(raw), &evidence); err != nil {
			return nil, fmt.Errorf("decode deliver evidence: %w", err)
		}
		if evidence.UnitID == "" {
			continue
		}
		out[evidence.UnitID] = Delivery{Seq: seq, Evidence: evidence}
	}
	return out, rows.Err()
}

// LastObservation returns the newest observe evidence recording signal for
// unit, or false when none exists. Watch compares its reference and thread
// tokens to tell already-recorded feedback from new feedback.
func (s *Store) LastObservation(runID, unitID string, signal machine.ObserveSignal) (machine.ObserveEvidence, bool, error) {
	rows, err := s.db.Query(`SELECT evidence_json FROM events
		WHERE run_id = ? AND command = ? ORDER BY seq DESC`, runID, string(machine.CmdObserve))
	if err != nil {
		return machine.ObserveEvidence{}, false, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return machine.ObserveEvidence{}, false, err
		}
		var evidence machine.ObserveEvidence
		if err := json.Unmarshal([]byte(raw), &evidence); err != nil {
			return machine.ObserveEvidence{}, false, fmt.Errorf("decode observe evidence: %w", err)
		}
		if evidence.UnitID == unitID && evidence.Signal == signal {
			return evidence, true, rows.Err()
		}
	}
	return machine.ObserveEvidence{}, false, rows.Err()
}

// Totals aggregates recorded telemetry across a run's events.
type Totals struct {
	DurationMS int64
	Tokens     int
	CostCents  int
	// RecordedEvents counts events that carried any telemetry.
	RecordedEvents int
}

// TelemetryTotals sums recorded telemetry for a run. Events without
// telemetry contribute nothing; totals over none are zero.
func (s *Store) TelemetryTotals(runID string) (Totals, error) {
	rows, err := s.db.Query(`SELECT telemetry_json FROM events WHERE run_id = ?`, runID)
	if err != nil {
		return Totals{}, err
	}
	defer func() { _ = rows.Close() }()
	var totals Totals
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return Totals{}, err
		}
		var t machine.Telemetry
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			return Totals{}, fmt.Errorf("decode event telemetry: %w", err)
		}
		if t.IsZero() {
			continue
		}
		totals.DurationMS = SaturatingAdd64(totals.DurationMS, t.DurationMS)
		totals.Tokens = SaturatingAddInt(totals.Tokens, t.Tokens)
		totals.CostCents = SaturatingAddInt(totals.CostCents, t.CostCents)
		totals.RecordedEvents++
	}
	return totals, rows.Err()
}

// Validation bounds each recorded value, but rows written by other tools
// bypass it; totals saturate instead of wrapping negative.
func SaturatingAdd64(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	if b < 0 && a < math.MinInt64-b {
		return math.MinInt64
	}
	return a + b
}

func SaturatingAddInt(a, b int) int {
	if b > 0 && a > math.MaxInt-b {
		return math.MaxInt
	}
	if b < 0 && a < math.MinInt-b {
		return math.MinInt
	}
	return a + b
}

// GetRunProjection loads run, units, and events for a read-only projector.
func (s *Store) GetRunProjection(repoKey, runID string) (machine.Run, []machine.Unit, []Event, error) {
	run, units, err := s.getRunForRepo(repoKey, runID)
	if err != nil {
		return machine.Run{}, nil, nil, err
	}
	events, err := s.ListEvents(repoKey, runID)
	if err != nil {
		return machine.Run{}, nil, nil, err
	}
	return run, units, events, nil
}
