package scoring

import (
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
)

type VisibilityInput struct {
	Rule               domain.Rule
	Phase              domain.ContestPhase
	Now                time.Time
	EndsAt             time.Time
	FreezeFor          time.Duration // ACM freeze window before end; 0 disables
	Granularity        domain.FeedbackGranularity
	IOIPublicBoard     bool
	SourcePublic       bool
	Published          bool
	Audience           access.Audience
	OwnSubmission      bool
	SubmissionRevision uuid.UUID
	ScoringRevision    uuid.UUID
}

type FeedbackLevel int

const (
	FeedbackNone FeedbackLevel = iota
	FeedbackCompileOnly
	FeedbackScore
	FeedbackFull
)

func (f FeedbackLevel) Name() string {
	switch f {
	case FeedbackCompileOnly:
		return "compile"
	case FeedbackScore:
		return "score"
	case FeedbackFull:
		return "full"
	default:
		return "none"
	}
}

type View struct {
	Feedback     FeedbackLevel
	Granularity  domain.FeedbackGranularity
	ShowOnBoard  bool
	ShowSource   bool
	NotEffective bool // visible personal result that is not the scoring revision
}

func Visible(in VisibilityInput) View {
	admin := in.Audience == access.AudienceContestAdmin || in.Audience == access.AudienceOrgAdmin
	self := in.Audience == access.AudienceSelf || in.Audience == access.AudienceTeammate
	v := View{Granularity: in.Granularity, NotEffective: revisionNotEffective(in)}

	v.ShowSource = sourceVisible(in, admin, self)
	if admin {
		v.Feedback = FeedbackFull
		v.ShowOnBoard = true
		return v
	}

	frozen := acmFrozen(in)
	switch in.Rule {
	case domain.RuleOI:
		switch in.Phase {
		case domain.PhaseRunning, domain.PhaseBefore:
			if self {
				v.Feedback = FeedbackCompileOnly
			}
			v.ShowOnBoard = false
		case domain.PhaseEndedUnpublished:
			if self {
				v.Feedback = FeedbackCompileOnly
			}
			v.ShowOnBoard = false
		case domain.PhasePublished:
			if self {
				v.Feedback = FeedbackFull
			} else {
				v.Feedback = FeedbackScore
			}
			v.ShowOnBoard = true
		}
	case domain.RuleIOI:
		if self {
			v.Feedback = FeedbackFull
		}
		v.ShowOnBoard = in.IOIPublicBoard || in.Phase == domain.PhasePublished
		if in.Phase != domain.PhasePublished && !self && !in.IOIPublicBoard {
			v.ShowOnBoard = false
			v.Feedback = FeedbackNone
		}
	case domain.RuleACM:
		if self {
			v.Feedback = FeedbackFull
		}
		v.ShowOnBoard = true
		if frozen && !self {
			v.ShowOnBoard = true
			if !self {
				v.Feedback = FeedbackNone
			}
		}
		if in.Phase == domain.PhaseBefore {
			v.ShowOnBoard = false
		}
	}
	if in.Phase != domain.PhasePublished && in.Granularity == domain.FeedbackCases && in.Rule == domain.RuleOI {
		v.Granularity = domain.FeedbackSubtasks
	}
	if in.Rule == domain.RuleOI && (in.Phase == domain.PhaseRunning || in.Phase == domain.PhaseEndedUnpublished) {
		v.Granularity = domain.FeedbackSubtasks
		if self {
			v.Feedback = FeedbackCompileOnly
		}
	}
	_ = frozen
	return v
}

func acmFrozen(in VisibilityInput) bool {
	if in.Rule != domain.RuleACM || in.FreezeFor <= 0 || in.Published {
		return false
	}
	freezeAt := in.EndsAt.Add(-in.FreezeFor)
	return !in.Now.Before(freezeAt) && in.Phase != domain.PhasePublished
}

func sourceVisible(in VisibilityInput, admin, self bool) bool {
	if admin || self {
		return true
	}
	if in.Audience == access.AudienceAnonymous {
		return false
	}
	if in.SourcePublic && in.Published && in.Audience == access.AudienceLoggedIn {
		return true
	}
	if in.SourcePublic && in.Published && in.Audience == access.AudienceOtherContestant {
		return true
	}
	return false
}

// FilterFrozenStandings hides other entrants' post-freeze ACM solves and re-ranks.
// viewerEntrant is the caller's team; vis.Audience is the caller, not each row.
func FilterFrozenStandings(vis VisibilityInput, viewerEntrant uuid.UUID, rows []Standing) []Standing {
	freezeAt := vis.EndsAt.Add(-vis.FreezeFor)
	out := append([]Standing(nil), rows...)
	for i := range out {
		out[i].Problems = append([]ProblemScore(nil), rows[i].Problems...)
		out[i].Solved = 0
		out[i].PenaltyMinutes = 0
		rowAud := vis.Audience
		if (rowAud == access.AudienceSelf || rowAud == access.AudienceTeammate) && out[i].EntrantID != viewerEntrant {
			rowAud = access.AudienceOtherContestant
		}
		for j := range out[i].Problems {
			ps := &out[i].Problems[j]
			if ps.EffectiveSubmissionID == uuid.Nil {
				continue
			}
			afterFreeze := !ps.ReceivedAt.Before(freezeAt)
			if !FilterBoard(vis, afterFreeze, rowAud) {
				ps.Solved = false
				ps.Score = exact.Zero()
				ps.PenaltyMinutes = 0
				ps.EffectiveSubmissionID = uuid.Nil
				ps.ReceivedAt = time.Time{}
			}
			if ps.Solved {
				out[i].Solved++
				out[i].PenaltyMinutes += ps.PenaltyMinutes
			}
		}
	}
	rank(domain.RuleACM, out)
	return out
}

// FilterBoard hides others' post-freeze ACM solves from public standings.
func FilterBoard(in VisibilityInput, solvedAfterFreeze bool, audience access.Audience) bool {
	if audience == access.AudienceContestAdmin || audience == access.AudienceOrgAdmin {
		return true
	}
	if in.Rule != domain.RuleACM || !acmFrozen(in) {
		return true
	}
	if audience == access.AudienceSelf || audience == access.AudienceTeammate {
		return true
	}
	return !solvedAfterFreeze
}

func revisionNotEffective(in VisibilityInput) bool {
	if in.ScoringRevision == uuid.Nil || in.SubmissionRevision == uuid.Nil {
		return false
	}
	return in.SubmissionRevision != in.ScoringRevision
}
