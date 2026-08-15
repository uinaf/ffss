package store_test

import (
	"path/filepath"
	"testing"

	"github.com/uinaf/slopshipper/internal/machine"
	"github.com/uinaf/slopshipper/internal/store"
)

func TestLatestDeliveriesKeepsNewestPerUnit(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	run := machine.NewRun("run", "repo")
	units := []machine.Unit{{ID: "u1", Title: "one"}, {ID: "u2", Title: "two", Blockers: []string{"u1"}}}
	if err := s.CreateRun(run, nil, nil); err != nil {
		t.Fatal(err)
	}
	apply := func(cmd machine.Command, in machine.ApplyInput) {
		t.Helper()
		current, currentUnits, err := s.GetRun("run")
		if err != nil {
			t.Fatal(err)
		}
		in.ExpectedRevision = current.Revision
		res, err := machine.Apply(current, currentUnits, cmd, in)
		if err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
		if err := s.SaveApply(res); err != nil {
			t.Fatalf("save %s: %v", cmd, err)
		}
	}
	bound := 2
	apply(machine.CmdIntake, machine.ApplyInput{Intake: &machine.IntakePatch{Units: units, SeriesBound: &bound}})
	apply(machine.CmdRelease, machine.ApplyInput{IntakeRevision: 2})
	deliverUnit := func(prURL, sha string) {
		apply(machine.CmdBuild, machine.ApplyInput{})
		apply(machine.CmdVerify, machine.ApplyInput{Verify: &machine.VerifyEvidence{Command: "true", ExitCode: 0}})
		apply(machine.CmdReview, machine.ApplyInput{Review: &machine.ReviewEvidence{
			Reviewer: machine.ReviewerSlopguard, Verdict: machine.ReviewClean, ArtifactRef: "test://r",
		}})
		apply(machine.CmdDeliver, machine.ApplyInput{Deliver: &machine.DeliverEvidence{
			DeliveryMode: machine.DeliveryPRHold, PRURL: prURL, CommitSHA: sha,
		}})
	}
	deliverUnit("https://github.com/o/r/pull/1", "beef00feed11")

	// u1 comes back for rework and re-delivers; the newest evidence wins.
	apply(machine.CmdObserve, machine.ApplyInput{Observe: &machine.ObserveEvidence{
		UnitID: "u1", Signal: machine.SignalChecksFailed,
	}})
	deliverUnit("https://github.com/o/r/pull/1", "beef11feed22")

	deliveries, err := s.LatestDeliveries("run")
	if err != nil {
		t.Fatal(err)
	}
	if len(deliveries) != 1 || deliveries["u1"].Evidence.CommitSHA != "beef11feed22" {
		t.Fatalf("newest delivery evidence must win: %+v", deliveries)
	}
	if deliveries["u1"].Evidence.PRURL != "https://github.com/o/r/pull/1" || deliveries["u1"].Seq == 0 {
		t.Fatalf("%+v", deliveries)
	}

	// The newest observe reference per unit and signal backs watch's
	// new-feedback baseline; other units and signals never match.
	apply(machine.CmdObserve, machine.ApplyInput{Observe: &machine.ObserveEvidence{
		UnitID: "u1", Signal: machine.SignalReviewFeedback, Reference: "1 unresolved review thread(s)",
		ThreadTokens: []string{"t1@c1"},
	}})
	last, found, err := s.LastObservation("run", "u1", machine.SignalReviewFeedback)
	if err != nil || !found || last.Reference != "1 unresolved review thread(s)" || len(last.ThreadTokens) != 1 {
		t.Fatalf("last=%+v found=%v err=%v", last, found, err)
	}
	if _, found, err := s.LastObservation("run", "u1", machine.SignalHeadMoved); err != nil || found {
		t.Fatalf("other signals must not match the feedback baseline: %v %v", found, err)
	}
	if _, found, _ := s.LastObservation("run", "u2", machine.SignalReviewFeedback); found {
		t.Fatal("other units must not match")
	}
}
