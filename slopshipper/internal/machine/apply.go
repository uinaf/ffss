package machine

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// NewRun constructs an INTAKE run after init.
func NewRun(id, repoKey string) Run {
	return Run{
		ID:                id,
		RepoKey:           repoKey,
		State:             StateIntake,
		IntakeRevision:    1,
		Revision:          1,
		DeliveryMode:      DeliveryPRHold,
		RequiredReviewers: []ReviewerIdentity{ReviewerAutoreview},
		SeriesBound:       1,
	}
}

// Apply mutates a deep copy of run/units according to cmd.
func Apply(run Run, units []Unit, cmd Command, in ApplyInput) (ApplyResult, error) {
	if in.ExpectedRevision != 0 && in.ExpectedRevision != run.Revision {
		return ApplyResult{}, fmt.Errorf("%w: expected %d got %d", ErrRevisionConflict, in.ExpectedRevision, run.Revision)
	}
	if !slices.Contains(AllowedCommands(run, units), cmd) {
		return ApplyResult{}, fmt.Errorf("%w: %s not allowed from %s", ErrIllegalTransition, cmd, run.State)
	}

	from := run.State
	units = cloneUnits(units)

	var err error
	switch cmd {
	case CmdIntake:
		err = applyIntake(&run, &units, in)
	case CmdRelease:
		err = applyRelease(&run, units, in)
	case CmdBuild:
		err = applyBuild(&run, &units)
	case CmdVerify:
		err = applyVerify(&run, in)
	case CmdReview:
		err = applyReview(&run, in)
	case CmdRework:
		err = applyRework(&run)
	case CmdDeliver:
		err = applyDeliver(&run, &units, in)
	case CmdAsk:
		err = applyAsk(&run, in)
	case CmdDecide:
		err = applyDecide(&run, units, in)
	case CmdRetry:
		err = applyRetry(&run, units, in)
	case CmdBlock:
		err = applyBlock(&run, in)
	case CmdObserve:
		err = applyObserve(&run, &units, in)
	default:
		return ApplyResult{}, fmt.Errorf("%w: unknown command %q", ErrBadArgs, cmd)
	}
	if err != nil {
		return ApplyResult{}, err
	}

	run.Revision++
	return ApplyResult{
		Run:       run,
		Units:     units,
		EventFrom: from,
		EventTo:   run.State,
		Command:   cmd,
		Evidence:  canonicalEvidence(run, units, cmd, in),
	}, nil
}

func applyIntake(run *Run, units *[]Unit, in ApplyInput) error {
	if in.Intake == nil {
		return fmt.Errorf("%w: intake patch required", ErrBadArgs)
	}
	patch := in.Intake
	if patch.DeliveryMode != nil {
		if err := validDelivery(*patch.DeliveryMode); err != nil {
			return err
		}
		run.DeliveryMode = *patch.DeliveryMode
	}
	if patch.RequiredReviewers != nil {
		required, err := validRequiredReviewers(patch.RequiredReviewers, in.RegisteredReviewers)
		if err != nil {
			return err
		}
		run.RequiredReviewers = required
	} else if len(run.RequiredReviewers) > 0 {
		// A retained set must still be registered, so dry runs agree with
		// the transactional guard at commit time.
		if _, err := validRequiredReviewers(run.RequiredReviewers, in.RegisteredReviewers); err != nil {
			return err
		}
	}
	if patch.RiskTier != nil {
		if err := validRiskTier(*patch.RiskTier); err != nil {
			return err
		}
		run.RiskTier = *patch.RiskTier
	}
	if patch.Budget != nil {
		if err := validBudget(*patch.Budget); err != nil {
			return err
		}
		run.Budget = *patch.Budget
	}
	if patch.SeriesBound != nil {
		if *patch.SeriesBound < 1 {
			return fmt.Errorf("%w: series_bound must be >= 1", ErrBadArgs)
		}
		run.SeriesBound = *patch.SeriesBound
	}
	if patch.Units != nil {
		if err := validateGraph(patch.Units); err != nil {
			return err
		}
		next := cloneUnits(patch.Units)
		for i := range next {
			next[i].Attempt = 0
			next[i].Phase = PhasePending
			next[i].ReworkCause = ""
		}
		*units = next
		run.CurrentUnitID = ""
		run.CompletedUnits = 0
	}
	run.IntakeRevision++
	run.ReleasedRevision = nil
	run.State = StateIntake
	run.DecisionQuestion = ""
	run.ReturnState = ""
	return nil
}

