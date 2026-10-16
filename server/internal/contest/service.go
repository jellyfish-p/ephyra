package contest

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"gorm.io/gorm"
)

type Module struct {
	DB    *gorm.DB
	Clock clock.Clock
}

type CreateInput struct {
	OrganizationID             uuid.UUID
	Slug                       string
	Title                      string
	Rule                       domain.Rule
	EntryMode                  domain.EntryMode
	StartsAt                   time.Time
	EndsAt                     time.Time
	RegisterStartsAt           time.Time
	AllowExternal              bool
	RegistrationRequiresReview bool
	MaxTeamSize                int
	ACMPenaltyMinutes          int
	FreezeMinutesBeforeEnd     *int
	IOIPublicScoreboard        bool
	CaseFeedback               domain.FeedbackGranularity
	SourcePublicAfter          bool
}

func (m *Module) now() time.Time { return m.Clock.Now().UTC() }

func (m *Module) Create(ctx context.Context, p access.Principal, in CreateInput) (persist.Contest, error) {
	if err := m.requireManage(ctx, p, in.OrganizationID); err != nil {
		return persist.Contest{}, err
	}
	if in.MaxTeamSize < 1 {
		in.MaxTeamSize = 1
	}
	if in.EntryMode == domain.EntryIndividual {
		in.MaxTeamSize = 1
	}
	if in.ACMPenaltyMinutes <= 0 {
		in.ACMPenaltyMinutes = domain.DefaultACMPenaltyMinutes
	}
	if in.CaseFeedback == "" {
		in.CaseFeedback = domain.FeedbackSubtasks
	}
	if !in.EndsAt.After(in.StartsAt) {
		return persist.Contest{}, apperr.New(apperr.Invalid, "end must be after start")
	}
	now := m.now()
	c := persist.Contest{
		ID: uuid.New(), OrganizationID: in.OrganizationID,
		Slug: strings.ToLower(strings.TrimSpace(in.Slug)), Title: strings.TrimSpace(in.Title),
		Rule: string(in.Rule), EntryMode: string(in.EntryMode),
		StartsAt: in.StartsAt.UTC(), EndsAt: in.EndsAt.UTC(), RegisterStartsAt: in.RegisterStartsAt.UTC(),
		AllowExternal: in.AllowExternal, RegistrationRequiresReview: in.RegistrationRequiresReview,
		MaxTeamSize: in.MaxTeamSize, ACMPenaltyMinutes: in.ACMPenaltyMinutes,
		FreezeMinutesBeforeEnd: in.FreezeMinutesBeforeEnd, IOIPublicScoreboard: in.IOIPublicScoreboard,
		CaseFeedback: string(in.CaseFeedback), SourcePublicAfter: in.SourcePublicAfter,
		SourceVisibilityLocked: !now.Before(in.RegisterStartsAt.UTC()),
		CreatedAt:              now,
	}
	if c.Slug == "" || c.Title == "" {
		return persist.Contest{}, apperr.New(apperr.Invalid, "slug and title are required")
	}
	if err := m.DB.WithContext(ctx).Create(&c).Error; err != nil {
		return persist.Contest{}, apperr.New(apperr.Conflict, "could not create contest")
	}
	return c, nil
}

func (m *Module) SetRule(ctx context.Context, p access.Principal, contestID uuid.UUID, rule domain.Rule) error {
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		c, err := m.loadContest(tx, contestID)
		if err != nil {
			return err
		}
		if err := m.requireManage(ctx, p, c.OrganizationID); err != nil {
			return err
		}
		if err := CanChangeRule(snap(c), m.now()); err != nil {
			return err
		}
		return tx.Model(&c).Update("rule", string(rule)).Error
	})
}

