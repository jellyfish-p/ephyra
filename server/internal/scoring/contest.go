package scoring

import (
	"sort"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
)

// ScoreContest is a pure function of receive order, rule, and effective results.
func ScoreContest(in ContestInput) []Standing {
	subsByEntrant := map[uuid.UUID][]Submission{}
	for _, s := range in.Submissions {
		subsByEntrant[s.EntrantID] = append(subsByEntrant[s.EntrantID], s)
	}
	for id := range subsByEntrant {
		sort.Slice(subsByEntrant[id], func(i, j int) bool {
			a, b := subsByEntrant[id][i], subsByEntrant[id][j]
			if a.ReceiveSeq != b.ReceiveSeq {
				return a.ReceiveSeq < b.ReceiveSeq
			}
			return a.ID.String() < b.ID.String()
		})
	}

	out := make([]Standing, 0, len(in.Entrants))
	for _, e := range in.Entrants {
		st := Standing{
			EntrantID: e.ID, DisplayName: e.DisplayName,
			Problems: make([]ProblemScore, 0, len(in.Problems)),
		}
		for _, p := range in.Problems {
			ps := scoreProblem(in, p, filterProblem(subsByEntrant[e.ID], p.ID))
			st.Problems = append(st.Problems, ps)
			if ps.Pending {
				st.Unsettled = true
			}
			if ps.Pending && in.Rule == domain.RuleOI {
				continue
			}
			if in.Rule == domain.RuleACM {
				if ps.Solved {
					st.Solved++
					st.PenaltyMinutes += ps.PenaltyMinutes
				}
			} else {
				st.TotalScore = st.TotalScore.Add(ps.Score)
			}
		}
		out = append(out, st)
	}
	rank(in.Rule, out)
	return out
}

func filterProblem(subs []Submission, problemID uuid.UUID) []Submission {
	var out []Submission
	for _, s := range subs {
		if s.ProblemID == problemID {
			out = append(out, s)
		}
	}
	return out
}

func scoreProblem(in ContestInput, p Problem, subs []Submission) ProblemScore {
	ps := ProblemScore{ProblemID: p.ID, Letter: p.Letter}
	switch in.Rule {
	case domain.RuleACM:
		return scoreACM(in, p, subs)
	case domain.RuleOI:
		return scoreOI(p, subs)
	case domain.RuleIOI:
		return scoreIOI(p, subs)
	default:
		return ps
	}
}

func scoreACM(in ContestInput, p Problem, subs []Submission) ProblemScore {
	ps := ProblemScore{ProblemID: p.ID, Letter: p.Letter}
	fails := 0
	for _, s := range subs {
		if !s.NormalOn(p.RevisionID) {
			if s.Result != nil && s.Result.SystemFault && s.RevisionID == p.RevisionID {
				ps.Pending = true
			} else if s.Result == nil && s.RevisionID == p.RevisionID {
				ps.Pending = true
			}
			continue
		}
		if s.Result.Verdict == domain.VerdictAccepted {
			ps.Solved = true
			ps.Score = exact.FromInt(1)
			ps.EffectiveSubmissionID = s.ID
			ps.ReceivedAt = s.ReceivedAt
			ps.FailedAttemptsBeforeAC = fails
			elapsed := s.ReceivedAt.Sub(in.Start)
			if elapsed < 0 {
				elapsed = 0
			}
			mins := roundHalfUpMinutes(elapsed.Milliseconds())
			ps.PenaltyMinutes = mins + int64(fails*in.PenaltyMinutes)
			ps.Pending = false
			return ps
		}
		if s.Result.Verdict.CountsTowardACMPenalty() {
			fails++
		}
	}
	ps.FailedAttemptsBeforeAC = fails
	return ps
}

func scoreOI(p Problem, subs []Submission) ProblemScore {
	ps := ProblemScore{ProblemID: p.ID, Letter: p.Letter}
	if len(subs) == 0 {
		return ps
	}
	last := subs[len(subs)-1]
	if !last.NormalOn(p.RevisionID) {
		ps.Pending = true
		ps.EffectiveSubmissionID = last.ID
		return ps
	}
	ps.Score = last.Result.Score
	ps.EffectiveSubmissionID = last.ID
	ps.Solved = last.Result.Verdict == domain.VerdictAccepted
	return ps
}

func scoreIOI(p Problem, subs []Submission) ProblemScore {
	ps := ProblemScore{ProblemID: p.ID, Letter: p.Letter}
	var best *Submission
	pending := false
	for i := range subs {
		s := &subs[i]
		if s.RevisionID == p.RevisionID && (s.Result == nil || s.Result.SystemFault) {
			pending = true
			continue
		}
		if !s.NormalOn(p.RevisionID) {
			continue
		}
		if best == nil || s.Result.Score.Cmp(best.Result.Score) > 0 {
			best = s
		}
	}
	if best == nil {
		ps.Pending = pending
		return ps
	}
	ps.Score = best.Result.Score
	ps.EffectiveSubmissionID = best.ID
	ps.Solved = best.Result.Verdict == domain.VerdictAccepted
	ps.Pending = pending
	return ps
}

// roundHalfUpMinutes converts elapsed milliseconds to minutes, rounding .5 up.
// 1:29 (89000ms) -> 1; 1:30 (90000ms) -> 2.
func roundHalfUpMinutes(elapsedMS int64) int64 {
	if elapsedMS < 0 {
		return 0
	}
	return (elapsedMS + 30_000) / 60_000
}

func rank(rule domain.Rule, rows []Standing) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if rule == domain.RuleACM {
			if a.Solved != b.Solved {
				return a.Solved > b.Solved
			}
			if a.PenaltyMinutes != b.PenaltyMinutes {
				return a.PenaltyMinutes < b.PenaltyMinutes
			}
		} else if a.TotalScore.Cmp(b.TotalScore) != 0 {
			return a.TotalScore.Cmp(b.TotalScore) > 0
		}
		return a.EntrantID.String() < b.EntrantID.String()
	})
	for i := range rows {
		if i == 0 {
			rows[i].Rank = 1
			continue
		}
		prev := rows[i-1]
		same := false
		if rule == domain.RuleACM {
			same = rows[i].Solved == prev.Solved && rows[i].PenaltyMinutes == prev.PenaltyMinutes
		} else {
			same = rows[i].TotalScore.Cmp(prev.TotalScore) == 0
		}
		if same {
			rows[i].Rank = prev.Rank
		} else {
			rows[i].Rank = i + 1
		}
	}
}