func applyRelease(run *Run, units []Unit, in ApplyInput) error {
	if in.IntakeRevision == 0 {
		return fmt.Errorf("%w: release requires --revision", ErrBadArgs)
	}
	if in.IntakeRevision != run.IntakeRevision {
		return fmt.Errorf("%w: release revision %d does not match intake_revision %d", ErrUnmetGuard, in.IntakeRevision, run.IntakeRevision)
	}
	// Re-check registration at the human latch: a reviewer unregistered
	// after intake must fail closed here, not at review time.
	allowed := registeredSet(in.RegisteredReviewers)
	for _, reviewer := range run.RequiredReviewers {
		if _, ok := allowed[reviewer]; !ok {
			return fmt.Errorf("%w: required reviewer %q is not registered", ErrUnmetGuard, reviewer)
		}
	}
	rev := run.IntakeRevision
	run.ReleasedRevision = &rev
	// Project the latched graph: signals recorded while unreleased may
	// already have settled every unit.
	if len(units) > 0 {
		projectResting(run, units)
	}
	return nil
}

func applyBuild(run *Run, units *[]Unit) error {
	if !run.Released() {
		return fmt.Errorf("%w: release required before build", ErrUnmetGuard)
	}
	if run.State == StateRework {
		if run.CurrentUnitID == "" {
			return fmt.Errorf("%w: rework requires current unit", ErrCorruptState)
		}
		u := findUnit(*units, run.CurrentUnitID)
		if u == nil {
			return fmt.Errorf("%w: current unit missing", ErrCorruptState)
		}
		u.Attempt++
		run.CompletedReviewers = nil
		run.State = StateBuild
		run.DecisionQuestion = ""
		run.ReturnState = ""
		return nil
	}

	// Rework-phase units re-enter the pipeline before fresh frontier claims;
	// their earlier delivery no longer counts toward the series bound.
	if next := firstReworkUnit(*units); next != nil {
		claimUnit(run, next)
		return nil
	}
	next, err := frontierUnit(*units)
	if err != nil {
		return err
	}
	if next == nil {
		return fmt.Errorf("%w: no frontier unit to build", ErrUnmetGuard)
	}
	if settledCount(*units) >= run.SeriesBound {
		return fmt.Errorf("%w: series_bound %d reached", ErrUnmetGuard, run.SeriesBound)
	}
	claimUnit(run, next)
	return nil
}

func claimUnit(run *Run, unit *Unit) {
	run.CurrentUnitID = unit.ID
	unit.Phase = PhaseActive
	unit.ReworkCause = ""
	unit.Attempt++
	run.CompletedReviewers = nil
	run.State = StateBuild
	run.BlockerReason = ""
	run.DecisionQuestion = ""
	run.ReturnState = ""
}

func applyVerify(run *Run, in ApplyInput) error {
	if run.CurrentUnitID == "" {
		return fmt.Errorf("%w: no current unit", ErrUnmetGuard)
	}
	if err := validateVerifyCommand(in.Verify); err != nil {
		return err
	}
	// Failure path → BLOCKED
	if in.Verify.ExitCode != 0 {
		run.State = StateBlocked
		run.BlockerReason = fmt.Sprintf("verify failed: %s exit %d", in.Verify.Command, in.Verify.ExitCode)
		return nil
	}
	run.State = StateReview
	run.BlockerReason = ""
	return nil
}

func applyReview(run *Run, in ApplyInput) error {
	if err := validateReviewEvidence(in.Review, run.RequiredReviewers); err != nil {
		return err
	}
	switch in.Review.Verdict {
	case ReviewClean:
		if slices.Contains(run.CompletedReviewers, in.Review.Reviewer) {
			return fmt.Errorf("%w: reviewer %q already recorded", ErrUnmetGuard, in.Review.Reviewer)
		}
		run.CompletedReviewers = append(run.CompletedReviewers, in.Review.Reviewer)
		if reviewsSatisfied(run.RequiredReviewers, run.CompletedReviewers) {
			run.State = StateDeliver
		}
	case ReviewFindings:
		run.CompletedReviewers = nil
		run.State = StateRework
	case ReviewAmbiguous:
		run.ReturnState = StateReview
		run.State = StateNeedsDecision
		run.DecisionQuestion = "Review outcome is ambiguous; decide how to proceed"
	}
	return nil
}

