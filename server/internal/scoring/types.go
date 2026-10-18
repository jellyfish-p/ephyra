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
	ID uuid.UUID
	// Letter is the contest problem letter shown on the scoreboard.
	Letter     string
	MaxScore   exact.Score
	RevisionID uuid.UUID
}

type Entrant struct {
	ID          uuid.UUID
	DisplayName string
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
	Verdict       domain.Verdict
	Score         exact.Score
	SubtaskScores []exact.Score
	SystemFault   bool
	CompileFailed bool
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
	ProblemID              uuid.UUID   `json:"problem_id"`
	Letter                 string      `json:"letter"`
	Score                  exact.Score `json:"score"`
	Solved                 bool        `json:"solved"`
	PenaltyMinutes         int64       `json:"penalty_minutes"`
	FailedAttemptsBeforeAC int         `json:"failed_attempts_before_ac"`
	EffectiveSubmissionID  uuid.UUID   `json:"effective_submission_id"`
	Pending                bool        `json:"pending"`
	ReceivedAt             time.Time   `json:"-"`
}

type Standing struct {
	EntrantID      uuid.UUID      `json:"entrant_id"`
	DisplayName    string         `json:"display_name"`
	Rank           int            `json:"rank"`
	Solved         int            `json:"solved"`
	TotalScore     exact.Score    `json:"total_score"`
	PenaltyMinutes int64          `json:"penalty_minutes"`
	Problems       []ProblemScore `json:"problems"`
	Unsettled      bool           `json:"unsettled"`
}
