package identity

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/mailer"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/rate"
	"github.com/jellyfish-p/ephyra/server/internal/sec"
	"gorm.io/gorm"
)

const (
	sessionTTL = 14 * 24 * time.Hour
	verifyTTL  = 24 * time.Hour
	resetTTL   = time.Hour
)

type Module struct {
	DB        *gorm.DB
	Clock     clock.Clock
	Mail      mailer.Mailer
	Limit     *rate.Limiter
	PublicURL string
}

type SessionInfo struct {
	RawToken   string
	CSRFSecret string
	ExpiresAt  time.Time
	Account    persist.Account
}

func (m *Module) now() time.Time { return m.Clock.Now().UTC() }

func (m *Module) Register(ctx context.Context, email, password, displayName, ip string) error {
	if !m.Limit.Allow("register:"+ip, 10, time.Hour) {
		return apperr.New(apperr.TooMany, "too many registrations")
	}
	email = strings.TrimSpace(email)
	norm := sec.NormalizeEmail(email)
	displayName = strings.TrimSpace(displayName)
	if email == "" || displayName == "" || len(password) < 8 {
		return apperr.New(apperr.Invalid, "email, display name, and a password of at least 8 characters are required")
	}
	hash, err := sec.HashPassword(password)
	if err != nil {
		return err
	}
	now := m.now()
	acc := persist.Account{
		ID: uuid.New(), Email: email, EmailNormalized: norm, PasswordHash: hash,
		DisplayName: displayName, CreatedAt: now, UpdatedAt: now,
	}
	if err := m.DB.WithContext(ctx).Create(&acc).Error; err != nil {
		if isUnique(err) {
			return apperr.New(apperr.Conflict, "email already registered")
		}
		return err
	}
	return m.issueEmail(ctx, acc, "verify_email", verifyTTL)
}

func (m *Module) Login(ctx context.Context, email, password, ip, ua string) (SessionInfo, error) {
	if !m.Limit.Allow("login:"+ip, 20, time.Minute) {
		return SessionInfo{}, apperr.New(apperr.TooMany, "too many login attempts")
	}
	var acc persist.Account
	err := m.DB.WithContext(ctx).Where("email_normalized = ?", sec.NormalizeEmail(email)).First(&acc).Error
	if err != nil || acc.DisabledAt != nil || !sec.VerifyPassword(password, acc.PasswordHash) {
		return SessionInfo{}, apperr.New(apperr.Unauthorized, "invalid credentials")
	}
	return m.createSession(ctx, acc, ip, ua)
}

func (m *Module) Logout(ctx context.Context, rawToken string) error {
	now := m.now()
	return m.DB.WithContext(ctx).Model(&persist.Session{}).
		Where("token_hash = ? AND revoked_at IS NULL", sec.HashToken(rawToken)).
		Update("revoked_at", now).Error
}

func (m *Module) Resolve(ctx context.Context, rawToken string) (access.Principal, persist.Session, error) {
	if rawToken == "" {
		return access.Principal{}, persist.Session{}, apperr.New(apperr.Unauthorized, "not signed in")
	}
	var sess persist.Session
	if err := m.DB.WithContext(ctx).Where("token_hash = ?", sec.HashToken(rawToken)).First(&sess).Error; err != nil {
		return access.Principal{}, persist.Session{}, apperr.New(apperr.Unauthorized, "not signed in")
	}
	now := m.now()
	if sess.RevokedAt != nil || now.After(sess.ExpiresAt) {
		return access.Principal{}, persist.Session{}, apperr.New(apperr.Unauthorized, "session expired")
	}
	var acc persist.Account
	if err := m.DB.WithContext(ctx).First(&acc, "id = ?", sess.AccountID).Error; err != nil || acc.DisabledAt != nil {
		return access.Principal{}, persist.Session{}, apperr.New(apperr.Unauthorized, "not signed in")
	}
	_ = m.DB.WithContext(ctx).Model(&sess).Update("last_seen_at", now).Error
	p := access.Principal{
		AccountID: acc.ID, Email: acc.Email, DisplayName: acc.DisplayName,
		EmailVerified: acc.EmailVerifiedAt != nil, SiteAdmin: acc.SiteAdmin,
	}
	return p, sess, nil
}