func applyRework(run *Run) error {
	run.CompletedReviewers = nil
	run.State = StateRework
	return nil
}

func applyDeliver(run *Run, units *[]Unit, in ApplyInput) error {
	if err := validateDeliverEvidence(in.Deliver, run.DeliveryMode); err != nil {
		return err
	}
	u := findUnit(*units, run.CurrentUnitID)
	if u == nil {
		return fmt.Errorf("%w: current unit missing", ErrCorruptState)
	}
	// Delivery opens a change request; the unit settles only when an
	// observed external signal closes it (observe --signal merged).
	u.Phase = PhaseDelivered
	u.ReworkCause = ""
	run.CurrentUnitID = ""
	run.CompletedReviewers = nil
	projectResting(run, *units)
	return nil
}

// projectResting derives the run's resting state and delivery count from
// authoritative unit phases once no unit is active in the pipeline.
func projectResting(run *Run, units []Unit) {
	run.CompletedUnits = settledCount(units)
	if firstReworkUnit(units) != nil {
		// Park on released INTAKE: build re-claims the rework unit next.
		run.State = StateIntake
		return
	}
	frontier, _ := frontierUnit(units)
	if frontier != nil && run.CompletedUnits < run.SeriesBound {
		run.State = StateIntake
		return
	}
	if anyDelivered(units) {
		run.State = StateAwaitingSignals
		return
	}
	run.State = StateRunDone
}

func applyObserve(run *Run, units *[]Unit, in ApplyInput) error {
	if in.Observe == nil {
		return fmt.Errorf("%w: observe evidence required", ErrBadArgs)
	}
	switch in.Observe.Signal {
	case SignalMerged, SignalChecksFailed, SignalReviewFeedback:
	default:
		return fmt.Errorf("%w: observe.signal must be merged|checks_failed|review_feedback", ErrBadArgs)
	}
	unit, err := resolveDeliveredUnit(*units, in.Observe.UnitID)
	if err != nil {
		return err
	}
	in.Observe.UnitID = unit.ID
	switch in.Observe.Signal {
	case SignalMerged:
		unit.Phase = PhaseDone
		unit.ReworkCause = ""
	default:
		unit.Phase = PhaseRework
		cause := string(in.Observe.Signal)
		if strings.TrimSpace(in.Observe.Reference) != "" {
			cause += ": " + strings.TrimSpace(in.Observe.Reference)
		}
		unit.ReworkCause = cause
	}
	// Only released resting runs re-project; unreleased, mid-pipeline, and
	// human-parked states (BLOCKED, NEEDS_DECISION) record the phase change
	// without moving the run, so a signal can never settle an amended
	// contract or disturb an active question.
	if (run.State == StateIntake || run.State == StateAwaitingSignals) && run.Released() {
		projectResting(run, *units)
	} else {
		run.CompletedUnits = settledCount(*units)
	}
	return nil
}

func resolveDeliveredUnit(units []Unit, id string) (*Unit, error) {
	if id != "" {
		unit := findUnit(units, id)
		if unit == nil {
			return nil, fmt.Errorf("%w: unit %q not found", ErrBadArgs, id)
		}
		if unit.Phase != PhaseDelivered {
			return nil, fmt.Errorf("%w: unit %q is %s, not delivered", ErrUnmetGuard, id, unit.Phase)
		}
		return unit, nil
	}
	var match *Unit
	for i := range units {
		if units[i].Phase != PhaseDelivered {
			continue
		}
		if match != nil {
			return nil, fmt.Errorf("%w: multiple delivered units; pass observe.unit", ErrBadArgs)
		}
		match = &units[i]
	}
	if match == nil {
		return nil, fmt.Errorf("%w: no delivered unit awaits signals", ErrUnmetGuard)
	}
	return match, nil
}

func settledCount(units []Unit) int {
	count := 0
	for i := range units {
		if units[i].Settled() {
			count++
		}
	}
	return count
}

