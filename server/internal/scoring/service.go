package scoring

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"gorm.io/gorm"
)

type Module struct {
	DB      *gorm.DB
	Clock   clock.Clock
	Hub     *Hub
	hubOnce sync.Once
}

func (m *Module) Standings(ctx context.Context, p access.Principal, contestID uuid.UUID) ([]Standing, error) {
	c, err := m.contest(ctx, contestID)
	if err != nil {
		return nil, err
	}
	in, err := m.input(ctx, c)
	if err != nil {
		return nil, err
	}
	rows := ScoreContest(in)
	aud := m.audience(ctx, p, c)
	vis := m.visibility(c, aud)
	view := Visible(vis)
	if view.ShowOnBoard || aud == access.AudienceContestAdmin || aud == access.AudienceOrgAdmin {
		if domain.Rule(c.Rule) == domain.RuleACM && acmFrozen(vis) && aud != access.AudienceContestAdmin && aud != access.AudienceOrgAdmin {
			return m.filterFrozen(c, vis, aud, rows), nil
		}
		return rows, nil
	}
	if aud == access.AudienceSelf || aud == access.AudienceTeammate {
		return m.onlySelf(ctx, p, contestID, rows), nil
	}
	return nil, apperr.New(apperr.Forbidden, "scoreboard is not visible")
}

func (m *Module) TryPublish(ctx context.Context) error {
	var contests []persist.Contest
	now := m.Clock.Now().UTC()
	if err := m.DB.WithContext(ctx).Where("published_at IS NULL AND ends_at <= ?", now).Find(&contests).Error; err != nil {
		return err
	}
	for _, c := range contests {
		items, err := m.workItems(ctx, c)
		if err != nil {
			continue
		}
		if !PublicationBarrier(true, items).Ready {
			continue
		}
		_ = m.DB.WithContext(ctx).Model(&c).Update("published_at", now).Error
	}
	return nil
}

func (m *Module) CanReadSource(ctx context.Context, p access.Principal, sub persist.Submission) error {
	c, err := m.contest(ctx, sub.ContestID)
	if err != nil {
		return err
	}
	aud := m.audienceForSubmission(ctx, p, c, sub)
	vis := m.visibility(c, aud)
	vis.OwnSubmission = aud == access.AudienceSelf || aud == access.AudienceTeammate
	if !Visible(vis).ShowSource {
		return apperr.New(apperr.Forbidden, "source is not visible")
	}
	if p.Zero() {
		return apperr.New(apperr.Forbidden, "source is not visible")
	}
	return nil
}

func (m *Module) FeedbackLevel(ctx context.Context, p access.Principal, sub persist.Submission) View {
	c, err := m.contest(ctx, sub.ContestID)
	if err != nil {
		return View{}
	}
	aud := m.audienceForSubmission(ctx, p, c, sub)
	vis := m.visibility(c, aud)
	vis.OwnSubmission = aud == access.AudienceSelf || aud == access.AudienceTeammate
	var cp persist.ContestProblem
	if err := m.DB.WithContext(ctx).First(&cp, "id = ?", sub.ContestProblemID).Error; err == nil {
		_, displayed := m.loadResult(ctx, sub, cp)
		vis.ScoringRevision = cp.ScoringRevisionID
		vis.SubmissionRevision = displayed
	}
	return Visible(vis)
}

func (m *Module) ViewSubmission(ctx context.Context, p access.Principal, sub persist.Submission) (SubmissionPublic, View, error) {
	c, err := m.contest(ctx, sub.ContestID)
	if err != nil {
		return SubmissionPublic{}, View{}, err
	}
	var cp persist.ContestProblem
	if err := m.DB.WithContext(ctx).First(&cp, "id = ?", sub.ContestProblemID).Error; err != nil {
		return SubmissionPublic{}, View{}, apperr.New(apperr.NotFound, "contest problem not found")
	}
	detail, displayed := m.loadResult(ctx, sub, cp)
	aud := m.audienceForSubmission(ctx, p, c, sub)
	vis := m.visibility(c, aud)
	vis.OwnSubmission = aud == access.AudienceSelf || aud == access.AudienceTeammate
	vis.ScoringRevision = cp.ScoringRevisionID
	vis.SubmissionRevision = displayed
	view := Visible(vis)
	meta := SubmissionMeta{
		ID: sub.ID, ContestID: sub.ContestID, EntrantID: sub.EntrantID,
		SubmitterAccountID: sub.SubmitterAccountID, LanguageID: sub.LanguageID,
		ReceivedAt: sub.ReceivedAt, ReceiveSeq: sub.ReceiveSeq, RevisionID: displayed,
	}
	return ProjectSubmission(meta, view, detail), view, nil
}

