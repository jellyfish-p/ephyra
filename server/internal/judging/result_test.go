package judging

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
	"github.com/jellyfish-p/ephyra/server/internal/problem"
	"github.com/jellyfish-p/ephyra/server/internal/scoring"
)

func sampleCfg() problem.NormalizedConfig {
	return problem.NormalizedConfig{
		Version: 1, Kind: domain.KindBatch,
		Checker:     problem.CheckerSpec{Kind: domain.CheckerToken},
		TimeLimitMS: 1000, MemoryLimitBytes: 64 << 20,
		Cases:    []problem.NormalizedCase{{ID: "1", InputName: "1.in", OutputName: "1.out"}},
		Subtasks: []problem.NormalizedSubtask{{ID: "s1", Score: exact.FromInt(100), CaseIDs: []string{"1"}}},
	}
}

func TestRejectInvalidatedAndMismatch(t *testing.T) {
	id := uuid.New()
	att := Attempt{ID: id, TaskID: uuid.New(), Generation: 1, NodeID: uuid.New(), RevisionID: uuid.New(), ProfileID: "p", State: domain.AttemptInvalidated}
	rep := Report{AttemptID: id, TaskID: att.TaskID, Generation: 1, NodeID: att.NodeID, RevisionID: att.RevisionID, ProfileID: "p"}
	if AcceptReport(att, sampleCfg(), rep).Action != AcceptReject {
		t.Fatal("invalidated")
	}
	att.State = domain.AttemptDispatched
	rep.ProfileID = "other"
	if AcceptReport(att, sampleCfg(), rep).Action != AcceptReject {
		t.Fatal("mismatch")
	}
}

func TestRejectInvalidatedAttempt(t *testing.T) {
	id := uuid.New()
	att := Attempt{ID: id, TaskID: uuid.New(), Generation: 2, NodeID: uuid.New(), RevisionID: uuid.New(), ProfileID: "p", State: domain.AttemptInvalidated}
	rep := Report{
		AttemptID: id, TaskID: att.TaskID, Generation: 2, NodeID: att.NodeID, RevisionID: att.RevisionID, ProfileID: "p",
		Verdict: domain.VerdictAccepted, Score: exact.FromInt(100),
		Cases:    []scoring.CaseOutcome{{CaseID: "1", Verdict: domain.VerdictAccepted}},
		Subtasks: []scoring.SubtaskOutcome{{ID: "s1", Score: exact.FromInt(100)}},
	}
	res := AcceptReport(att, sampleCfg(), rep)
	if res.Action != AcceptReject {
		t.Fatal("late result on an invalidated attempt must be rejected")
	}
}

func TestRejectGenerationMismatch(t *testing.T) {
	id := uuid.New()
	att := Attempt{ID: id, TaskID: uuid.New(), Generation: 2, NodeID: uuid.New(), RevisionID: uuid.New(), ProfileID: "p", State: domain.AttemptDispatched}
	rep := Report{
		AttemptID: id, TaskID: att.TaskID, Generation: 1, NodeID: att.NodeID, RevisionID: att.RevisionID, ProfileID: "p",
		Verdict: domain.VerdictAccepted, Score: exact.FromInt(100),
		Cases:    []scoring.CaseOutcome{{CaseID: "1", Verdict: domain.VerdictAccepted}},
		Subtasks: []scoring.SubtaskOutcome{{ID: "s1", Score: exact.FromInt(100)}},
	}
	if AcceptReport(att, sampleCfg(), rep).Action != AcceptReject {
		t.Fatal("stale generation must not apply")
	}
}

func TestSystemFaultNotAppliedAsWrongAnswer(t *testing.T) {
	id := uuid.New()
	att := Attempt{ID: id, TaskID: uuid.New(), Generation: 1, NodeID: uuid.New(), RevisionID: uuid.New(), ProfileID: "p", State: domain.AttemptDispatched}
	rep := Report{
		AttemptID: id, TaskID: att.TaskID, Generation: 1, NodeID: att.NodeID, RevisionID: att.RevisionID, ProfileID: "p",
		Verdict: domain.VerdictWrongAnswer, Score: exact.FromInt(0), SystemFault: true, FaultKind: "checker_crash",
		Cases:    []scoring.CaseOutcome{{CaseID: "1", Verdict: domain.VerdictWrongAnswer}},
		Subtasks: []scoring.SubtaskOutcome{{ID: "s1", Score: exact.FromInt(0), Verdict: domain.VerdictWrongAnswer}},
	}
	res := AcceptReport(att, sampleCfg(), rep)
	if res.Action != AcceptApply {
		t.Fatalf("system fault should apply as a fault, got %v %v", res.Action, res.Err)
	}
	if res.Computed.Verdict == domain.VerdictWrongAnswer {
		t.Fatal("system fault must not be scored as wrong answer")
	}
	if res.Computed.Verdict != "" {
		t.Fatalf("system fault must not produce a player verdict, got %s", res.Computed.Verdict)
	}
}

func TestIdempotentAndConflict(t *testing.T) {
	id := uuid.New()
	att := Attempt{
		ID: id, TaskID: uuid.New(), Generation: 1, NodeID: uuid.New(), RevisionID: uuid.New(), ProfileID: "p",
		State: domain.AttemptSucceeded, HasStored: true, StoredVerdict: domain.VerdictAccepted, StoredScore: exact.FromInt(100),
	}
	rep := Report{
		AttemptID: id, TaskID: att.TaskID, Generation: 1, NodeID: att.NodeID, RevisionID: att.RevisionID, ProfileID: "p",
		Verdict: domain.VerdictAccepted, Score: exact.FromInt(100),
		Cases:    []scoring.CaseOutcome{{CaseID: "1", Verdict: domain.VerdictAccepted}},
		Subtasks: []scoring.SubtaskOutcome{{ID: "s1", Score: exact.FromInt(100)}},
	}
	if AcceptReport(att, sampleCfg(), rep).Action != AcceptIdempotent {
		t.Fatal("idempotent")
	}
	rep.Score = exact.FromInt(0)
	rep.Verdict = domain.VerdictWrongAnswer
	if AcceptReport(att, sampleCfg(), rep).Action != AcceptReject {
		t.Fatal("conflict")
	}
}

func TestRejectsScoreMismatch(t *testing.T) {
	id := uuid.New()
	att := Attempt{ID: id, TaskID: uuid.New(), Generation: 1, NodeID: uuid.New(), RevisionID: uuid.New(), ProfileID: "p", State: domain.AttemptDispatched}
	rep := Report{
		AttemptID: id, TaskID: att.TaskID, Generation: 1, NodeID: att.NodeID, RevisionID: att.RevisionID, ProfileID: "p",
		Verdict: domain.VerdictAccepted, Score: exact.FromInt(100),
		Cases:    []scoring.CaseOutcome{{CaseID: "1", Verdict: domain.VerdictWrongAnswer}},
		Subtasks: []scoring.SubtaskOutcome{{ID: "s1", Score: exact.FromInt(100)}},
	}
	if AcceptReport(att, sampleCfg(), rep).Action != AcceptReject {
		t.Fatal("node cannot override computed WA")
	}
}