func anyDelivered(units []Unit) bool {
	for i := range units {
		if units[i].Phase == PhaseDelivered {
			return true
		}
	}
	return false
}

// firstReworkUnit returns the first rework-phase unit whose blockers are
// all settled, so a dependent never rebuilds on a churning prerequisite.
func firstReworkUnit(units []Unit) *Unit {
	settled := map[string]bool{}
	for i := range units {
		if units[i].Settled() {
			settled[units[i].ID] = true
		}
	}
	for i := range units {
		if units[i].Phase != PhaseRework {
			continue
		}
		ready := true
		for _, b := range units[i].Blockers {
			if !settled[b] {
				ready = false
				break
			}
		}
		if ready {
			return &units[i]
		}
	}
	return nil
}

func applyAsk(run *Run, in ApplyInput) error {
	if in.Question == "" {
		return fmt.Errorf("%w: ask requires --question", ErrBadArgs)
	}
	run.ReturnState = run.State
	run.State = StateNeedsDecision
	run.DecisionQuestion = in.Question
	return nil
}

func applyDecide(run *Run, units []Unit, in ApplyInput) error {
	if in.Decision == nil || in.Decision.Answer == "" {
		return fmt.Errorf("%w: decision.answer required", ErrBadArgs)
	}
	ret := run.ReturnState
	if ret == "" {
		ret = StateIntake
	}
	run.DecisionQuestion = ""
	run.ReturnState = ""
	// Observations recorded while parked can make a remembered resting
	// state stale; re-project instead of restoring it verbatim. Unreleased
	// runs never observe, so they resume literally.
	if (ret == StateIntake || ret == StateAwaitingSignals) && run.Released() && len(units) > 0 {
		run.State = ret
		projectResting(run, units)
		return nil
	}
	run.State = ret
	return nil
}

func applyRetry(run *Run, units []Unit, in ApplyInput) error {
	if strings.TrimSpace(in.RetryReason) == "" {
		return fmt.Errorf("%w: retry reason required", ErrBadArgs)
	}
	if run.CurrentUnitID == "" {
		return fmt.Errorf("%w: blocked run has no current unit", ErrCorruptState)
	}
	unit := findUnit(units, run.CurrentUnitID)
	if unit == nil || unit.Settled() {
		return fmt.Errorf("%w: blocked run current unit %q is missing or complete", ErrCorruptState, run.CurrentUnitID)
	}
	run.State = StateBuild
	run.BlockerReason = ""
	run.CompletedReviewers = nil
	return nil
}

func applyBlock(run *Run, in ApplyInput) error {
	if in.BlockReason == "" {
		return fmt.Errorf("%w: block reason required", ErrBadArgs)
	}
	run.State = StateBlocked
	run.BlockerReason = in.BlockReason
	return nil
}

func AllowedCommands(run Run, units []Unit) []Command {
	withObserve := func(commands []Command) []Command {
		if anyDelivered(units) {
			return append(commands, CmdObserve)
		}
		return commands
	}
	switch run.State {
	case StateIntake:
		out := []Command{CmdIntake, CmdAsk}
		if run.Released() {
			buildable := firstReworkUnit(units) != nil
			if !buildable {
				if u, _ := frontierUnit(units); u != nil && settledCount(units) < run.SeriesBound {
					buildable = true
				}
			}
			if buildable {
				out = append(out, CmdBuild)
			}
		} else if len(units) > 0 {
			out = append(out, CmdRelease)
		}
		return withObserve(out)
	case StateBuild:
		return withObserve([]Command{CmdVerify, CmdAsk, CmdBlock})
	case StateVerify:
		return withObserve([]Command{CmdVerify, CmdAsk, CmdBlock})
	case StateReview:
		return withObserve([]Command{CmdReview, CmdRework, CmdAsk, CmdBlock})
	case StateDeliver:
		return withObserve([]Command{CmdDeliver, CmdAsk})
	case StateRework:
		return withObserve([]Command{CmdBuild, CmdAsk})
	case StateAwaitingSignals:
		return withObserve([]Command{CmdIntake, CmdAsk})
	case StateNeedsDecision:
		return withObserve([]Command{CmdDecide})
	case StateBlocked:
		return withObserve([]Command{CmdRetry})
	case StateRunDone:
		return nil
	default:
		return nil
	}
}