func (m *Module) AddProblem(ctx context.Context, p access.Principal, contestID, problemID, revisionID uuid.UUID, letter string) (persist.ContestProblem, error) {
	letter = strings.ToUpper(strings.TrimSpace(letter))
	var cp persist.ContestProblem
	err := m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		c, err := m.loadContest(tx, contestID)
		if err != nil {
			return err
		}
		if err := m.requireManage(ctx, p, c.OrganizationID); err != nil {
			return err
		}
		var prob persist.Problem
		if err := tx.First(&prob, "id = ?", problemID).Error; err != nil || prob.OrganizationID != c.OrganizationID {
			return apperr.New(apperr.NotFound, "problem not found in this organization")
		}
		var rev persist.JudgeDataRevision
		if err := tx.First(&rev, "id = ? AND problem_id = ?", revisionID, problemID).Error; err != nil {
			return apperr.New(apperr.NotFound, "revision not found")
		}
		cp = persist.ContestProblem{
			ID: uuid.New(), ContestID: contestID, ProblemID: problemID, Letter: letter,
			ScoringRevisionID: revisionID, TargetRevisionID: revisionID,
		}
		if err := tx.Create(&cp).Error; err != nil {
			return apperr.New(apperr.Conflict, "letter or problem already in contest")
		}
		return nil
	})
	return cp, err
}

func (m *Module) RegisterIndividual(ctx context.Context, p access.Principal, contestID uuid.UUID) (persist.Entrant, error) {
	var out persist.Entrant
	err := m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		c, err := m.loadContest(tx, contestID)
		if err != nil {
			return err
		}
		if domain.EntryMode(c.EntryMode) != domain.EntryIndividual {
			return apperr.New(apperr.Conflict, "contest uses team entry")
		}
		member := m.isOrgMember(tx, p.AccountID, c.OrganizationID)
		if err := CanRegister(snap(c), m.now(), member, p.EmailVerified); err != nil {
			return err
		}
		m.lockSource(tx, &c)
		status := domain.EntrantAccepted
		now := m.now()
		var accepted *time.Time
		if c.RegistrationRequiresReview {
			status = domain.EntrantPending
		} else {
			accepted = &now
		}
		out = persist.Entrant{
			ID: uuid.New(), ContestID: contestID, Kind: string(domain.EntryIndividual),
			DisplayName: p.DisplayName, Status: string(status), RegisteredAt: now, AcceptedAt: accepted,
		}
		if accepted != nil {
			out.RosterFrozenAt = accepted
		}
		if err := tx.Create(&out).Error; err != nil {
			return err
		}
		mem := persist.EntrantMember{
			ID: uuid.New(), ContestID: contestID, EntrantID: out.ID, AccountID: p.AccountID,
			Role: "captain", ConfirmedAt: &now,
		}
		if err := tx.Create(&mem).Error; err != nil {
			return apperr.New(apperr.Conflict, "already registered in this contest")
		}
		return nil
	})
	return out, err
}

func (m *Module) CreateTeam(ctx context.Context, p access.Principal, contestID uuid.UUID, name string) (persist.Entrant, error) {
	name = strings.TrimSpace(name)
	var out persist.Entrant
	err := m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		c, err := m.loadContest(tx, contestID)
		if err != nil {
			return err
		}
		if domain.EntryMode(c.EntryMode) != domain.EntryTeam {
			return apperr.New(apperr.Conflict, "contest uses individual entry")
		}
		member := m.isOrgMember(tx, p.AccountID, c.OrganizationID)
		if err := CanRegister(snap(c), m.now(), member, p.EmailVerified); err != nil {
			return err
		}
		m.lockSource(tx, &c)
		now := m.now()
		out = persist.Entrant{
			ID: uuid.New(), ContestID: contestID, Kind: string(domain.EntryTeam),
			DisplayName: name, Status: string(domain.EntrantPending), RegisteredAt: now,
		}
		if err := tx.Create(&out).Error; err != nil {
			return err
		}
		cap := persist.EntrantMember{
			ID: uuid.New(), ContestID: contestID, EntrantID: out.ID, AccountID: p.AccountID,
			Role: "captain", ConfirmedAt: &now,
		}
		if err := tx.Create(&cap).Error; err != nil {
			return apperr.New(apperr.Conflict, "already registered in this contest")
		}
		return nil
	})
	return out, err
}

