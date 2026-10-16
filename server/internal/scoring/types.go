package scoring

import (
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
)

type ContestInput struct {
	Rule           domain.Rule
	PenaltyMinutes int
	Start          time.Time
	Problems       []Problem
	Entrants       []Entrant
	Submissions    []Submission
}

type Problem struct {
	ID         uuid.UUID
	MaxScore   exact.Score
	RevisionID uuid.UUID
}

type Entrant struct {
	ID uuid.UUID
}

type Submission struct {
	ID             uuid.UUID
	EntrantID      uuid.UUID
	ProblemID      uuid.UUID
	ReceiveSeq     int64
	ReceivedAt     time.Time
	RevisionID     uuid.UUID
	Result         *Result
	OnThisRevision bool
}

type Result struct {
	Verdict        domain.Verdict
	Score          exact.Score
	SubtaskScores  []exact.Score
	SystemFault    bool
	CompileFailed  bool
}

func (s Submission) NormalOn(revision uuid.UUID) bool {
	if s.RevisionID != revision || s.Result == nil {
		return false
	}
	if s.Result.SystemFault {
		return false
	}
	return s.Result.Verdict.IsNormalTerminal()
}

type ProblemScore struct {
	ProblemID              uuid.UUID
	Score                  exact.Score
	Solved                 bool
	PenaltyMinutes         int64
	FailedAttemptsBeforeAC int
	EffectiveSubmissionID  uuid.UUID
	Pending                bool
}

type Standing struct {
	EntrantID      uuid.UUID
	Rank           int
	Solved         int
	TotalScore     exact.Score
	PenaltyMinutes int64
	Problems       []ProblemScore
	Unsettled      bool
}
