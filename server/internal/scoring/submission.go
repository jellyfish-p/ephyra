package scoring

import (
	"fmt"

	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
	"github.com/jellyfish-p/ephyra/server/internal/problem"
)

type CaseOutcome struct {
	CaseID  string
	Verdict domain.Verdict
}

type SubtaskOutcome struct {
	ID      string
	Score   exact.Score
	Verdict domain.Verdict
}

type ComputedScore struct {
	Total    exact.Score
	Verdict  domain.Verdict
	Subtasks []SubtaskOutcome
}

// ComputeSubmissionScore applies all-or-nothing subtasks and sums them.
// The platform, not the node, is the authority for this aggregation.
func ComputeSubmissionScore(cfg problem.NormalizedConfig, cases []CaseOutcome, compilerFailed bool) (ComputedScore, error) {
	if compilerFailed {
		subs := make([]SubtaskOutcome, 0, len(cfg.Subtasks))
		for _, st := range cfg.Subtasks {
			subs = append(subs, SubtaskOutcome{ID: st.ID, Score: exact.Zero(), Verdict: domain.VerdictCompilationError})
		}
		return ComputedScore{Total: exact.Zero(), Verdict: domain.VerdictCompilationError, Subtasks: subs}, nil
	}
	byID := make(map[string]CaseOutcome, len(cases))
	for _, c := range cases {
		if _, ok := byID[c.CaseID]; ok {
			return ComputedScore{}, fmt.Errorf("duplicate case %s", c.CaseID)
		}
		if !c.Verdict.IsNormalTerminal() {
			return ComputedScore{}, fmt.Errorf("case %s is not a normal terminal verdict", c.CaseID)
		}
		byID[c.CaseID] = c
	}
	if len(byID) != len(cfg.Cases) {
		return ComputedScore{}, fmt.Errorf("result has %d cases, config has %d", len(byID), len(cfg.Cases))
	}
	for _, c := range cfg.Cases {
		if _, ok := byID[c.ID]; !ok {
			return ComputedScore{}, fmt.Errorf("missing case %s", c.ID)
		}
	}

	var total exact.Score
	subs := make([]SubtaskOutcome, 0, len(cfg.Subtasks))
	allAC := true
	firstFail := domain.VerdictAccepted
	for _, st := range cfg.Subtasks {
		ok := true
		stFail := domain.VerdictAccepted
		for _, id := range st.CaseIDs {
			cr := byID[id]
			if cr.Verdict != domain.VerdictAccepted {
				ok = false
				allAC = false
				if stFail == domain.VerdictAccepted {
					stFail = cr.Verdict
				}
				if firstFail == domain.VerdictAccepted {
					firstFail = cr.Verdict
				}
			}
		}
		sc := exact.Zero()
		ver := stFail
		if ok {
			sc = st.Score
			ver = domain.VerdictAccepted
		}
		total = total.Add(sc)
		subs = append(subs, SubtaskOutcome{ID: st.ID, Score: sc, Verdict: ver})
	}
	v := domain.VerdictAccepted
	if !allAC {
		v = firstFail
	}
	return ComputedScore{Total: total, Verdict: v, Subtasks: subs}, nil
}
