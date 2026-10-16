package judgegrpc

import (
	"testing"

	"github.com/google/uuid"
	judgev1 "github.com/jellyfish-p/ephyra/protocol/judge/v1"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
)

func TestReportFromProto(t *testing.T) {
	attemptID, taskID, nodeID, revID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	rep, err := reportFromProto(&judgev1.AttemptResult{
		AttemptId: attemptID.String(), TaskId: taskID.String(), NodeId: nodeID.String(),
		JudgeDataRevisionId: revID.String(), ExecutionProfileId: "cpp23.gcc.linux-amd64.v0",
		Generation: 1, Verdict: string(domain.VerdictAccepted), Score: "100.00",
		Cases: []*judgev1.CaseResult{{CaseId: "1", Verdict: string(domain.VerdictAccepted)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.AttemptID != attemptID || rep.TaskID != taskID || rep.NodeID != nodeID {
		t.Fatalf("ids %+v", rep)
	}
	if rep.Score.Cmp(exact.FromInt(100)) != 0 {
		t.Fatalf("score %s", rep.Score)
	}
	if len(rep.Cases) != 1 || rep.Cases[0].Verdict != domain.VerdictAccepted {
		t.Fatalf("cases %+v", rep.Cases)
	}
}

func TestReportFromProtoRejectsBadAttemptID(t *testing.T) {
	_, err := reportFromProto(&judgev1.AttemptResult{AttemptId: "nope"})
	if err == nil {
		t.Fatal("expected error")
	}
}
