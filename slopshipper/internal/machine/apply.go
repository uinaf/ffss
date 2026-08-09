package machine

import (
	"fmt"
	"slices"
	"strings"
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
	case CmdRetry:
		err = applyRetry(&run, units, in)
	case CmdBlock:
		err = applyBlock(&run, in)
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
	run.CompletedReviewers = nil
	run.State = StateBuild
	run.BlockerReason = ""
	run.DecisionQuestion = ""
	run.ReturnState = ""
	return nil
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
	if err := validateReviewEvidence(in.Review, run.ReviewConsent); err != nil {
		return err
	}
	switch in.Review.Verdict {
	case ReviewClean:
		if slices.Contains(run.CompletedReviewers, in.Review.Reviewer) {
			return fmt.Errorf("%w: reviewer %q already recorded", ErrUnmetGuard, in.Review.Reviewer)
		}
		run.CompletedReviewers = append(run.CompletedReviewers, in.Review.Reviewer)
		if reviewsSatisfied(run.ReviewConsent, run.CompletedReviewers) {
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
	u.Done = true
	run.CompletedUnits++
	run.CurrentUnitID = ""
	run.CompletedReviewers = nil

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
	if in.Question == "" {
		return fmt.Errorf("%w: ask requires --question", ErrBadArgs)
	}
	run.ReturnState = run.State
	run.State = StateNeedsDecision
	run.DecisionQuestion = in.Question
	return nil
}

func applyDecide(run *Run, in ApplyInput) error {
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

func applyRetry(run *Run, units []Unit, in ApplyInput) error {
	if strings.TrimSpace(in.RetryReason) == "" {
		return fmt.Errorf("%w: retry reason required", ErrBadArgs)
	}
	if run.CurrentUnitID == "" {
		return fmt.Errorf("%w: blocked run has no current unit", ErrCorruptState)
	}
	unit := findUnit(units, run.CurrentUnitID)
	if unit == nil || unit.Done {
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
	switch run.State {
	case StateIntake:
		out := []Command{CmdIntake, CmdAsk}
		if run.Released() {
			if u, _ := frontierUnit(units); u != nil && run.CompletedUnits < run.SeriesBound {
				out = append(out, CmdBuild)
			}
		} else if len(units) > 0 {
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
		return []Command{CmdDecide}
	case StateBlocked:
		return []Command{CmdRetry}
	case StateRunDone:
		return nil
	default:
		return nil
	}
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

func Frontier(units []Unit) []string {
	done := map[string]bool{}
	for i := range units {
		if units[i].Done {
			done[units[i].ID] = true
		}
	}
	frontier := make([]string, 0)
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
	}
	return out
}

func reviewsSatisfied(consent ReviewConsent, completed []ReviewerIdentity) bool {
	switch consent {
	case ReviewAutoreview:
		return slices.Contains(completed, ReviewerAutoreview)
	case ReviewBugbot:
		return slices.Contains(completed, ReviewerBugbot)
	case ReviewHuman:
		return slices.Contains(completed, ReviewerHuman)
	case ReviewBoth:
		return slices.Contains(completed, ReviewerAutoreview) && slices.Contains(completed, ReviewerBugbot)
	default:
		return false
	}
}

func canonicalEvidence(run Run, units []Unit, cmd Command, in ApplyInput) any {
	switch cmd {
	case CmdIntake:
		return IntakeEvidence{
			IntakeRevision: run.IntakeRevision,
			DeliveryMode:   run.DeliveryMode,
			ReviewConsent:  run.ReviewConsent,
			SeriesBound:    run.SeriesBound,
			Units:          cloneUnits(units),
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
	default:
		return nil
	}
}