func (m *Module) VerifyEmail(ctx context.Context, rawToken string) error {
	return m.consumeChallenge(ctx, rawToken, "verify_email", func(tx *gorm.DB, acc persist.Account) error {
		now := m.now()
		return tx.Model(&persist.Account{}).Where("id = ?", acc.ID).Updates(map[string]any{
			"email_verified_at": now, "updated_at": now,
		}).Error
	})
}

func (m *Module) RequestPasswordReset(ctx context.Context, email, ip string) error {
	if !m.Limit.Allow("reset:"+ip, 5, time.Hour) {
		return apperr.New(apperr.TooMany, "too many reset requests")
	}
	var acc persist.Account
	if err := m.DB.WithContext(ctx).Where("email_normalized = ?", sec.NormalizeEmail(email)).First(&acc).Error; err != nil {
		return nil
	}
	if acc.EmailVerifiedAt == nil {
		return nil
	}
	return m.issueEmail(ctx, acc, "reset_password", resetTTL)
}

func (m *Module) ResetPassword(ctx context.Context, rawToken, password string) error {
	if len(password) < 8 {
		return apperr.New(apperr.Invalid, "password too short")
	}
	hash, err := sec.HashPassword(password)
	if err != nil {
		return err
	}
	return m.consumeChallenge(ctx, rawToken, "reset_password", func(tx *gorm.DB, acc persist.Account) error {
		now := m.now()
		if err := tx.Model(&persist.Account{}).Where("id = ?", acc.ID).Updates(map[string]any{
			"password_hash": hash, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&persist.Session{}).Where("account_id = ? AND revoked_at IS NULL", acc.ID).
			Update("revoked_at", now).Error
	})
}

func (m *Module) CreateOrganization(ctx context.Context, p access.Principal, slug, name string) (persist.Organization, error) {
	if !p.SiteAdmin {
		return persist.Organization{}, apperr.New(apperr.Forbidden, "only site admins can create organizations")
	}
	slug = strings.ToLower(strings.TrimSpace(slug))
	name = strings.TrimSpace(name)
	if slug == "" || name == "" {
		return persist.Organization{}, apperr.New(apperr.Invalid, "slug and name are required")
	}
	now := m.now()
	org := persist.Organization{ID: uuid.New(), Slug: slug, Name: name, CreatedAt: now}
	err := m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&org).Error; err != nil {
			if isUnique(err) {
				return apperr.New(apperr.Conflict, "slug already used")
			}
			return err
		}
		mem := persist.OrganizationMembership{
			ID: uuid.New(), OrganizationID: org.ID, AccountID: p.AccountID,
			Role: string(domain.OrgRoleOwner), CreatedAt: now,
		}
		if err := tx.Create(&mem).Error; err != nil {
			return err
		}
		return persist.Audit(tx, persist.AuditEvent{
			ActorID: &p.AccountID, Action: "organization.create", OrganizationID: &org.ID, CreatedAt: now,
		})
	})
	return org, err
}

func (m *Module) ApplyToJoin(ctx context.Context, p access.Principal, orgID uuid.UUID) error {
	if p.Zero() {
		return apperr.New(apperr.Unauthorized, "not signed in")
	}
	now := m.now()
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		org, err := m.loadOrg(tx, orgID)
		if err != nil {
			return err
		}
		if _, ok := m.membershipOf(tx, p.AccountID, org.ID); ok {
			return apperr.New(apperr.Conflict, "already a member")
		}
		req := persist.OrganizationJoinRequest{
			ID: uuid.New(), OrganizationID: org.ID, AccountID: p.AccountID,
			Status: "pending", CreatedAt: now,
		}
		if err := tx.Create(&req).Error; err != nil {
			if isUnique(err) {
				return apperr.New(apperr.Conflict, "request already pending")
			}
			return err
		}
		return nil
	})
}

