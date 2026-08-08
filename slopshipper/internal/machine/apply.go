package machine

import (
	"fmt"
	"slices"
)

// NewRun constructs an INTAKE run after init.
func NewRun(id, repoKey string) Run {
	return Run{
		ID:             id,
		RepoKey:        repoKey,
		State:          StateIntake,
		IntakeRevision: 1,
		Revision:       1,
		DeliveryMode:   DeliveryPRHold,
		ReviewConsent:  ReviewAutoreview,
		SeriesBound:    1,
	}
}

// Apply mutates a deep copy of run/units according to cmd.
func Apply(run Run, units []Unit, cmd Command, in ApplyInput) (ApplyResult, error) {
	if in.ExpectedRevision != 0 && in.ExpectedRevision != run.Revision {
		return ApplyResult{}, fmt.Errorf("%w: expected %d got %d", ErrRevisionConflict, in.ExpectedRevision, run.Revision)
	}

	from := run.State
	units = cloneUnits(units)

	var err error
	switch cmd {
	case CmdIntake:
		err = applyIntake(&run, &units, in)
	case CmdRelease:
		err = applyRelease(&run, in)
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
		err = applyDecide(&run, in)
	case CmdBlock:
		err = applyBlock(&run, in)
	default:
		return ApplyResult{}, fmt.Errorf("%w: unknown command %q", ErrBadArgs, cmd)
	}
	if err != nil {
		return ApplyResult{}, err
	}

	run.Revision++
	allowed := AllowedCommands(run, units)
	return ApplyResult{
		Run:              run,
		Units:            units,
		EventFrom:        from,
		EventTo:          run.State,
		Command:          cmd,
		AllowedCommands:  allowed,
		NextAction:       nextActionCommand(allowed),
		RequiredEvidence: requiredEvidence(run.State, allowed),
	}, nil
}