func (m *Module) WatchContest(ctx context.Context, contestID uuid.UUID) (map[uuid.UUID]string, error) {
	var subs []persist.Submission
	if err := m.DB.WithContext(ctx).Select("id", "judge_data_revision_id").Where("contest_id = ?", contestID).Find(&subs).Error; err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]string, len(subs))
	ids := make([]uuid.UUID, 0, len(subs))
	for _, s := range subs {
		ids = append(ids, s.ID)
		out[s.ID] = s.JudgeDataRevisionID.String()
	}
	if len(ids) == 0 {
		return out, nil
	}
	var results []persist.SubmissionResult
	if err := m.DB.WithContext(ctx).Where("submission_id IN ?", ids).Find(&results).Error; err != nil {
		return nil, err
	}
	for _, sr := range results {
		out[sr.SubmissionID] += fmt.Sprintf("|%s:%s:%d:%t:%s", sr.RevisionID, sr.Verdict, sr.ScoreHundredths, sr.SystemFault, sr.AttemptID)
	}
	return out, nil
}

func (m *Module) loadResult(ctx context.Context, sub persist.Submission, cp persist.ContestProblem) (*ResultDetail, uuid.UUID) {
	order := []uuid.UUID{cp.TargetRevisionID, sub.JudgeDataRevisionID, cp.ScoringRevisionID}
	seen := map[uuid.UUID]struct{}{}
	for _, rev := range order {
		if rev == uuid.Nil {
			continue
		}
		if _, ok := seen[rev]; ok {
			continue
		}
		seen[rev] = struct{}{}
		var sr persist.SubmissionResult
		if err := m.DB.WithContext(ctx).Where("submission_id = ? AND revision_id = ?", sub.ID, rev).First(&sr).Error; err != nil {
			continue
		}
		d := resultDetailFrom(sr)
		m.attachCases(ctx, &d, sr.AttemptID)
		return &d, rev
	}
	return nil, sub.JudgeDataRevisionID
}

func resultDetailFrom(sr persist.SubmissionResult) ResultDetail {
	var stored []SubtaskOutcome
	_ = json.Unmarshal(sr.SubtaskScores, &stored)
	return ResultDetail{
		HasResult:     true,
		RevisionID:    sr.RevisionID,
		Verdict:       domain.Verdict(sr.Verdict),
		Score:         exact.FromHundredths(sr.ScoreHundredths),
		Subtasks:      stored,
		SystemFault:   sr.SystemFault,
		CompileFailed: domain.Verdict(sr.Verdict) == domain.VerdictCompilationError,
	}
}

func (m *Module) attachCases(ctx context.Context, d *ResultDetail, attemptID uuid.UUID) {
	if attemptID == uuid.Nil {
		return
	}
	var att persist.JudgingAttempt
	if err := m.DB.WithContext(ctx).First(&att, "id = ?", attemptID).Error; err != nil || len(att.Result) == 0 {
		return
	}
	var raw struct {
		Cases          []CaseOutcome `json:"Cases"`
		CompilerFailed bool          `json:"CompilerFailed"`
	}
	if json.Unmarshal(att.Result, &raw) != nil {
		return
	}
	if len(raw.Cases) > 0 {
		cases := make([]CaseOutcome, 0, len(raw.Cases))
		for _, c := range raw.Cases {
			cases = append(cases, CaseOutcome{CaseID: c.CaseID, Verdict: c.Verdict})
		}
		d.Cases = cases
	}
	if raw.CompilerFailed {
		d.CompileFailed = true
	}
}

func (m *Module) contest(ctx context.Context, id uuid.UUID) (persist.Contest, error) {
	var c persist.Contest
	if err := m.DB.WithContext(ctx).First(&c, "id = ?", id).Error; err != nil {
		return persist.Contest{}, apperr.New(apperr.NotFound, "contest not found")
	}
	return c, nil
}