func (m *Module) DecideJoin(ctx context.Context, p access.Principal, requestID uuid.UUID, approve bool) error {
	now := m.now()
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var req persist.OrganizationJoinRequest
		if err := tx.First(&req, "id = ?", requestID).Error; err != nil {
			return apperr.New(apperr.NotFound, "request not found")
		}
		if _, err := m.loadOrg(tx, req.OrganizationID); err != nil {
			return err
		}
		if err := m.requireOrgManage(tx, p, req.OrganizationID); err != nil {
			return err
		}
		if req.Status != "pending" {
			return apperr.New(apperr.Conflict, "request already decided")
		}
		status := "rejected"
		if approve {
			status = "approved"
			mem := persist.OrganizationMembership{
				ID: uuid.New(), OrganizationID: req.OrganizationID, AccountID: req.AccountID,
				Role: string(domain.OrgRoleMember), CreatedAt: now,
			}
			if err := tx.Create(&mem).Error; err != nil {
				if isUnique(err) {
					return apperr.New(apperr.Conflict, "already a member")
				}
				return err
			}
		}
		return tx.Model(&req).Updates(map[string]any{
			"status": status, "decided_at": now, "decided_by": p.AccountID,
		}).Error
	})
}

func (m *Module) MyMembership(ctx context.Context, p access.Principal, orgID uuid.UUID) (Membership, error) {
	if p.Zero() {
		return Membership{}, apperr.New(apperr.Unauthorized, "not signed in")
	}
	org, err := m.loadOrg(m.DB.WithContext(ctx), orgID)
	if err != nil {
		return Membership{}, err
	}
	mem, ok := m.membershipOf(m.DB.WithContext(ctx), p.AccountID, org.ID)
	if !ok {
		return Membership{OrganizationID: org.ID, Member: false}, nil
	}
	return Membership{OrganizationID: org.ID, Member: true, Role: mem.Role}, nil
}

func (m *Module) ListMembers(ctx context.Context, p access.Principal, orgID uuid.UUID) ([]Member, error) {
	org, err := m.loadOrg(m.DB.WithContext(ctx), orgID)
	if err != nil {
		return nil, err
	}
	_, member := m.membershipOf(m.DB.WithContext(ctx), p.AccountID, org.ID)
	if err := AllowViewOrgRoster(p, member); err != nil {
		return nil, err
	}
	var rows []Member
	err = m.DB.WithContext(ctx).Model(&persist.OrganizationMembership{}).
		Select("organization_memberships.account_id, accounts.display_name, organization_memberships.role").
		Joins("JOIN accounts ON accounts.id = organization_memberships.account_id").
		Where("organization_memberships.organization_id = ?", org.ID).
		Order("organization_memberships.created_at").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []Member{}
	}
	return rows, nil
}

func (m *Module) ListJoinRequests(ctx context.Context, p access.Principal, orgID uuid.UUID) ([]JoinRequest, error) {
	org, err := m.loadOrg(m.DB.WithContext(ctx), orgID)
	if err != nil {
		return nil, err
	}
	if err := m.requireOrgManage(m.DB.WithContext(ctx), p, org.ID); err != nil {
		return nil, err
	}
	var rows []JoinRequest
	err = m.DB.WithContext(ctx).Model(&persist.OrganizationJoinRequest{}).
		Select("organization_join_requests.id, organization_join_requests.account_id, accounts.display_name, organization_join_requests.status, organization_join_requests.created_at").
		Joins("JOIN accounts ON accounts.id = organization_join_requests.account_id").
		Where("organization_join_requests.organization_id = ? AND organization_join_requests.status = ?", org.ID, "pending").
		Order("organization_join_requests.created_at").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []JoinRequest{}
	}
	return rows, nil
}

func (m *Module) requireOrgManage(tx *gorm.DB, p access.Principal, orgID uuid.UUID) error {
	mem, ok := m.membershipOf(tx, p.AccountID, orgID)
	return AllowManageOrg(p, ok, domain.OrgRole(mem.Role))
}

func (m *Module) loadOrg(tx *gorm.DB, id uuid.UUID) (persist.Organization, error) {
	var org persist.Organization
	if err := tx.First(&org, "id = ?", id).Error; err != nil {
		return persist.Organization{}, apperr.New(apperr.NotFound, "organization not found")
	}
	return org, nil
}

