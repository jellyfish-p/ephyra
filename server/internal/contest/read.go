package contest

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/objectstore"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/problem"
)

func (m *Module) contestProblem(ctx context.Context, contestID uuid.UUID, letter string) (persist.ContestProblem, error) {
	var cp persist.ContestProblem
	err := m.DB.WithContext(ctx).
		Where("contest_id = ? AND LOWER(TRIM(letter)) = ?", contestID, strings.ToLower(strings.TrimSpace(letter))).
		First(&cp).Error
	if persist.IsNotFound(err) {
		return cp, apperr.New(apperr.NotFound, "contest problem not found")
	}
	return cp, err
}

// ContestProblemIDByLetter resolves a contest-problem id, not a problem-bank id.
// Callers must apply the visibility policy for the resource they expose.
func (m *Module) ContestProblemIDByLetter(ctx context.Context, contestID uuid.UUID, letter string) (uuid.UUID, error) {
	cp, err := m.contestProblem(ctx, contestID, letter)
	return cp.ID, err
}

// GetProblemDetail reads public problem data after the caller authorizes with Get.
func (m *Module) GetProblemDetail(ctx context.Context, contestID uuid.UUID, letter string, blobs objectstore.Store) (ProblemDetail, error) {
	cp, err := m.contestProblem(ctx, contestID, letter)
	if err != nil {
		return ProblemDetail{}, err
	}
	var rev persist.JudgeDataRevision
	if err := m.DB.WithContext(ctx).First(&rev, "id = ?", cp.ScoringRevisionID).Error; err != nil {
		if persist.IsNotFound(err) {
			return ProblemDetail{}, apperr.New(apperr.NotFound, "revision not found")
		}
		return ProblemDetail{}, err
	}
	var cfg problem.NormalizedConfig
	if err := json.Unmarshal(rev.NormalizedConfig, &cfg); err != nil {
		return ProblemDetail{}, err
	}
	var prob persist.Problem
	if err := m.DB.WithContext(ctx).Select("title").First(&prob, "id = ?", cp.ProblemID).Error; err != nil && !persist.IsNotFound(err) {
		return ProblemDetail{}, err
	}
	out := problemDetailOf(cp, prob.Title, cfg)
	out.StatementMarkdown = m.statement(ctx, cp.ScoringRevisionID, blobs)
	return out, nil
}

func problemDetailOf(cp persist.ContestProblem, title string, cfg problem.NormalizedConfig) ProblemDetail {
	subtasks := make([]ProblemSubtask, 0, len(cfg.Subtasks))
	for _, st := range cfg.Subtasks {
		subtasks = append(subtasks, ProblemSubtask{ID: st.ID, Score: st.Score})
	}
	return ProblemDetail{
		Letter: cp.Letter, Title: title, ProblemID: cp.ProblemID,
		ScoringRevisionID: cp.ScoringRevisionID, TargetRevisionID: cp.TargetRevisionID,
		Kind: string(cfg.Kind), CheckerKind: string(cfg.Checker.Kind),
		TimeLimitMS: cfg.TimeLimitMS, MemoryLimitBytes: cfg.MemoryLimitBytes,
		Subtasks: subtasks,
	}
}

func (m *Module) statement(ctx context.Context, revisionID uuid.UUID, blobs objectstore.Store) *string {
	// Filter before loading any object: checker/interactor and non-markdown files
	// must never be fetched, even when they have a statement-like name.
	var file persist.RevisionFile
	if err := m.DB.WithContext(ctx).Select("id", "object_id").
		Where("revision_id = ? AND role = ? AND LOWER(name) LIKE ?", revisionID, "data", "%.md").
		Order("CASE WHEN name = 'statement.md' THEN 0 ELSE 1 END").Order("name COLLATE \"C\"").
		First(&file).Error; err != nil {
		return nil
	}
	var obj persist.Object
	if err := m.DB.WithContext(ctx).Select("storage_key").First(&obj, "id = ?", file.ObjectID).Error; err != nil {
		return nil
	}
	return readStatement(ctx, blobs, obj.StorageKey)
}

func readStatement(ctx context.Context, blobs objectstore.Store, key string) *string {
	if blobs == nil {
		return nil
	}
	rc, _, _, err := blobs.Get(ctx, key)
	if err != nil {
		return nil
	}
	defer rc.Close()
	body, err := io.ReadAll(io.LimitReader(rc, 1<<20))
	if err != nil {
		return nil
	}
	text := string(body)
	return &text
}

// MyEntrant reads the account's own roster after the caller authorizes with Get.
func (m *Module) MyEntrant(ctx context.Context, p access.Principal, contestID uuid.UUID) (*MyEntrantView, error) {
	var member persist.EntrantMember
	err := m.DB.WithContext(ctx).Where("contest_id = ? AND account_id = ?", contestID, p.AccountID).First(&member).Error
	if persist.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entrant persist.Entrant
	if err := m.DB.WithContext(ctx).First(&entrant, "id = ? AND contest_id = ?", member.EntrantID, contestID).Error; err != nil {
		if persist.IsNotFound(err) {
			return nil, apperr.New(apperr.NotFound, "entrant not found")
		}
		return nil, err
	}
	out, err := m.entrantView(ctx, entrant)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListEntrants returns a contest's entrants for an organization manager, so a
// contest that requires review can actually be reviewed. Registration review and
// organization membership review are separate processes.
func (m *Module) ListEntrants(ctx context.Context, p access.Principal, contestID uuid.UUID, status string) ([]MyEntrantView, error) {
	var c persist.Contest
	if err := m.DB.WithContext(ctx).First(&c, "id = ?", contestID).Error; err != nil {
		return nil, apperr.New(apperr.NotFound, "contest not found")
	}
	if err := m.requireManage(ctx, p, c.OrganizationID); err != nil {
		return nil, err
	}
	query := m.DB.WithContext(ctx).Where("contest_id = ?", contestID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var ens []persist.Entrant
	if err := query.Order("registered_at").Find(&ens).Error; err != nil {
		return nil, err
	}
	out := make([]MyEntrantView, 0, len(ens))
	for _, e := range ens {
		view, err := m.entrantView(ctx, e)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func (m *Module) entrantView(ctx context.Context, entrant persist.Entrant) (MyEntrantView, error) {
	var members []struct {
		AccountID   uuid.UUID
		DisplayName string
		Role        string
		ConfirmedAt *time.Time
	}
	if err := m.DB.WithContext(ctx).Table("entrant_members AS em").
		Select("em.account_id, a.display_name, em.role, em.confirmed_at").
		Joins("JOIN accounts AS a ON a.id = em.account_id").
		Where("em.entrant_id = ? AND em.contest_id = ?", entrant.ID, entrant.ContestID).
		Order("em.account_id").Scan(&members).Error; err != nil {
		return MyEntrantView{}, err
	}
	out := MyEntrantView{
		ID: entrant.ID, ContestID: entrant.ContestID, Kind: entrant.Kind,
		DisplayName: entrant.DisplayName, Status: entrant.Status, RegisteredAt: entrant.RegisteredAt,
		AcceptedAt: entrant.AcceptedAt, RosterFrozenAt: entrant.RosterFrozenAt,
		Members: make([]EntrantMemberView, 0, len(members)),
	}
	for _, mem := range members {
		out.Members = append(out.Members, EntrantMemberView{
			AccountID: mem.AccountID, DisplayName: mem.DisplayName, Role: mem.Role, Confirmed: mem.ConfirmedAt != nil,
		})
	}
	return out, nil
}