func (m *Module) Invite(ctx context.Context, p access.Principal, entrantID, accountID uuid.UUID) error {
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var en persist.Entrant
		if err := tx.First(&en, "id = ?", entrantID).Error; err != nil {
			return apperr.New(apperr.NotFound, "team not found")
		}
		c, err := m.loadContest(tx, en.ContestID)
		if err != nil {
			return err
		}
		if err := CanMutateRoster(en.AcceptedAt, c.StartsAt, m.now()); err != nil {
			return err
		}
		if err := m.requireCaptain(tx, p.AccountID, en.ID); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&persist.EntrantMember{}).Where("entrant_id = ?", en.ID).Count(&count).Error; err != nil {
			return err
		}
		if err := TeamSizeOK(snap(c), int(count)+1); err != nil {
			return err
		}
		member := persist.EntrantMember{
			ID: uuid.New(), ContestID: en.ContestID, EntrantID: en.ID, AccountID: accountID, Role: "member",
		}
		if err := tx.Create(&member).Error; err != nil {
			return apperr.New(apperr.Conflict, "account already belongs to an entrant in this contest")
		}
		return nil
	})
}

func (m *Module) ConfirmInvite(ctx context.Context, p access.Principal, entrantID uuid.UUID) error {
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var mem persist.EntrantMember
		if err := tx.Where("entrant_id = ? AND account_id = ?", entrantID, p.AccountID).First(&mem).Error; err != nil {
			return apperr.New(apperr.NotFound, "invitation not found")
		}
		if mem.ConfirmedAt != nil {
			return nil
		}
		if !p.EmailVerified {
			return apperr.New(apperr.Unverified, "verify email before joining a team")
		}
		var en persist.Entrant
		if err := tx.First(&en, "id = ?", entrantID).Error; err != nil {
			return err
		}
		c, err := m.loadContest(tx, en.ContestID)
		if err != nil {
			return err
		}
		orgMember := m.isOrgMember(tx, p.AccountID, c.OrganizationID)
		if !orgMember && !c.AllowExternal {
			return apperr.New(apperr.Forbidden, "this contest does not allow external contestants")
		}
		now := m.now()
		return tx.Model(&mem).Update("confirmed_at", now).Error
	})
}

func (m *Module) SubmitTeam(ctx context.Context, p access.Principal, entrantID uuid.UUID) error {
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var en persist.Entrant
		if err := tx.First(&en, "id = ?", entrantID).Error; err != nil {
			return apperr.New(apperr.NotFound, "team not found")
		}
		if err := m.requireCaptain(tx, p.AccountID, en.ID); err != nil {
			return err
		}
		c, err := m.loadContest(tx, en.ContestID)
		if err != nil {
			return err
		}
		var members []persist.EntrantMember
		if err := tx.Where("entrant_id = ?", en.ID).Find(&members).Error; err != nil {
			return err
		}
		for _, mem := range members {
			if mem.ConfirmedAt == nil {
				return apperr.New(apperr.Conflict, "all members must confirm before registration")
			}
		}
		if err := TeamSizeOK(snap(c), len(members)); err != nil {
			return err
		}
		now := m.now()
		status := domain.EntrantAccepted
		var accepted *time.Time
		if c.RegistrationRequiresReview {
			status = domain.EntrantPending
		} else {
			accepted = &now
		}
		updates := map[string]any{"status": string(status), "accepted_at": accepted}
		if accepted != nil {
			updates["roster_frozen_at"] = accepted
		}
		return tx.Model(&en).Updates(updates).Error
	})
}

func (m *Module) DecideRegistration(ctx context.Context, p access.Principal, entrantID uuid.UUID, approve bool) error {
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var en persist.Entrant
		if err := tx.First(&en, "id = ?", entrantID).Error; err != nil {
			return apperr.New(apperr.NotFound, "entrant not found")
		}
		c, err := m.loadContest(tx, en.ContestID)
		if err != nil {
			return err
		}
		if err := m.requireManage(ctx, p, c.OrganizationID); err != nil {
			return err
		}
		now := m.now()
		if !approve {
			return tx.Model(&en).Update("status", string(domain.EntrantRejected)).Error
		}
		return tx.Model(&en).Updates(map[string]any{
			"status": string(domain.EntrantAccepted), "accepted_at": now, "roster_frozen_at": now,
		}).Error
	})
}