func (m *Module) membershipOf(tx *gorm.DB, accountID, orgID uuid.UUID) (persist.OrganizationMembership, bool) {
	var mem persist.OrganizationMembership
	err := tx.Where("organization_id = ? AND account_id = ?", orgID, accountID).First(&mem).Error
	return mem, err == nil
}

func (m *Module) Organization(ctx context.Context, id uuid.UUID) (persist.Organization, error) {
	return m.loadOrg(m.DB.WithContext(ctx), id)
}

func (m *Module) BootstrapAdmin(ctx context.Context, email, password string) error {
	if email == "" || password == "" {
		return nil
	}
	var n int64
	if err := m.DB.WithContext(ctx).Model(&persist.Account{}).Where("site_admin = ?", true).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := sec.HashPassword(password)
	if err != nil {
		return err
	}
	now := m.now()
	verified := now
	acc := persist.Account{
		ID: uuid.New(), Email: email, EmailNormalized: sec.NormalizeEmail(email),
		PasswordHash: hash, DisplayName: "Site Admin", EmailVerifiedAt: &verified,
		SiteAdmin: true, CreatedAt: now, UpdatedAt: now,
	}
	return m.DB.WithContext(ctx).Create(&acc).Error
}

func (m *Module) createSession(ctx context.Context, acc persist.Account, ip, ua string) (SessionInfo, error) {
	raw, err := sec.RandomToken(32)
	if err != nil {
		return SessionInfo{}, err
	}
	csrf, err := sec.RandomToken(32)
	if err != nil {
		return SessionInfo{}, err
	}
	now := m.now()
	sess := persist.Session{
		ID: uuid.New(), AccountID: acc.ID, TokenHash: sec.HashToken(raw), CSRFSecret: csrf,
		ExpiresAt: now.Add(sessionTTL), CreatedAt: now, LastSeenAt: now, UserAgent: ua, IP: ip,
	}
	if err := m.DB.WithContext(ctx).Create(&sess).Error; err != nil {
		return SessionInfo{}, err
	}
	return SessionInfo{RawToken: raw, CSRFSecret: csrf, ExpiresAt: sess.ExpiresAt, Account: acc}, nil
}

func (m *Module) issueEmail(ctx context.Context, acc persist.Account, kind string, ttl time.Duration) error {
	raw, err := sec.RandomToken(32)
	if err != nil {
		return err
	}
	now := m.now()
	ch := persist.EmailChallenge{
		ID: uuid.New(), AccountID: acc.ID, Kind: kind, TokenHash: sec.HashToken(raw),
		ExpiresAt: now.Add(ttl), CreatedAt: now,
	}
	if err := m.DB.WithContext(ctx).Create(&ch).Error; err != nil {
		return err
	}
	subject := "Verify your Ephyra email"
	body := "Verification token: " + raw + "\n" + m.PublicURL
	if kind == "reset_password" {
		subject = "Reset your Ephyra password"
		body = "Password reset token: " + raw + "\n" + m.PublicURL
	}
	return m.Mail.Send(ctx, acc.Email, subject, body)
}

func (m *Module) consumeChallenge(ctx context.Context, rawToken, kind string, fn func(*gorm.DB, persist.Account) error) error {
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ch persist.EmailChallenge
		if err := tx.Where("token_hash = ? AND kind = ?", sec.HashToken(rawToken), kind).First(&ch).Error; err != nil {
			return apperr.New(apperr.Invalid, "invalid or expired token")
		}
		now := m.now()
		if ch.ConsumedAt != nil || now.After(ch.ExpiresAt) {
			return apperr.New(apperr.Invalid, "invalid or expired token")
		}
		var acc persist.Account
		if err := tx.First(&acc, "id = ?", ch.AccountID).Error; err != nil {
			return apperr.New(apperr.Invalid, "invalid or expired token")
		}
		if err := tx.Model(&ch).Update("consumed_at", now).Error; err != nil {
			return err
		}
		return fn(tx, acc)
	})
}

func isUnique(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "duplicate key") || strings.Contains(s, "UNIQUE constraint")
}
