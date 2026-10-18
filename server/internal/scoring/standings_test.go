package scoring

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
)

// The frontend labels scoreboard rows and columns from these two fields, so a
// pure scoring run must carry them through.
func TestStandingsCarryEntrantNameAndProblemLetter(t *testing.T) {
	entrant := uuid.New()
	problemA, problemB := uuid.New(), uuid.New()
	revision := uuid.New()
	start := time.Unix(1_700_000_000, 0)

	out := ScoreContest(ContestInput{
		Rule: domain.RuleACM, PenaltyMinutes: 20, Start: start,
		Problems: []Problem{
			{ID: problemA, Letter: "A", RevisionID: revision},
			{ID: problemB, Letter: "B", RevisionID: revision},
		},
		Entrants: []Entrant{{ID: entrant, DisplayName: "Alice"}},
		Submissions: []Submission{{
			ID: uuid.New(), EntrantID: entrant, ProblemID: problemA,
			ReceiveSeq: 1, ReceivedAt: start.Add(90 * time.Second), RevisionID: revision,
			Result: &Result{Verdict: domain.VerdictAccepted, Score: exact.FromInt(1)},
		}},
	})

	if len(out) != 1 {
		t.Fatalf("standings %d rows", len(out))
	}
	row := out[0]
	if row.DisplayName != "Alice" {
		t.Fatalf("display name %q", row.DisplayName)
	}
	if len(row.Problems) != 2 {
		t.Fatalf("problems %d", len(row.Problems))
	}
	if row.Problems[0].Letter != "A" || row.Problems[1].Letter != "B" {
		t.Fatalf("letters %q %q", row.Problems[0].Letter, row.Problems[1].Letter)
	}
	// 1:30 rounds up to 2 minutes, which is the documented per-problem rule.
	if row.Problems[0].PenaltyMinutes != 2 {
		t.Fatalf("penalty %d", row.Problems[0].PenaltyMinutes)
	}
	if row.Problems[1].Letter != "B" || row.Problems[1].Solved {
		t.Fatalf("unsolved problem B: %+v", row.Problems[1])
	}
}

// The HTTP contract the frontend consumes is snake_case with decimal-string scores.
func TestStandingJSONUsesFrontendContract(t *testing.T) {
	row := Standing{
		EntrantID: uuid.New(), DisplayName: "Alice", Rank: 1, Solved: 1,
		TotalScore: exact.FromInt(50), PenaltyMinutes: 7,
		Problems: []ProblemScore{{
			ProblemID: uuid.New(), Letter: "A", Score: exact.FromInt(50), Solved: true,
			PenaltyMinutes: 7, FailedAttemptsBeforeAC: 2,
			EffectiveSubmissionID: uuid.New(),
		}},
	}
	body, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{
		"entrant_id", "display_name", "rank", "solved", "total_score",
		"penalty_minutes", "problems", "unsettled",
	} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing %q in %s", key, body)
		}
	}
	if decoded["total_score"] != "50.00" {
		t.Fatalf("score must be a decimal string, got %#v", decoded["total_score"])
	}
	problems, ok := decoded["problems"].([]any)
	if !ok || len(problems) != 1 {
		t.Fatalf("problems %#v", decoded["problems"])
	}
	first, ok := problems[0].(map[string]any)
	if !ok {
		t.Fatalf("problem entry %#v", problems[0])
	}
	for _, key := range []string{
		"problem_id", "letter", "score", "solved", "penalty_minutes",
		"failed_attempts_before_ac", "effective_submission_id", "pending",
	} {
		if _, ok := first[key]; !ok {
			t.Fatalf("missing problem key %q in %s", key, body)
		}
	}
	if _, leaked := first["ReceivedAt"]; leaked {
		t.Fatalf("internal ReceivedAt must not be serialized: %s", body)
	}
}
