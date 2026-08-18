package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/uinaf/ffss/cli/slopmachine/internal/forge"
	"github.com/uinaf/ffss/cli/slopmachine/internal/machine"
	"github.com/uinaf/ffss/cli/slopmachine/internal/status"
	"github.com/uinaf/ffss/cli/slopmachine/internal/store"
	"github.com/uinaf/ffss/cli/slopmachine/internal/watch"
)

const (
	watchDefaultIterations = 20
	watchMinIntervalSec    = 5
	watchMaxIntervalSec    = 3600
	watchMaxIterations     = 1000
)

type observationResult struct {
	Iteration int    `json:"iteration"`
	Unit      string `json:"unit"`
	Signal    string `json:"signal,omitempty"`
	Reference string `json:"reference,omitempty"`
	Recorded  bool   `json:"recorded"`
	Note      string `json:"note,omitempty"`
	ErrorKind string `json:"error_kind,omitempty"`
}

type watchDocument struct {
	SchemaVersion int                 `json:"schema_version"`
	RunID         string              `json:"run_id"`
	Iterations    int                 `json:"iterations"`
	Observations  []observationResult `json:"observations"`
	State         string              `json:"state"`
	NextAction    string              `json:"next_action"`
	Stopped       string              `json:"stopped"`
	// ErrorKind classifies a forge failure that aborted the watch (exit 7);
	// observations already recorded stay reported above it.
	ErrorKind        string `json:"error_kind,omitempty"`
	DryRun           bool   `json:"dry_run,omitempty"`
	ValidatedCommand string `json:"validated_command,omitempty"`
}

