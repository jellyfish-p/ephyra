package scoring

import (
	"context"
	"encoding/json"
	"fmt"
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
	DB    *gorm.DB
	Clock clock.Clock
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
			return FilterFrozenStandings(vis, m.viewerEntrant(ctx, p, contestID), rows), nil
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
			if ctx.Err() == nil {
				return fmt.Errorf("publication work for %s: %w", c.ID, err)
			}
			return err
		}
		if !PublicationBarrier(true, items).Ready {
			continue
		}
		if err := m.DB.WithContext(ctx).Model(&c).Update("published_at", now).Error; err != nil {
			return err
		}
	}
	return nil
}

func (m *Module) loadSubmission(ctx context.Context, id uuid.UUID) (persist.Submission, error) {
	var sub persist.Submission
	if err := m.DB.WithContext(ctx).First(&sub, "id = ?", id).Error; err != nil {
		if persist.IsNotFound(err) {
			return persist.Submission{}, apperr.New(apperr.NotFound, "submission not found")
		}
		return persist.Submission{}, err
	}
	return sub, nil
}

// SourceRef is the storage handle for a submission's source. It is not a GORM model.
type SourceRef struct {
	StorageKey  string
	SHA256      string
	ContentType string
}

// AuthorizedSource checks visibility then returns the blob identity. Callers
// must not fetch source by id without going through this method.
func (m *Module) AuthorizedSource(ctx context.Context, p access.Principal, submissionID uuid.UUID) (SourceRef, error) {
	if err := m.CanReadSource(ctx, p, submissionID); err != nil {
		return SourceRef{}, err
	}
	sub, err := m.loadSubmission(ctx, submissionID)
	if err != nil {
		return SourceRef{}, err
	}
	var obj persist.Object
	if err := m.DB.WithContext(ctx).First(&obj, "id = ?", sub.SourceObjectID).Error; err != nil {
		if persist.IsNotFound(err) {
			return SourceRef{}, apperr.New(apperr.NotFound, "object not found")
		}
		return SourceRef{}, err
	}
	return SourceRef{StorageKey: obj.StorageKey, SHA256: obj.SHA256, ContentType: obj.ContentType}, nil
}

func (m *Module) CanReadSource(ctx context.Context, p access.Principal, submissionID uuid.UUID) error {
	sub, err := m.loadSubmission(ctx, submissionID)
	if err != nil {
		return err
	}
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

func (m *Module) ViewSubmission(ctx context.Context, p access.Principal, submissionID uuid.UUID) (SubmissionPublic, View, error) {
	sub, err := m.loadSubmission(ctx, submissionID)
	if err != nil {
		return SubmissionPublic{}, View{}, err
	}
	c, err := m.contest(ctx, sub.ContestID)
	if err != nil {
		return SubmissionPublic{}, View{}, err
	}
	return m.projectSubmission(ctx, p, c, sub)
}

func (m *Module) projectSubmission(ctx context.Context, p access.Principal, c persist.Contest, sub persist.Submission) (SubmissionPublic, View, error) {
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

type SubmissionFilter struct {
	EntrantID        *uuid.UUID
	ContestProblemID *uuid.UUID
	Limit            int
	BeforeSeq        *int64
}

// ListSubmissions uses the same projection as individual reads and SSE. A row
// without feedback rights is omitted entirely, including its identity metadata.
func (m *Module) ListSubmissions(ctx context.Context, p access.Principal, contestID uuid.UUID, f SubmissionFilter) ([]SubmissionPublic, error) {
	if p.Zero() {
		return nil, apperr.New(apperr.Unauthorized, "not signed in")
	}
	c, err := m.contest(ctx, contestID)
	if err != nil {
		return nil, err
	}
	limit := f.Limit
	if limit == 0 {
		limit = 50
	}
	limit = max(1, min(200, limit))
	// Entrant names label scoreboard-style rows. They are only attached to rows
	// the viewer already has feedback rights to, so this reveals nothing beyond
	// what the public board shows.
	var ens []persist.Entrant
	if err := m.DB.WithContext(ctx).Select("id", "display_name").Where("contest_id = ?", contestID).Find(&ens).Error; err != nil {
		return nil, err
	}
	names := make(map[uuid.UUID]string, len(ens))
	for _, e := range ens {
		names[e.ID] = e.DisplayName
	}
	out := make([]SubmissionPublic, 0, limit)
	before := f.BeforeSeq
	// Apply the limit to visible rows, not candidates: hidden newer submissions
	// must not prevent the viewer from paging through their own older work.
	batchSize := max(50, limit)
	for {
		query := m.DB.WithContext(ctx).Where("contest_id = ?", contestID)
		if f.EntrantID != nil {
			query = query.Where("entrant_id = ?", *f.EntrantID)
		}
		if f.ContestProblemID != nil {
			query = query.Where("contest_problem_id = ?", *f.ContestProblemID)
		}
		if before != nil {
			query = query.Where("receive_seq < ?", *before)
		}
		var candidates []persist.Submission
		if err := query.Order("receive_seq DESC").Limit(batchSize).Find(&candidates).Error; err != nil {
			return nil, err
		}
		for _, sub := range candidates {
			item, view, err := m.projectSubmission(ctx, p, c, sub)
			if err != nil {
				return nil, err
			}
			if ShouldEmit(view) {
				item.EntrantDisplayName = names[item.EntrantID]
				out = append(out, item)
				if len(out) == limit {
					return out, nil
				}
			}
		}
		if len(candidates) < batchSize {
			return out, nil
		}
		seq := candidates[len(candidates)-1].ReceiveSeq
		before = &seq
	}
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
	// Ordered by letter so the scoreboard columns line up with the contest's
	// problem list, which is ordered the same way.
	if err := m.DB.WithContext(ctx).Where("contest_id = ?", c.ID).Order("letter").Find(&cps).Error; err != nil {
		return ContestInput{}, err
	}
	problems := make([]Problem, 0, len(cps))
	for _, cp := range cps {
		problems = append(problems, Problem{ID: cp.ProblemID, Letter: cp.Letter, RevisionID: cp.ScoringRevisionID})
	}
	var ens []persist.Entrant
	if err := m.DB.WithContext(ctx).Where("contest_id = ? AND status = ?", c.ID, domain.EntrantAccepted).Find(&ens).Error; err != nil {
		return ContestInput{}, err
	}
	entrants := make([]Entrant, 0, len(ens))
	for _, e := range ens {
		entrants = append(entrants, Entrant{ID: e.ID, DisplayName: e.DisplayName})
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

func (m *Module) viewerEntrant(ctx context.Context, p access.Principal, contestID uuid.UUID) uuid.UUID {
	var em persist.EntrantMember
	if err := m.DB.WithContext(ctx).Where("contest_id = ? AND account_id = ?", contestID, p.AccountID).First(&em).Error; err != nil {
		return uuid.Nil
	}
	return em.EntrantID
}

func (m *Module) onlySelf(ctx context.Context, p access.Principal, contestID uuid.UUID, rows []Standing) []Standing {
	id := m.viewerEntrant(ctx, p, contestID)
	if id == uuid.Nil {
		return nil
	}
	for _, r := range rows {
		if r.EntrantID == id {
			return []Standing{r}
		}
	}
	return nil
}