func validRiskTier(tier RiskTier) error {
	switch tier {
	case RiskLow, RiskMedium, RiskHigh:
		return nil
	default:
		return fmt.Errorf("%w: risk_tier must be low|medium|high", ErrBadArgs)
	}
}

func validComplexity(c Complexity) error {
	switch c {
	case "", ComplexityLow, ComplexityMedium, ComplexityHigh:
		return nil
	default:
		return fmt.Errorf("%w: complexity must be low|medium|high", ErrBadArgs)
	}
}

const (
	maxAcceptanceCriteria       = 32
	maxAcceptanceCriterionBytes = 500
)

func validAcceptanceCriteria(unitID string, criteria []string) error {
	if len(criteria) > maxAcceptanceCriteria {
		return fmt.Errorf("%w: unit %q has more than %d acceptance criteria", ErrBadArgs, unitID, maxAcceptanceCriteria)
	}
	for _, criterion := range criteria {
		if strings.TrimSpace(criterion) == "" {
			return fmt.Errorf("%w: unit %q has an empty acceptance criterion", ErrBadArgs, unitID)
		}
		if len(criterion) > maxAcceptanceCriterionBytes {
			return fmt.Errorf("%w: unit %q acceptance criterion exceeds %d bytes", ErrBadArgs, unitID, maxAcceptanceCriterionBytes)
		}
		for _, r := range criterion {
			if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
				return fmt.Errorf("%w: unit %q acceptance criterion must be a single line without control characters", ErrBadArgs, unitID)
			}
		}
	}
	return nil
}

func validBudget(b Budget) error {
	if b.Tokens < 0 || b.Minutes < 0 {
		return fmt.Errorf("%w: budget dimensions must be positive when set", ErrBadArgs)
	}
	return nil
}

func validDelivery(m DeliveryMode) error {
	switch m {
	case DeliveryPRHold, DeliveryPRMergeWhenReady, DeliveryDirectTrunk:
		return nil
	default:
		return fmt.Errorf("%w: invalid delivery_mode %q", ErrBadArgs, m)
	}
}

// validRequiredReviewers validates the replacement set: non-empty, valid
// identities, no duplicates, every identity built-in or registered.
func validRequiredReviewers(required, registered []ReviewerIdentity) ([]ReviewerIdentity, error) {
	if len(required) == 0 {
		return nil, fmt.Errorf("%w: required_reviewers must name at least one registered reviewer", ErrBadArgs)
	}
	allowed := registeredSet(registered)
	seen := make(map[ReviewerIdentity]struct{}, len(required))
	out := make([]ReviewerIdentity, 0, len(required))
	for _, reviewer := range required {
		if err := ValidateResourceID("reviewer identity", string(reviewer)); err != nil {
			return nil, err
		}
		if _, dup := seen[reviewer]; dup {
			return nil, fmt.Errorf("%w: duplicate required reviewer %q", ErrBadArgs, reviewer)
		}
		if _, ok := allowed[reviewer]; !ok {
			return nil, fmt.Errorf("%w: reviewer %q is not registered; register it with slopshipper reviewers --add or use a built-in (autoreview, bugbot)", ErrBadArgs, reviewer)
		}
		seen[reviewer] = struct{}{}
		out = append(out, reviewer)
	}
	return out, nil
}

func registeredSet(registered []ReviewerIdentity) map[ReviewerIdentity]struct{} {
	allowed := make(map[ReviewerIdentity]struct{}, len(registered)+3)
	for _, reviewer := range BuiltinReviewers() {
		allowed[reviewer] = struct{}{}
	}
	for _, reviewer := range registered {
		allowed[reviewer] = struct{}{}
	}
	return allowed
}

