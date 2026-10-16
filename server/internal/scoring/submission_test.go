package scoring

import (
	"testing"

	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
	"github.com/jellyfish-p/ephyra/server/internal/problem"
)

func TestSubtaskAllOrNothing(t *testing.T) {
	cfg := problem.NormalizedConfig{
		Version: 1, Kind: domain.KindBatch,
		Checker:          problem.CheckerSpec{Kind: domain.CheckerToken},
		TimeLimitMS:      1000,
		MemoryLimitBytes: 64 << 20,
		Cases: []problem.NormalizedCase{
			{ID: "1", InputName: "1.in", OutputName: "1.out"},
			{ID: "2", InputName: "2.in", OutputName: "2.out"},
			{ID: "3", InputName: "3.in", OutputName: "3.out"},
		},
		Subtasks: []problem.NormalizedSubtask{
			{ID: "s1", Score: exact.FromInt(40), CaseIDs: []string{"1", "2"}},
			{ID: "s2", Score: exact.FromInt(60), CaseIDs: []string{"3"}},
		},
	}
	got, err := ComputeSubmissionScore(cfg, []CaseOutcome{
		{CaseID: "1", Verdict: domain.VerdictAccepted},
		{CaseID: "2", Verdict: domain.VerdictWrongAnswer},
		{CaseID: "3", Verdict: domain.VerdictAccepted},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subtasks[0].Score.String() != "0.00" {
		t.Fatalf("partial subtask must be 0, got %s", got.Subtasks[0].Score)
	}
	if got.Subtasks[1].Score.String() != "60.00" || got.Total.String() != "60.00" {
		t.Fatalf("total %s", got.Total)
	}
}
