package scoring

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
)

func TestOILastSubmissionWins(t *testing.T) {
	e, p := uuid.New(), uuid.New()
	rev := uuid.New()
	start := time.Unix(0, 0).UTC()
	in := ContestInput{
		Rule: domain.RuleOI, Start: start,
		Problems: []Problem{{ID: p, RevisionID: rev, MaxScore: exact.FromInt(100)}},
		Entrants: []Entrant{{ID: e}},
		Submissions: []Submission{
			scored(e, p, rev, 1, start.Add(time.Minute), 30),
			scored(e, p, rev, 2, start.Add(2*time.Minute), 80),
			scored(e, p, rev, 3, start.Add(3*time.Minute), 60),
		},
	}
	got := ScoreContest(in)
	if got[0].TotalScore.String() != "60.00" {
		t.Fatalf("got %s", got[0].TotalScore)
	}
}

func TestOILastCEDoesNotFallBack(t *testing.T) {
	e, p, rev := uuid.New(), uuid.New(), uuid.New()
	start := time.Unix(0, 0).UTC()
	in := ContestInput{
		Rule: domain.RuleOI, Start: start,
		Problems: []Problem{{ID: p, RevisionID: rev}},
		Entrants: []Entrant{{ID: e}},
		Submissions: []Submission{
			scored(e, p, rev, 1, start.Add(time.Minute), 80),
			{
				ID: uuid.New(), EntrantID: e, ProblemID: p, RevisionID: rev,
				ReceiveSeq: 2, ReceivedAt: start.Add(2 * time.Minute),
				Result: &Result{Verdict: domain.VerdictCompilationError, CompileFailed: true},
			},
		},
	}
	got := ScoreContest(in)
	if got[0].TotalScore.String() != "0.00" {
		t.Fatalf("got %s want 0 (CE), not previous 80", got[0].TotalScore)
	}
	if got[0].Problems[0].EffectiveSubmissionID != in.Submissions[1].ID {
		t.Fatal("should use last CE submission")
	}
}

func TestOILastSystemFaultUnsettled(t *testing.T) {
	e, p, rev := uuid.New(), uuid.New(), uuid.New()
	start := time.Unix(0, 0).UTC()
	in := ContestInput{
		Rule: domain.RuleOI, Start: start,
		Problems: []Problem{{ID: p, RevisionID: rev}},
		Entrants: []Entrant{{ID: e}},
		Submissions: []Submission{
			scored(e, p, rev, 1, start.Add(time.Minute), 80),
			{
				ID: uuid.New(), EntrantID: e, ProblemID: p, RevisionID: rev,
				ReceiveSeq: 2, ReceivedAt: start.Add(2 * time.Minute),
				Result: &Result{Verdict: domain.VerdictSystemError, SystemFault: true},
			},
		},
	}
	got := ScoreContest(in)
	if !got[0].Unsettled || !got[0].Problems[0].Pending {
		t.Fatal("system fault must not settle as CE or 0")
	}
	if got[0].TotalScore.String() != "0.00" {
		t.Fatalf("must not fall back to 80, got %s", got[0].TotalScore)
	}
}

func TestIOIWholeSubmissionMax(t *testing.T) {
	e, p, rev := uuid.New(), uuid.New(), uuid.New()
	start := time.Unix(0, 0).UTC()
	in := ContestInput{
		Rule: domain.RuleIOI, Start: start,
		Problems: []Problem{{ID: p, RevisionID: rev}},
		Entrants: []Entrant{{ID: e}},
		Submissions: []Submission{
			{
				ID: uuid.New(), EntrantID: e, ProblemID: p, RevisionID: rev,
				ReceiveSeq: 1, ReceivedAt: start.Add(time.Minute),
				Result: &Result{Verdict: domain.VerdictWrongAnswer, Score: exact.FromInt(50), SubtaskScores: []exact.Score{exact.FromInt(50), exact.Zero()}},
			},
			{
				ID: uuid.New(), EntrantID: e, ProblemID: p, RevisionID: rev,
				ReceiveSeq: 2, ReceivedAt: start.Add(2 * time.Minute),
				Result: &Result{Verdict: domain.VerdictWrongAnswer, Score: exact.FromInt(50), SubtaskScores: []exact.Score{exact.Zero(), exact.FromInt(50)}},
			},
		},
	}
	got := ScoreContest(in)
	if got[0].TotalScore.String() != "50.00" {
		t.Fatalf("IOI whole-submission max must be 50 not 100, got %s", got[0].TotalScore)
	}
}