func validateGraph(units []Unit) error {
	ids := map[string]struct{}{}
	for _, u := range units {
		if err := ValidateResourceID("unit id", u.ID); err != nil {
			return err
		}
		if _, ok := ids[u.ID]; ok {
			return fmt.Errorf("%w: duplicate unit id %q", ErrBadArgs, u.ID)
		}
		if err := validComplexity(u.Complexity); err != nil {
			return fmt.Errorf("%w (unit %q)", err, u.ID)
		}
		if err := validAcceptanceCriteria(u.ID, u.AcceptanceCriteria); err != nil {
			return err
		}
		ids[u.ID] = struct{}{}
	}
	// dependents[b] = units that list b as a blocker (edge b → u).
	dependents := make(map[string][]string, len(units))
	indeg := make(map[string]int, len(units))
	for _, u := range units {
		indeg[u.ID] = 0
	}
	for _, u := range units {
		for _, b := range u.Blockers {
			if _, ok := ids[b]; !ok {
				return fmt.Errorf("%w: unit %q blocker %q unknown", ErrBadArgs, u.ID, b)
			}
			if b == u.ID {
				return fmt.Errorf("%w: unit %q blocks itself", ErrBadArgs, u.ID)
			}
			dependents[b] = append(dependents[b], u.ID)
			indeg[u.ID]++
		}
	}
	q := make([]string, 0, len(units))
	for _, u := range units {
		if indeg[u.ID] == 0 {
			q = append(q, u.ID)
		}
	}
	seen := 0
	for len(q) > 0 {
		id := q[0]
		q = q[1:]
		seen++
		for _, next := range dependents[id] {
			indeg[next]--
			if indeg[next] == 0 {
				q = append(q, next)
			}
		}
	}
	if seen != len(units) {
		return fmt.Errorf("%w: unit graph contains a cycle", ErrBadArgs)
	}
	return nil
}

// Frontier lists pending units whose blockers all settled (delivered or
// done), so dependents can build while earlier units await signals.
func Frontier(units []Unit) []string {
	settled := map[string]bool{}
	for i := range units {
		if units[i].Settled() {
			settled[units[i].ID] = true
		}
	}
	frontier := make([]string, 0)
	for i := range units {
		u := &units[i]
		if u.Phase != PhasePending && u.Phase != "" {
			continue
		}
		ready := true
		for _, b := range u.Blockers {
			if !settled[b] {
				ready = false
				break
			}
		}
		if ready {
			frontier = append(frontier, u.ID)
		}
	}
	return frontier
}

func frontierUnit(units []Unit) (*Unit, error) {
	frontier := Frontier(units)
	if len(frontier) == 0 {
		return nil, nil
	}
	return findUnit(units, frontier[0]), nil
}

func findUnit(units []Unit, id string) *Unit {
	for i := range units {
		if units[i].ID == id {
			return &units[i]
		}
	}
	return nil
}

func cloneUnits(units []Unit) []Unit {
	out := make([]Unit, len(units))
	for i, u := range units {
		out[i] = u
		if u.Blockers != nil {
			out[i].Blockers = append([]string(nil), u.Blockers...)
		}
		if u.AcceptanceCriteria != nil {
			out[i].AcceptanceCriteria = append([]string(nil), u.AcceptanceCriteria...)
		}
	}
	return out
}

func reviewsSatisfied(required, completed []ReviewerIdentity) bool {
	if len(required) == 0 {
		return false
	}
	for _, reviewer := range required {
		if !slices.Contains(completed, reviewer) {
			return false
		}
	}
	return true
}

func canonicalEvidence(run Run, units []Unit, cmd Command, in ApplyInput) any {
	switch cmd {
	case CmdIntake:
		return IntakeEvidence{
			IntakeRevision:    run.IntakeRevision,
			DeliveryMode:      run.DeliveryMode,
			RequiredReviewers: append([]ReviewerIdentity(nil), run.RequiredReviewers...),
			RiskTier:          run.RiskTier,
			Budget:            run.Budget,
			SeriesBound:       run.SeriesBound,
			Units:             cloneUnits(units),
		}
	case CmdRelease:
		return ReleaseEvidence{IntakeRevision: in.IntakeRevision}
	case CmdVerify:
		return *in.Verify
	case CmdReview:
		return *in.Review
	case CmdDeliver:
		return *in.Deliver
	case CmdAsk:
		return QuestionEvidence{Question: in.Question}
	case CmdDecide:
		return DecisionEvidence{Answer: in.Decision.Answer}
	case CmdRetry:
		return RetryEvidence{Reason: in.RetryReason}
	case CmdBlock:
		return BlockEvidence{Reason: in.BlockReason}
	case CmdObserve:
		return *in.Observe
	default:
		return nil
	}
}