func cmdWatch(st *store.Store, args []string, opts runOptions) int {
	fs, code := requireFlags("watch", args, opts)
	if code != 0 {
		return code
	}
	once := fs["once"] == "1"
	intervalRaw, hasInterval := fs["interval"]
	if once && hasInterval {
		return writeFailure(opts, 2, fmt.Errorf("watch accepts either --once or --interval, not both"))
	}
	// A dry run records nothing, so every poll would be identical; the
	// combination validates exactly one pass instead of pretending to poll.
	if opts.dryRun && hasInterval {
		return writeFailure(opts, 2, fmt.Errorf("--dry-run validates a single pass; drop --interval or run without --dry-run"))
	}
	interval := 0
	if hasInterval {
		parsed, err := strconv.Atoi(intervalRaw)
		if err != nil || parsed < watchMinIntervalSec || parsed > watchMaxIntervalSec {
			return writeFailure(opts, 2, fmt.Errorf("--interval must be an integer between %d and %d seconds", watchMinIntervalSec, watchMaxIntervalSec))
		}
		interval = parsed
	}
	iterations := 1
	if hasInterval {
		iterations = watchDefaultIterations
	}
	if raw, ok := fs["iterations"]; ok {
		if !hasInterval {
			return writeFailure(opts, 2, fmt.Errorf("--iterations requires --interval"))
		}
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > watchMaxIterations {
			return writeFailure(opts, 2, fmt.Errorf("--iterations must be an integer between 1 and %d", watchMaxIterations))
		}
		iterations = parsed
	}
	runID := fs["run"]
	if err := validateRunID(runID); err != nil {
		return writeFailure(opts, 2, err)
	}
	key, err := resolveRepoKeyForOptions(st, opts)
	if err != nil {
		return writeFailure(opts, 2, err)
	}
	adapter, err := forge.New(forge.KindGitHub)
	if err != nil {
		return writeFailure(opts, 10, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return watchLoop(ctx, st, adapter, key, runID, iterations, interval, opts)
}

// watchLoop polls the forge for every delivered unit and records observation
// events. Bounded by iterations; a pass with no delivered units stops early.
func watchLoop(ctx context.Context, st *store.Store, adapter forge.Forge, repoKey, runID string, iterations, intervalSec int, opts runOptions) int {
	doc := watchDocument{SchemaVersion: 1, Observations: []observationResult{}}
	if opts.dryRun {
		doc.DryRun = true
		doc.ValidatedCommand = "watch"
	}
	stopped := "iterations exhausted"
	var run machine.Run
	var units []machine.Unit

	for pass := 1; pass <= iterations; pass++ {
		doc.Iterations = pass
		var err error
		run, units, err = st.ResolveActiveRun(repoKey, runID)
		if err != nil {
			return mapErr(err, opts)
		}
		// Pin the run on first resolution: an auto-resolved run that settles
		// mid-watch must not let later passes drift onto a different run.
		runID = run.ID
		doc.RunID = run.ID
		delivered := deliveredTargets(units)
		if len(delivered) == 0 {
			stopped = "no delivered unit awaits signals"
			break
		}
		deliveries, err := st.LatestDeliveries(run.ID)
		if err != nil {
			return mapErr(err, opts)
		}
		recordedAny := false
		aborted := false
		for _, unit := range delivered {
			result, abort := observeUnit(ctx, st, adapter, repoKey, run.ID, unit, deliveries, pass, opts)
			doc.Observations = append(doc.Observations, result)
			if abort == watchAbortForge {
				aborted = true
				// Later calls in this pass would fail identically; a later
				// interval pass may still recover (rate limits reset, auth
				// gets fixed), so only the final pass ends the watch here.
				if pass == iterations || opts.dryRun {
					// Emit everything already recorded alongside the failure
					// so partial success is never hidden behind the abort.
					doc.ErrorKind = "observation_" + result.ErrorKind
					stopped = result.Note
				}
				break
			}
			if abort != 0 {
				return abort
			}
			if result.Recorded {
				recordedAny = true
			}
		}
		if aborted && (pass == iterations || opts.dryRun) {
			if opts.dryRun {
				projectDryRunSignals(&run, &units, doc.Observations)
			}
			break
		}
		if recordedAny {
			// Phases changed; reload before deciding whether anything is left.
			run, units, err = st.ResolveActiveRun(repoKey, run.ID)
			if err != nil {
				return mapErr(err, opts)
			}
			if len(deliveredTargets(units)) == 0 {
				stopped = "every delivered unit settled or re-entered the build loop"
				break
			}
		}
		if opts.dryRun {
			projectDryRunSignals(&run, &units, doc.Observations)
			stopped = "dry run records nothing"
			break
		}
		// An interrupt during the pass (mid-forge-call) must not read as a
		// completed watch.
		if ctx.Err() != nil {
			stopped = "interrupted"
			break
		}
		if pass == iterations {
			break
		}
		select {
		case <-ctx.Done():
			stopped = "interrupted"
		case <-time.After(time.Duration(intervalSec) * time.Second):
			continue
		}
		break
	}

	// A concurrent writer may have advanced the run even when this watch
	// recorded nothing; the closing state and next_action reflect now.
	if doc.RunID != "" && !opts.dryRun {
		fresh, freshUnits, err := st.ResolveActiveRun(repoKey, doc.RunID)
		if err == nil {
			run, units = fresh, freshUnits
		}
	}
	doc.State = string(run.State)
	doc.NextAction = status.From(run, units).NextAction
	doc.Stopped = stopped
	// A final pass that could not observe something is not a clean wait:
	// exit 7 tells automation the forge answer is incomplete, while the
	// document still reports everything observed and recorded. Failures
	// recovered by a later interval pass do not poison the exit status.
	if doc.ErrorKind == "" {
		for _, observation := range doc.Observations {
			if observation.Iteration == doc.Iterations && observation.ErrorKind != "" {
				doc.ErrorKind = "observation_" + observation.ErrorKind
				break
			}
		}
	}
	code := writeWatch(doc, opts)
	if doc.ErrorKind != "" && code == 0 {
		return 7
	}
	return code
}

// watchAbortForge marks an auth or rate-limit forge failure: every later
// call would fail the same way, but committed observations must still be
// reported.
const watchAbortForge = -7

// projectDryRunSignals applies would-be signals through the machine in
// memory so a dry run's closing state and next_action match what a real
// pass would leave behind; nothing is saved.
func projectDryRunSignals(run *machine.Run, units *[]machine.Unit, observations []observationResult) {
	for _, observation := range observations {
		if observation.Signal == "" {
			continue
		}
		res, err := machine.Apply(*run, *units, machine.CmdObserve, machine.ApplyInput{
			Observe: &machine.ObserveEvidence{
				UnitID:    observation.Unit,
				Signal:    machine.ObserveSignal(observation.Signal),
				Reference: observation.Reference,
			},
		})
		if err != nil {
			continue
		}
		*run, *units = res.Run, res.Units
	}
}

func deliveredTargets(units []machine.Unit) []machine.Unit {
	out := make([]machine.Unit, 0)
	for _, u := range units {
		if u.Phase == machine.PhaseDelivered {
			out = append(out, u)
		}
	}
	return out
}

// observeUnit reads one unit's forge state and records at most one signal.
// It returns a non-zero exit code only for failures that make further calls
// pointless (auth, rate limit) or for store-level errors.
func observeUnit(ctx context.Context, st *store.Store, adapter forge.Forge, repoKey, runID string, unit machine.Unit, deliveries map[string]store.Delivery, pass int, opts runOptions) (observationResult, int) {
	result := observationResult{Iteration: pass, Unit: unit.ID}
	delivery, ok := deliveries[unit.ID]
	if !ok {
		// Legacy events without a stamped unit cannot be correlated; the
		// unit is unobservable data, not a clean wait.
		result.ErrorKind = "unobservable"
		result.Note = "no delivery evidence names this unit; record the signal manually with slopmachine observe"
		return result, 0
	}
	evidence := delivery.Evidence
	if evidence.PRURL == "" {
		if evidence.CommitSHA != "" {
			// Direct-trunk deliveries are manual-observe by design, not an
			// incomplete answer.
			result.Note = "direct-trunk delivery is not forge-observable; record signals manually with slopmachine observe"
			return result, 0
		}
		result.ErrorKind = "unobservable"
		result.Note = "delivery recorded no change request URL; record the signal manually with slopmachine observe"
		return result, 0
	}
	ref, err := adapter.ParseChangeRequestURL(evidence.PRURL)
	if err != nil {
		result.ErrorKind = "unobservable"
		result.Note = fmt.Sprintf("unrecognized change request URL %q: %v", evidence.PRURL, err)
		return result, 0
	}
	obs, err := adapter.Observe(ctx, ref)
	if err != nil {
		var forgeErr *forge.Error
		if errors.As(err, &forgeErr) {
			result.ErrorKind = string(forgeErr.Kind)
			switch forgeErr.Kind {
			case forge.ErrorAuth, forge.ErrorRateLimit:
				result.Note = fmt.Sprintf("forge not observable (%s) at %s; fix access (gh auth status) and rerun", forgeErr.Kind, ref)
				return result, watchAbortForge
			}
			result.Note = fmt.Sprintf("observation failed (%s); rerun watch or use --interval to retry", forgeErr.Kind)
			return result, 0
		}
		result.ErrorKind = "unknown"
		result.Note = fmt.Sprintf("observation failed: %v", err)
		return result, 0
	}
	outcome := watch.Decide(watch.Target{UnitID: unit.ID, PRURL: evidence.PRURL, CommitSHA: evidence.CommitSHA}, obs)
	result.Signal = string(outcome.Signal)
	result.Reference = outcome.Reference
	result.Note = outcome.Note
	if outcome.Signal == "" {
		// Even a nothing-to-record answer is stale if the delivery changed
		// while the forge call was in flight; the new delivery is unobserved.
		if note, fresh, err := deliveryStillFresh(st, repoKey, runID, unit.ID, delivery); err != nil {
			return result, mapErr(err, opts)
		} else if !fresh && note == deliveryChangedNote {
			result.Note = note
			result.ErrorKind = "conflict"
		}
		return result, 0
	}
	// Only NEW feedback pulls a unit back: a current unresolved set that is
	// a subset of what was already recorded (same threads, same newest
	// comments) carries nothing new — resolving some threads or re-delivering
	// does not re-trigger rework. Added threads, new comments, and (when the
	// forge exposes them) reopened threads produce unseen tokens. This runs
	// before the dry-run return so dry passes report exactly what a real
	// pass would record.
	var tokens []string
	if outcome.Signal == machine.SignalReviewFeedback {
		tokens = watch.ThreadTokens(obs)
		last, found, err := st.LastObservation(runID, unit.ID, machine.SignalReviewFeedback)
		if err != nil {
			return result, mapErr(err, opts)
		}
		// The subset shortcut is only sound when the sampled tokens cover the
		// complete unresolved set; beyond the sample bound an unseen thread
		// could hide behind it, so only exact reference equality dedups.
		// When the BASELINE was the incomplete one, a formerly hidden thread
		// entering the sample re-triggers once by design: with uncertain
		// coverage watch surfaces potentially new feedback rather than
		// suppressing it, and the re-recorded set completes the baseline.
		sampleComplete := obs.UnresolvedThreads <= len(tokens)
		if found && (last.Reference == outcome.Reference || (sampleComplete && feedbackSubset(tokens, last))) {
			result.Signal = ""
			result.Note = "no new feedback beyond what was already recorded; resolve the threads on the forge or record a new signal manually"
			// Like every other no-record return: a delivery that changed
			// mid-flight makes even this answer stale.
			if note, fresh, err := deliveryStillFresh(st, repoKey, runID, unit.ID, delivery); err != nil {
				return result, mapErr(err, opts)
			} else if !fresh && note == deliveryChangedNote {
				result.Note = note
				result.ErrorKind = "conflict"
			}
			return result, 0
		}
	}
	if opts.dryRun {
		// Dry runs run the same freshness check a real write performs, so a
		// concurrently reworked or re-delivered unit never reports stale
		// would-be work.
		if note, fresh, err := deliveryStillFresh(st, repoKey, runID, unit.ID, delivery); err != nil {
			return result, mapErr(err, opts)
		} else if !fresh {
			result.Signal = ""
			result.Note = note
			// Same classification a real write gives an unobserved re-delivery.
			if note == deliveryChangedNote {
				result.ErrorKind = "conflict"
			}
		}
		return result, 0
	}
	recorded, note, err := applyObserveQuiet(st, repoKey, runID, machine.ObserveEvidence{
		UnitID: unit.ID, Signal: outcome.Signal, Reference: outcome.Reference, ThreadTokens: tokens,
	}, delivery)
	if err != nil {
		return result, mapErr(err, opts)
	}
	result.Recorded = recorded
	if note != "" {
		result.Note = note
	}
	// A detected signal that was not recorded because the ledger moved under
	// us (CAS exhaustion, or a re-delivery whose new evidence is still
	// unobserved) never reads as a clean success; a unit another writer
	// settled or reworked is complete and stays a note.
	if !recorded && (note == casExhaustedNote || note == deliveryChangedNote) {
		result.ErrorKind = "conflict"
	}
	return result, 0
}

// casExhaustedNote marks bounded retry exhaustion under concurrent writers.
const casExhaustedNote = "concurrent writers kept changing the run; rerun watch to re-observe"

// deliveryChangedNote marks a re-delivery whose new evidence is unobserved.
const deliveryChangedNote = "delivery evidence changed while observing; rerun watch to observe the new delivery"

// applyObserveQuiet records one observe event without printing status; the
// watch document reports the aggregate outcome instead. The signal was
// decided from observedDelivery, so each attempt re-checks that the unit is
// still delivered under that same evidence (rework and re-delivery while
// the forge call was in flight invalidate the decision). Revision conflicts
// from concurrent writers retry against fresh state a bounded number of
// times; the machine's CAS protects every attempt.
func applyObserveQuiet(st *store.Store, repoKey, runID string, evidence machine.ObserveEvidence, observedDelivery store.Delivery) (bool, string, error) {
	const attempts = 3
	for attempt := 0; attempt < attempts; attempt++ {
		run, units, err := st.ResolveActiveRun(repoKey, runID)
		if err != nil {
			return false, "", err
		}
		note, fresh, err := deliveryFreshInState(units, run.ID, evidence.UnitID, observedDelivery, st)
		if err != nil {
			return false, "", err
		}
		if !fresh {
			return false, note, nil
		}
		res, err := machine.Apply(run, units, machine.CmdObserve, machine.ApplyInput{
			ExpectedRevision: run.Revision,
			Observe:          &evidence,
		})
		if err != nil {
			return false, "", err
		}
		err = st.SaveApply(res)
		if err == nil {
			return true, "", nil
		}
		if !errors.Is(err, machine.ErrRevisionConflict) {
			return false, "", err
		}
	}
	return false, casExhaustedNote, nil
}

// deliveryStillFresh reloads state and answers whether the observed unit is
// still delivered under the exact delivery event the decision was made from.
func deliveryStillFresh(st *store.Store, repoKey, runID, unitID string, observed store.Delivery) (string, bool, error) {
	run, units, err := st.ResolveActiveRun(repoKey, runID)
	if err != nil {
		return "", false, err
	}
	return deliveryFreshInState(units, run.ID, unitID, observed, st)
}

func deliveryFreshInState(units []machine.Unit, runID, unitID string, observed store.Delivery, st *store.Store) (string, bool, error) {
	delivered := false
	for _, unit := range units {
		if unit.ID == unitID && unit.Phase == machine.PhaseDelivered {
			delivered = true
			break
		}
	}
	if !delivered {
		return "the unit is no longer delivered (a concurrent observer or writer moved it); nothing recorded", false, nil
	}
	deliveries, err := st.LatestDeliveries(runID)
	if err != nil {
		return "", false, err
	}
	if deliveries[unitID] != observed {
		return deliveryChangedNote, false, nil
	}
	return "", true, nil
}

// feedbackSubset reports whether every current token was already recorded.
// Legacy observations without tokens fall back to reference equality.
func feedbackSubset(current []string, last machine.ObserveEvidence) bool {
	if len(last.ThreadTokens) == 0 || len(current) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(last.ThreadTokens))
	for _, token := range last.ThreadTokens {
		seen[token] = struct{}{}
	}
	for _, token := range current {
		if _, ok := seen[token]; !ok {
			return false
		}
	}
	return true
}

func writeWatch(doc watchDocument, opts runOptions) int {
	if opts.json {
		if err := writeJSON(doc); err != nil {
			return writeFailure(opts, 10, err)
		}
		return 0
	}
	prefix := "slopmachine watch"
	if doc.DryRun {
		prefix += " dry-run"
	}
	for _, obs := range doc.Observations {
		line := fmt.Sprintf("%s [%d] unit=%s", prefix, obs.Iteration, obs.Unit)
		if obs.Signal != "" {
			line += " signal=" + obs.Signal
			if obs.Recorded {
				line += " recorded"
			}
		}
		if obs.ErrorKind != "" {
			line += " error_kind=" + obs.ErrorKind
		}
		if obs.Note != "" {
			line += " note=" + obs.Note
		}
		fmt.Fprintln(os.Stdout, line)
	}
	summary := fmt.Sprintf("%s run=%s state=%s stopped=%s", prefix, doc.RunID, doc.State, doc.Stopped)
	if doc.ErrorKind != "" {
		summary += " error_kind=" + doc.ErrorKind
	}
	fmt.Fprintf(os.Stdout, "%s next=%s\n", summary, doc.NextAction)
	if doc.ErrorKind != "" {
		fmt.Fprintf(os.Stderr, "slopmachine watch: %s: see slopmachine watch --help for recovery\n", doc.ErrorKind)
	}
	return 0
}