func applyIntake(run *Run, units *[]Unit, in ApplyInput) error {
	if run.State != StateIntake && run.State != StateNeedsDecision {
		return fmt.Errorf("%w: intake only from INTAKE or NEEDS_DECISION", ErrIllegalTransition)
	}
	if run.State == StateNeedsDecision && run.ReturnState != StateIntake && run.ReturnState != "" {
		return fmt.Errorf("%w: intake while decision expects return to %s", ErrIllegalTransition, run.ReturnState)
	}
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
	if patch.ReviewConsent != nil {
		if err := validConsent(*patch.ReviewConsent); err != nil {
			return err
		}
		run.ReviewConsent = *patch.ReviewConsent
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
			next[i].Done = false
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

func applyRelease(run *Run, in ApplyInput) error {
	if run.State != StateIntake {
		return fmt.Errorf("%w: release only from INTAKE", ErrIllegalTransition)
	}
	if in.IntakeRevision == 0 {
		return fmt.Errorf("%w: release requires --revision", ErrBadArgs)
	}
	if in.IntakeRevision != run.IntakeRevision {
		return fmt.Errorf("%w: release revision %d does not match intake_revision %d", ErrUnmetGuard, in.IntakeRevision, run.IntakeRevision)
	}
	rev := run.IntakeRevision
	run.ReleasedRevision = &rev
	// RELEASED is a latch; resting display stays INTAKE until build claims a unit.
	// Status reports released=true; build is the next action.
	return nil
}

func applyBuild(run *Run, units *[]Unit) error {
	if !run.Released() {
		return fmt.Errorf("%w: release required before build", ErrUnmetGuard)
	}
	switch run.State {
	case StateIntake, StateRework:
	case StateNeedsDecision:
		if run.ReturnState != StateBuild && run.ReturnState != StateIntake && run.ReturnState != "" {
			return fmt.Errorf("%w: cannot build from NEEDS_DECISION returning to %s", ErrIllegalTransition, run.ReturnState)
		}
	default:
		return fmt.Errorf("%w: build not allowed from %s", ErrIllegalTransition, run.State)
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
		run.State = StateBuild
		run.DecisionQuestion = ""
		run.ReturnState = ""
		return nil
	}

	next, err := frontierUnit(*units)
	if err != nil {
		return err
	}
	if next == nil {
		return fmt.Errorf("%w: no frontier unit to build", ErrUnmetGuard)
	}
	if run.CompletedUnits >= run.SeriesBound {
		return fmt.Errorf("%w: series_bound %d reached", ErrUnmetGuard, run.SeriesBound)
	}
	run.CurrentUnitID = next.ID
	next.Attempt++
	run.State = StateBuild
	run.BlockerReason = ""
	run.DecisionQuestion = ""
	run.ReturnState = ""
	return nil
}

func applyVerify(run *Run, in ApplyInput) error {
	if run.State != StateBuild && run.State != StateVerify {
		return fmt.Errorf("%w: verify only from BUILD or VERIFY", ErrIllegalTransition)
	}
	if run.CurrentUnitID == "" {
		return fmt.Errorf("%w: no current unit", ErrUnmetGuard)
	}
	// Failure path → BLOCKED
	if in.Verify != nil && in.Verify.Command != "" && in.Verify.ExitCode != 0 {
		run.State = StateBlocked
		run.BlockerReason = fmt.Sprintf("verify failed: %s exit %d", in.Verify.Command, in.Verify.ExitCode)
		return nil
	}
	if err := validateVerifyEvidence(in.Verify); err != nil {
		return err
	}
	run.State = StateReview
	run.BlockerReason = ""
	return nil
}

func applyReview(run *Run, in ApplyInput) error {
	if run.State != StateReview {
		return fmt.Errorf("%w: review only from REVIEW", ErrIllegalTransition)
	}
	if err := validateReviewEvidence(in.Review, run.ReviewConsent); err != nil {
		return err
	}
	run.State = StateDeliver
	return nil
}

func applyRework(run *Run) error {
	if run.State != StateReview {
		return fmt.Errorf("%w: rework only from REVIEW", ErrIllegalTransition)
	}
	run.State = StateRework
	return nil
}

func applyDeliver(run *Run, units *[]Unit, in ApplyInput) error {
	if run.State != StateDeliver {
		return fmt.Errorf("%w: deliver only from DELIVER", ErrIllegalTransition)
	}
	if err := validateDeliverEvidence(in.Deliver, run.DeliveryMode); err != nil {
		return err
	}
	u := findUnit(*units, run.CurrentUnitID)
	if u == nil {
		return fmt.Errorf("%w: current unit missing", ErrCorruptState)
	}
	u.Done = true
	run.CompletedUnits++
	run.CurrentUnitID = ""

	if run.CompletedUnits >= run.SeriesBound {
		run.State = StateRunDone
		return nil
	}
	next, err := frontierUnit(*units)
	if err != nil {
		return err
	}
	if next == nil {
		run.State = StateRunDone
		return nil
	}
	// Remaining frontier: park on released INTAKE so next_action is build.
	run.State = StateIntake
	return nil
}

func applyAsk(run *Run, in ApplyInput) error {
	switch run.State {
	case StateBlocked, StateRunDone, StateNeedsDecision:
		return fmt.Errorf("%w: ask not allowed from %s", ErrIllegalTransition, run.State)
	}
	if in.Question == "" {
		return fmt.Errorf("%w: ask requires --question", ErrBadArgs)
	}
	run.ReturnState = run.State
	run.State = StateNeedsDecision
	run.DecisionQuestion = in.Question
	return nil
}

func applyDecide(run *Run, in ApplyInput) error {
	if run.State != StateNeedsDecision {
		return fmt.Errorf("%w: decide only from NEEDS_DECISION", ErrIllegalTransition)
	}
	if in.Decision == nil || in.Decision.Answer == "" {
		return fmt.Errorf("%w: decision.answer required", ErrBadArgs)
	}
	ret := run.ReturnState
	if ret == "" {
		ret = StateIntake
	}
	run.State = ret
	run.DecisionQuestion = ""
	run.ReturnState = ""
	return nil
}

func applyBlock(run *Run, in ApplyInput) error {
	if run.State == StateRunDone || run.State == StateBlocked {
		return fmt.Errorf("%w: cannot block from %s", ErrIllegalTransition, run.State)
	}
	if in.BlockReason == "" {
		return fmt.Errorf("%w: block reason required", ErrBadArgs)
	}
	run.State = StateBlocked
	run.BlockerReason = in.BlockReason
	return nil
}

// NeedsDecision parks the run for a human question.
func NeedsDecision(run Run, units []Unit, question string, returnState State) (ApplyResult, error) {
	if question == "" {
		return ApplyResult{}, fmt.Errorf("%w: question required", ErrBadArgs)
	}
	from := run.State
	run.ReturnState = returnState
	if run.ReturnState == "" {
		run.ReturnState = from
	}
	run.State = StateNeedsDecision
	run.DecisionQuestion = question
	run.Revision++
	allowed := AllowedCommands(run, units)
	return ApplyResult{
		Run:             run,
		Units:           cloneUnits(units),
		EventFrom:       from,
		EventTo:         StateNeedsDecision,
		AllowedCommands: allowed,
		NextAction:      nextActionCommand(allowed),
	}, nil
}

func AllowedCommands(run Run, units []Unit) []Command {
	switch run.State {
	case StateIntake:
		out := []Command{CmdIntake, CmdAsk}
		if run.Released() {
			if u, _ := frontierUnit(units); u != nil && run.CompletedUnits < run.SeriesBound {
				out = append(out, CmdBuild)
			}
		} else {
			out = append(out, CmdRelease)
		}
		return out
	case StateBuild:
		return []Command{CmdVerify, CmdAsk, CmdBlock}
	case StateVerify:
		return []Command{CmdVerify, CmdAsk, CmdBlock}
	case StateReview:
		return []Command{CmdReview, CmdRework, CmdAsk, CmdBlock}
	case StateDeliver:
		return []Command{CmdDeliver, CmdAsk}
	case StateRework:
		return []Command{CmdBuild, CmdAsk}
	case StateNeedsDecision:
		return []Command{CmdDecide, CmdIntake}
	case StateBlocked, StateRunDone:
		return nil
	default:
		return nil
	}
}

func nextActionCommand(allowed []Command) string {
	if len(allowed) == 0 {
		return ""
	}
	// Prefer forward progress order.
	priority := []Command{CmdRelease, CmdBuild, CmdVerify, CmdReview, CmdDeliver, CmdRework, CmdDecide, CmdIntake}
	for _, p := range priority {
		if slices.Contains(allowed, p) {
			return "slopinator " + string(p)
		}
	}
	return "slopinator " + string(allowed[0])
}

func requiredEvidence(state State, allowed []Command) []string {
	if slices.Contains(allowed, CmdVerify) {
		return []string{"verify.command", "verify.exit_code"}
	}
	if slices.Contains(allowed, CmdReview) {
		return []string{"review.reviewer", "review.verdict", "review.artifact_ref"}
	}
	if slices.Contains(allowed, CmdDeliver) {
		return []string{"deliver.delivery_mode", "deliver.pr_url|deliver.commit_sha"}
	}
	_ = state
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

func validConsent(c ReviewConsent) error {
	switch c {
	case ReviewAutoreview, ReviewBugbot, ReviewBoth, ReviewHuman:
		return nil
	default:
		return fmt.Errorf("%w: invalid review_consent %q", ErrBadArgs, c)
	}
}

func validateGraph(units []Unit) error {
	ids := map[string]struct{}{}
	for _, u := range units {
		if u.ID == "" {
			return fmt.Errorf("%w: unit id required", ErrBadArgs)
		}
		if _, ok := ids[u.ID]; ok {
			return fmt.Errorf("%w: duplicate unit id %q", ErrBadArgs, u.ID)
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

func frontierUnit(units []Unit) (*Unit, error) {
	done := map[string]bool{}
	for i := range units {
		if units[i].Done {
			done[units[i].ID] = true
		}
	}
	var frontier []*Unit
	for i := range units {
		u := &units[i]
		if u.Done {
			continue
		}
		ready := true
		for _, b := range u.Blockers {
			if !done[b] {
				ready = false
				break
			}
		}
		if ready {
			frontier = append(frontier, u)
		}
	}
	if len(frontier) == 0 {
		return nil, nil
	}
	// Deterministic: first ready in slice order.
	return frontier[0], nil
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
	}
	return out
}