func (m *Module) Withdraw(ctx context.Context, p access.Principal, entrantID uuid.UUID) error {
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var en persist.Entrant
		if err := tx.First(&en, "id = ?", entrantID).Error; err != nil {
			return apperr.New(apperr.NotFound, "entrant not found")
		}
		c, err := m.loadContest(tx, en.ContestID)
		if err != nil {
			return err
		}
		if !m.now().Before(c.StartsAt) && en.AcceptedAt != nil {
			return apperr.New(apperr.Conflict, "cannot withdraw after start")
		}
		if err := m.requireCaptain(tx, p.AccountID, en.ID); err != nil {
			return err
		}
		if err := tx.Where("entrant_id = ?", en.ID).Delete(&persist.EntrantMember{}).Error; err != nil {
			return err
		}
		return tx.Model(&en).Updates(map[string]any{"status": string(domain.EntrantWithdrawn), "accepted_at": nil}).Error
	})
}

func (m *Module) Get(ctx context.Context, id uuid.UUID) (persist.Contest, error) {
	return m.loadContest(m.DB.WithContext(ctx), id)
}

func (m *Module) Problems(ctx context.Context, contestID uuid.UUID) ([]persist.ContestProblem, error) {
	var list []persist.ContestProblem
	if err := m.DB.WithContext(ctx).Where("contest_id = ?", contestID).Order("letter").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (m *Module) EntrantForAccount(ctx context.Context, contestID, accountID uuid.UUID) (persist.Entrant, persist.EntrantMember, error) {
	var mem persist.EntrantMember
	if err := m.DB.WithContext(ctx).Where("contest_id = ? AND account_id = ?", contestID, accountID).First(&mem).Error; err != nil {
		return persist.Entrant{}, persist.EntrantMember{}, apperr.New(apperr.Forbidden, "not registered")
	}
	var en persist.Entrant
	if err := m.DB.WithContext(ctx).First(&en, "id = ?", mem.EntrantID).Error; err != nil {
		return persist.Entrant{}, persist.EntrantMember{}, err
	}
	return en, mem, nil
}

func (m *Module) loadContest(tx *gorm.DB, id uuid.UUID) (persist.Contest, error) {
	var c persist.Contest
	if err := tx.First(&c, "id = ?", id).Error; err != nil {
		return persist.Contest{}, apperr.New(apperr.NotFound, "contest not found")
	}
	return c, nil
}

func (m *Module) requireManage(ctx context.Context, p access.Principal, orgID uuid.UUID) error {
	if p.SiteAdmin {
		return nil
	}
	var mem persist.OrganizationMembership
	if err := m.DB.WithContext(ctx).Where("organization_id = ? AND account_id = ?", orgID, p.AccountID).First(&mem).Error; err != nil {
		return apperr.New(apperr.Forbidden, "not an organization member")
	}
	if !domain.OrgRole(mem.Role).CanManageOrg() {
		return apperr.New(apperr.Forbidden, "insufficient organization role")
	}
	return nil
}

func (m *Module) isOrgMember(tx *gorm.DB, accountID, orgID uuid.UUID) bool {
	var mem persist.OrganizationMembership
	return tx.Where("organization_id = ? AND account_id = ?", orgID, accountID).First(&mem).Error == nil
}

func (m *Module) requireCaptain(tx *gorm.DB, accountID, entrantID uuid.UUID) error {
	var mem persist.EntrantMember
	if err := tx.Where("entrant_id = ? AND account_id = ?", entrantID, accountID).First(&mem).Error; err != nil {
		return apperr.New(apperr.Forbidden, "not a team member")
	}
	if mem.Role != "captain" {
		return apperr.New(apperr.Forbidden, "only the captain can do this")
	}
	return nil
}

func (m *Module) lockSource(tx *gorm.DB, c *persist.Contest) {
	if c.SourceVisibilityLocked {
		return
	}
	_ = tx.Model(c).Update("source_visibility_locked", true).Error
	c.SourceVisibilityLocked = true
}

func snap(c persist.Contest) Snapshot {
	return Snapshot{
		Rule: domain.Rule(c.Rule), EntryMode: domain.EntryMode(c.EntryMode),
		StartsAt: c.StartsAt, EndsAt: c.EndsAt, RegisterStartsAt: c.RegisterStartsAt,
		AllowExternal: c.AllowExternal, RegistrationRequiresReview: c.RegistrationRequiresReview,
		MaxTeamSize: c.MaxTeamSize, SourcePublicAfter: c.SourcePublicAfter,
		SourceVisibilityLocked: c.SourceVisibilityLocked, PublishedAt: c.PublishedAt,
	}
}