func (m *Module) input(ctx context.Context, c persist.Contest) (ContestInput, error) {
	var cps []persist.ContestProblem
	if err := m.DB.WithContext(ctx).Where("contest_id = ?", c.ID).Find(&cps).Error; err != nil {
		return ContestInput{}, err
	}
	problems := make([]Problem, 0, len(cps))
	for _, cp := range cps {
		problems = append(problems, Problem{ID: cp.ProblemID, RevisionID: cp.ScoringRevisionID})
	}
	var ens []persist.Entrant
	if err := m.DB.WithContext(ctx).Where("contest_id = ? AND status = ?", c.ID, domain.EntrantAccepted).Find(&ens).Error; err != nil {
		return ContestInput{}, err
	}
	entrants := make([]Entrant, 0, len(ens))
	for _, e := range ens {
		entrants = append(entrants, Entrant{ID: e.ID})
	}
	var subs []persist.Submission
	if err := m.DB.WithContext(ctx).Where("contest_id = ?", c.ID).Order("receive_seq").Find(&subs).Error; err != nil {
		return ContestInput{}, err
	}
	outSubs := make([]Submission, 0, len(subs))
	for _, s := range subs {
		rev := revisionFor(cps, s.ProblemID)
		item := Submission{
			ID: s.ID, EntrantID: s.EntrantID, ProblemID: s.ProblemID,
			ReceiveSeq: s.ReceiveSeq, ReceivedAt: s.ReceivedAt, RevisionID: s.JudgeDataRevisionID,
			OnThisRevision: s.JudgeDataRevisionID == rev,
		}
		var sr persist.SubmissionResult
		if err := m.DB.WithContext(ctx).Where("submission_id = ? AND revision_id = ?", s.ID, rev).First(&sr).Error; err == nil {
			item.Result = resultFrom(sr)
		} else if err := m.DB.WithContext(ctx).Where("submission_id = ? AND revision_id = ?", s.ID, s.JudgeDataRevisionID).First(&sr).Error; err == nil {
			item.Result = resultFrom(sr)
			item.RevisionID = s.JudgeDataRevisionID
		}
		outSubs = append(outSubs, item)
	}
	return ContestInput{
		Rule: domain.Rule(c.Rule), PenaltyMinutes: c.ACMPenaltyMinutes, Start: c.StartsAt,
		Problems: problems, Entrants: entrants, Submissions: outSubs,
	}, nil
}

func revisionFor(cps []persist.ContestProblem, problemID uuid.UUID) uuid.UUID {
	for _, cp := range cps {
		if cp.ProblemID == problemID {
			return cp.ScoringRevisionID
		}
	}
	return uuid.Nil
}

func resultFrom(sr persist.SubmissionResult) *Result {
	var stored []SubtaskOutcome
	_ = json.Unmarshal(sr.SubtaskScores, &stored)
	subs := make([]exact.Score, 0, len(stored))
	for _, s := range stored {
		subs = append(subs, s.Score)
	}
	return &Result{
		Verdict: domain.Verdict(sr.Verdict), Score: exact.FromHundredths(sr.ScoreHundredths),
		SubtaskScores: subs, SystemFault: sr.SystemFault,
		CompileFailed: domain.Verdict(sr.Verdict) == domain.VerdictCompilationError,
	}
}

func (m *Module) workItems(ctx context.Context, c persist.Contest) ([]WorkItem, error) {
	var subs []persist.Submission
	if err := m.DB.WithContext(ctx).Where("contest_id = ?", c.ID).Find(&subs).Error; err != nil {
		return nil, err
	}
	var cps []persist.ContestProblem
	if err := m.DB.WithContext(ctx).Where("contest_id = ?", c.ID).Find(&cps).Error; err != nil {
		return nil, err
	}
	openBatch := false
	for _, cp := range cps {
		var n int64
		if err := m.DB.WithContext(ctx).Model(&persist.RejudgeBatch{}).
			Where("contest_problem_id = ? AND state IN ?", cp.ID, []string{"running", "ready_to_switch"}).
			Count(&n).Error; err != nil {
			return nil, err
		}
		if n > 0 {
			openBatch = true
		}
	}
	items := make([]WorkItem, 0, len(subs))
	for _, s := range subs {
		rev := revisionFor(cps, s.ProblemID)
		it := WorkItem{SubmissionID: s.ID, OpenRejudgeBatch: openBatch}
		var sr persist.SubmissionResult
		if err := m.DB.WithContext(ctx).Where("submission_id = ? AND revision_id = ?", s.ID, rev).First(&sr).Error; err == nil {
			it.HasNormalResultForScoringRev = !sr.SystemFault && domain.Verdict(sr.Verdict).IsNormalTerminal()
			it.SystemFault = sr.SystemFault
		}
		var n int64
		if err := m.DB.WithContext(ctx).Model(&persist.JudgeTask{}).
			Where("submission_id = ? AND state IN ?", s.ID, []string{string(domain.TaskAwaitingAdmin), string(domain.TaskSchedulingError)}).
			Count(&n).Error; err != nil {
			return nil, err
		}
		it.AwaitingAdmin = n > 0
		items = append(items, it)
	}
	return items, nil
}

