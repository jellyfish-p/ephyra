package judging

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
	"github.com/jellyfish-p/ephyra/server/internal/problem"
	"github.com/jellyfish-p/ephyra/server/internal/scoring"
)

type Attempt struct {
	ID            uuid.UUID
	TaskID        uuid.UUID
	Generation    int32
	NodeID        uuid.UUID
	RevisionID    uuid.UUID
	ProfileID     string
	State         domain.AttemptState
	StoredVerdict domain.Verdict
	StoredScore   exact.Score
	StoredFault   bool
	HasStored     bool
}

type Report struct {
	AttemptID      uuid.UUID
	TaskID         uuid.UUID
	Generation     int32
	NodeID         uuid.UUID
	RevisionID     uuid.UUID
	ProfileID      string
	Verdict        domain.Verdict
	Score          exact.Score
	CompilerFailed bool
	SystemFault    bool
	FaultKind      string
	Cases          []scoring.CaseOutcome
	Subtasks       []scoring.SubtaskOutcome
}

type AcceptAction int

const (
	AcceptApply AcceptAction = iota
	AcceptIdempotent
	AcceptReject
)

type AcceptResult struct {
	Action   AcceptAction
	Computed scoring.ComputedScore
	Err      error
}

func AcceptReport(att Attempt, cfg problem.NormalizedConfig, report Report) AcceptResult {
	if att.State == domain.AttemptInvalidated {
		return AcceptResult{Action: AcceptReject, Err: apperr.New(apperr.Gone, "attempt is no longer valid")}
	}
	if report.AttemptID != att.ID || report.TaskID != att.TaskID || report.Generation != att.Generation ||
		report.NodeID != att.NodeID || report.RevisionID != att.RevisionID || report.ProfileID != att.ProfileID {
		return AcceptResult{Action: AcceptReject, Err: apperr.New(apperr.Invalid, "result does not match the attempt")}
	}
	if att.State == domain.AttemptSucceeded && att.HasStored {
		if sameStored(att, report) {
			return AcceptResult{Action: AcceptIdempotent}
		}
		return AcceptResult{Action: AcceptReject, Err: apperr.New(apperr.Conflict, "conflicting duplicate result")}
	}
	if report.SystemFault {
		if report.FaultKind == "" {
			return AcceptResult{Action: AcceptReject, Err: apperr.New(apperr.Invalid, "system fault requires a kind")}
		}
		return AcceptResult{Action: AcceptApply}
	}
	computed, err := scoring.ComputeSubmissionScore(cfg, report.Cases, report.CompilerFailed)
	if err != nil {
		return AcceptResult{Action: AcceptReject, Err: apperr.New(apperr.Invalid, err.Error())}
	}
	if computed.Verdict != report.Verdict || computed.Total.Cmp(report.Score) != 0 {
		return AcceptResult{Action: AcceptReject, Err: apperr.New(apperr.Invalid, "summary contradicts computed score")}
	}
	if len(report.Subtasks) != len(computed.Subtasks) {
		return AcceptResult{Action: AcceptReject, Err: apperr.New(apperr.Invalid, "subtask summary mismatch")}
	}
	for i, st := range computed.Subtasks {
		if report.Subtasks[i].ID != st.ID || report.Subtasks[i].Score.Cmp(st.Score) != 0 {
			return AcceptResult{Action: AcceptReject, Err: apperr.New(apperr.Invalid, fmt.Sprintf("subtask %s mismatch", st.ID))}
		}
	}
	return AcceptResult{Action: AcceptApply, Computed: computed}
}

func sameStored(att Attempt, report Report) bool {
	if att.StoredFault != report.SystemFault {
		return false
	}
	if att.StoredFault {
		return true
	}
	return att.StoredVerdict == report.Verdict && att.StoredScore.Cmp(report.Score) == 0
}