func TestACMPenaltyRoundingPerProblem(t *testing.T) {
	e := uuid.New()
	p1, p2, r1, r2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	start := time.Unix(1_000_000, 0).UTC()
	in := ContestInput{
		Rule: domain.RuleACM, PenaltyMinutes: 20, Start: start,
		Problems: []Problem{{ID: p1, RevisionID: r1}, {ID: p2, RevisionID: r2}},
		Entrants: []Entrant{{ID: e}},
		Submissions: []Submission{
			ac(e, p1, r1, 1, start.Add(89*time.Second)),
			ac(e, p2, r2, 2, start.Add(89*time.Second)),
		},
	}
	got := ScoreContest(in)
	if got[0].Solved != 2 || got[0].PenaltyMinutes != 2 {
		t.Fatalf("two 1:29 ACs must sum to 2 minutes, got solved=%d penalty=%d", got[0].Solved, got[0].PenaltyMinutes)
	}
}

func TestACMPenaltyWithWAAndCE(t *testing.T) {
	e, p, rev := uuid.New(), uuid.New(), uuid.New()
	start := time.Unix(1_000_000, 0).UTC()
	in := ContestInput{
		Rule: domain.RuleACM, PenaltyMinutes: 20, Start: start,
		Problems: []Problem{{ID: p, RevisionID: rev}},
		Entrants: []Entrant{{ID: e}},
		Submissions: []Submission{
			verdict(e, p, rev, 1, start.Add(10*time.Second), domain.VerdictWrongAnswer),
			verdict(e, p, rev, 2, start.Add(20*time.Second), domain.VerdictCompilationError),
			ac(e, p, rev, 3, start.Add(90*time.Second)),
		},
	}
	got := ScoreContest(in)
	// 1:30 -> 2 minutes + 1 WA * 20; CE ignored
	if got[0].PenaltyMinutes != 22 {
		t.Fatalf("got penalty %d want 22", got[0].PenaltyMinutes)
	}
	if got[0].Problems[0].FailedAttemptsBeforeAC != 1 {
		t.Fatalf("fails=%d", got[0].Problems[0].FailedAttemptsBeforeAC)
	}
}

func TestReceiveOrderNotCompletionOrder(t *testing.T) {
	e, p, rev := uuid.New(), uuid.New(), uuid.New()
	start := time.Unix(0, 0).UTC()
	first := ac(e, p, rev, 1, start.Add(time.Minute))
	second := ac(e, p, rev, 2, start.Add(2*time.Minute))
	in := ContestInput{
		Rule: domain.RuleACM, PenaltyMinutes: 20, Start: start,
		Problems:    []Problem{{ID: p, RevisionID: rev}},
		Entrants:    []Entrant{{ID: e}},
		Submissions: []Submission{second, first},
	}
	got := ScoreContest(in)
	if got[0].Problems[0].EffectiveSubmissionID != first.ID {
		t.Fatal("first AC must follow receive seq, not slice order")
	}
}

func TestCompetitionRank1224(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	p, rev := uuid.New(), uuid.New()
	start := time.Unix(0, 0).UTC()
	in := ContestInput{
		Rule: domain.RuleOI, Start: start,
		Problems: []Problem{{ID: p, RevisionID: rev}},
		Entrants: []Entrant{{ID: ids[0]}, {ID: ids[1]}, {ID: ids[2]}, {ID: ids[3]}},
		Submissions: []Submission{
			scored(ids[0], p, rev, 1, start.Add(time.Minute), 100),
			scored(ids[1], p, rev, 2, start.Add(time.Minute), 90),
			scored(ids[2], p, rev, 3, start.Add(time.Minute), 90),
			scored(ids[3], p, rev, 4, start.Add(time.Minute), 70),
		},
	}
	got := ScoreContest(in)
	ranks := []int{got[0].Rank, got[1].Rank, got[2].Rank, got[3].Rank}
	if ranks[0] != 1 || ranks[1] != 2 || ranks[2] != 2 || ranks[3] != 4 {
		t.Fatalf("ranks %v", ranks)
	}
}

func TestRoundHalfUpMinutes(t *testing.T) {
	if roundHalfUpMinutes(89_000) != 1 {
		t.Fatal("1:29")
	}
	if roundHalfUpMinutes(90_000) != 2 {
		t.Fatal("1:30")
	}
	if roundHalfUpMinutes(0) != 0 {
		t.Fatal("0")
	}
}

func scored(e, p, rev uuid.UUID, seq int64, at time.Time, score int64) Submission {
	return Submission{
		ID: uuid.New(), EntrantID: e, ProblemID: p, RevisionID: rev,
		ReceiveSeq: seq, ReceivedAt: at,
		Result: &Result{Verdict: domain.VerdictAccepted, Score: exact.FromInt(score)},
	}
}

func ac(e, p, rev uuid.UUID, seq int64, at time.Time) Submission {
	s := scored(e, p, rev, seq, at, 1)
	s.Result.Score = exact.FromInt(1)
	return s
}

func verdict(e, p, rev uuid.UUID, seq int64, at time.Time, v domain.Verdict) Submission {
	return Submission{
		ID: uuid.New(), EntrantID: e, ProblemID: p, RevisionID: rev,
		ReceiveSeq: seq, ReceivedAt: at,
		Result: &Result{Verdict: v},
	}
}