func (m *Module) visibility(c persist.Contest, aud access.Audience) VisibilityInput {
	now := m.Clock.Now().UTC()
	phase := domain.Phase(now, c.StartsAt, c.EndsAt, c.PublishedAt != nil)
	var freeze time.Duration
	if c.FreezeMinutesBeforeEnd != nil {
		freeze = time.Duration(*c.FreezeMinutesBeforeEnd) * time.Minute
	}
	return VisibilityInput{
		Rule: domain.Rule(c.Rule), Phase: phase, Now: now, EndsAt: c.EndsAt, FreezeFor: freeze,
		Granularity: domain.FeedbackGranularity(c.CaseFeedback), IOIPublicBoard: c.IOIPublicScoreboard,
		SourcePublic: c.SourcePublicAfter, Published: c.PublishedAt != nil, Audience: aud,
	}
}

func (m *Module) audience(ctx context.Context, p access.Principal, c persist.Contest) access.Audience {
	if p.Zero() {
		return access.AudienceAnonymous
	}
	if p.SiteAdmin {
		return access.AudienceOrgAdmin
	}
	var mem persist.OrganizationMembership
	if err := m.DB.WithContext(ctx).Where("organization_id = ? AND account_id = ?", c.OrganizationID, p.AccountID).First(&mem).Error; err == nil {
		if domain.OrgRole(mem.Role).CanManageOrg() {
			return access.AudienceOrgAdmin
		}
	}
	var em persist.EntrantMember
	if err := m.DB.WithContext(ctx).Where("contest_id = ? AND account_id = ?", c.ID, p.AccountID).First(&em).Error; err == nil {
		return access.AudienceSelf
	}
	return access.AudienceLoggedIn
}

func (m *Module) audienceForSubmission(ctx context.Context, p access.Principal, c persist.Contest, sub persist.Submission) access.Audience {
	aud := m.audience(ctx, p, c)
	if aud == access.AudienceOrgAdmin {
		return access.AudienceContestAdmin
	}
	if p.AccountID == sub.SubmitterAccountID {
		return access.AudienceSelf
	}
	var em persist.EntrantMember
	if err := m.DB.WithContext(ctx).Where("contest_id = ? AND account_id = ? AND entrant_id = ?", c.ID, p.AccountID, sub.EntrantID).First(&em).Error; err == nil {
		return access.AudienceTeammate
	}
	if aud == access.AudienceSelf {
		return access.AudienceOtherContestant
	}
	return aud
}

func (m *Module) onlySelf(ctx context.Context, p access.Principal, contestID uuid.UUID, rows []Standing) []Standing {
	var em persist.EntrantMember
	if err := m.DB.WithContext(ctx).Where("contest_id = ? AND account_id = ?", contestID, p.AccountID).First(&em).Error; err != nil {
		return nil
	}
	for _, r := range rows {
		if r.EntrantID == em.EntrantID {
			return []Standing{r}
		}
	}
	return nil
}

func (m *Module) filterFrozen(c persist.Contest, vis VisibilityInput, aud access.Audience, rows []Standing) []Standing {
	freezeAt := c.EndsAt.Add(-vis.FreezeFor)
	out := append([]Standing(nil), rows...)
	for i := range out {
		out[i].Problems = append([]ProblemScore(nil), rows[i].Problems...)
		out[i].Solved = 0
		out[i].PenaltyMinutes = 0
		for j := range out[i].Problems {
			ps := &out[i].Problems[j]
			if ps.EffectiveSubmissionID == uuid.Nil {
				continue
			}
			var s persist.Submission
			if err := m.DB.First(&s, "id = ?", ps.EffectiveSubmissionID).Error; err != nil {
				continue
			}
			if !FilterBoard(vis, !s.ReceivedAt.Before(freezeAt), aud) {
				ps.Solved = false
				ps.Score = exact.Zero()
				ps.PenaltyMinutes = 0
				ps.EffectiveSubmissionID = uuid.Nil
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
