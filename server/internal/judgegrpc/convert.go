package judgegrpc

import (
	"github.com/google/uuid"
	judgev1 "github.com/jellyfish-p/ephyra/protocol/judge/v1"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
	"github.com/jellyfish-p/ephyra/server/internal/judging"
	"github.com/jellyfish-p/ephyra/server/internal/scoring"
)

func helloFromProto(req *judgev1.HandshakeRequest) judging.Hello {
	return judging.Hello{
		ProtocolVersion: req.GetProtocolVersion(),
		NodeID:          req.GetNodeId(),
		Credential:      req.GetCredential(),
		Profiles:        req.GetExecutionProfileIds(),
		MaxSlots:        req.GetMaxSlots(),
	}
}

func admissionToProto(adm judging.Admission) *judgev1.HandshakeResponse {
	return &judgev1.HandshakeResponse{
		Accepted:            adm.Accepted,
		RejectReason:        adm.RejectReason,
		ProtocolVersion:     adm.ProtocolVersion,
		HeartbeatIntervalMs: adm.HeartbeatIntervalMs,
		LeaseMs:             adm.LeaseMs,
	}
}

func reportFromProto(r *judgev1.AttemptResult) (judging.Report, error) {
	attemptID, err := uuid.Parse(r.GetAttemptId())
	if err != nil {
		return judging.Report{}, apperr.New(apperr.Invalid, "bad attempt id")
	}
	taskID, err := uuid.Parse(r.GetTaskId())
	if err != nil {
		return judging.Report{}, apperr.New(apperr.Invalid, "bad task id")
	}
	nodeID, err := uuid.Parse(r.GetNodeId())
	if err != nil {
		return judging.Report{}, apperr.New(apperr.Invalid, "bad node id")
	}
	revID, err := uuid.Parse(r.GetJudgeDataRevisionId())
	if err != nil {
		return judging.Report{}, apperr.New(apperr.Invalid, "bad revision id")
	}
	verdict, ok := domain.ParseVerdict(r.GetVerdict())
	if !ok && !r.GetSystemFault() {
		return judging.Report{}, apperr.New(apperr.Invalid, "bad verdict")
	}
	score := exact.Zero()
	if r.GetScore() != "" {
		score, err = exact.Parse(r.GetScore())
		if err != nil {
			return judging.Report{}, apperr.New(apperr.Invalid, err.Error())
		}
	}
	return judging.Report{
		AttemptID: attemptID, TaskID: taskID, Generation: r.GetGeneration(), NodeID: nodeID,
		RevisionID: revID, ProfileID: r.GetExecutionProfileId(), Verdict: verdict, Score: score,
		CompilerFailed: r.GetCompilerFailed(), SystemFault: r.GetSystemFault(), FaultKind: r.GetSystemFaultKind(),
		Cases: toCaseOutcomes(r.GetCases()), Subtasks: toSubOutcomes(r.GetSubtasks()),
	}, nil
}

func toCaseOutcomes(in []*judgev1.CaseResult) []scoring.CaseOutcome {
	out := make([]scoring.CaseOutcome, 0, len(in))
	for _, c := range in {
		v, _ := domain.ParseVerdict(c.GetVerdict())
		out = append(out, scoring.CaseOutcome{CaseID: c.GetCaseId(), Verdict: v})
	}
	return out
}

func toSubOutcomes(in []*judgev1.SubtaskResult) []scoring.SubtaskOutcome {
	out := make([]scoring.SubtaskOutcome, 0, len(in))
	for _, s := range in {
		sc, _ := exact.Parse(s.GetScore())
		v, _ := domain.ParseVerdict(s.GetVerdict())
		out = append(out, scoring.SubtaskOutcome{ID: s.GetSubtaskId(), Score: sc, Verdict: v})
	}
	return out
}
