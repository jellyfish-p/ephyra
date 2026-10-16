package problem

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/objectstore"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"gorm.io/gorm"
)

type Module struct {
	DB    *gorm.DB
	Clock clock.Clock
	Blobs objectstore.Store
}

func (m *Module) ImportHydro(ctx context.Context, p access.Principal, orgID uuid.UUID, slug, title string, zip []byte) (persist.Problem, persist.JudgeDataRevision, error) {
	if err := m.requireManage(ctx, p, orgID); err != nil {
		return persist.Problem{}, persist.JudgeDataRevision{}, err
	}
	pkg, err := ImportHydroZip(zip)
	if err != nil {
		return persist.Problem{}, persist.JudgeDataRevision{}, err
	}
	slug = strings.ToLower(strings.TrimSpace(slug))
	title = strings.TrimSpace(title)
	if slug == "" || title == "" {
		return persist.Problem{}, persist.JudgeDataRevision{}, apperr.New(apperr.Invalid, "slug and title are required")
	}
	cfgJSON, err := json.Marshal(pkg.Config)
	if err != nil {
		return persist.Problem{}, persist.JudgeDataRevision{}, err
	}
	now := m.Clock.Now().UTC()
	prob := persist.Problem{ID: uuid.New(), OrganizationID: orgID, Slug: slug, Title: title, CreatedAt: now}
	rev := persist.JudgeDataRevision{
		ID: uuid.New(), ProblemID: prob.ID, RevisionN: 1, NormalizedConfig: cfgJSON,
		CreatedAt: now, CreatedBy: p.AccountID,
	}
	err = m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&prob).Error; err != nil {
			return apperr.New(apperr.Conflict, "problem slug already used")
		}
		if err := tx.Create(&rev).Error; err != nil {
			return err
		}
		return m.writeFiles(ctx, tx, rev.ID, pkg)
	})
	return prob, rev, err
}

func (m *Module) AddRevision(ctx context.Context, p access.Principal, problemID uuid.UUID, zip []byte) (persist.JudgeDataRevision, error) {
	pkg, err := ImportHydroZip(zip)
	if err != nil {
		return persist.JudgeDataRevision{}, err
	}
	cfgJSON, err := json.Marshal(pkg.Config)
	if err != nil {
		return persist.JudgeDataRevision{}, err
	}
	var rev persist.JudgeDataRevision
	err = m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var prob persist.Problem
		if err := tx.First(&prob, "id = ?", problemID).Error; err != nil {
			return apperr.New(apperr.NotFound, "problem not found")
		}
		if err := m.requireManage(ctx, p, prob.OrganizationID); err != nil {
			return err
		}
		var maxN int
		if err := tx.Model(&persist.JudgeDataRevision{}).Where("problem_id = ?", problemID).
			Select("COALESCE(MAX(revision_n), 0)").Scan(&maxN).Error; err != nil {
			return err
		}
		now := m.Clock.Now().UTC()
		rev = persist.JudgeDataRevision{
			ID: uuid.New(), ProblemID: problemID, RevisionN: maxN + 1, NormalizedConfig: cfgJSON,
			CreatedAt: now, CreatedBy: p.AccountID,
		}
		if err := tx.Create(&rev).Error; err != nil {
			return err
		}
		return m.writeFiles(ctx, tx, rev.ID, pkg)
	})
	return rev, err
}

func (m *Module) writeFiles(ctx context.Context, tx *gorm.DB, revisionID uuid.UUID, pkg HydroPackage) error {
	for _, f := range pkg.Files {
		obj, err := m.putBlob(ctx, tx, f.Data, contentType(f.Name))
		if err != nil {
			return err
		}
		role := "data"
		base := path.Base(f.Name)
		if pkg.Config.Checker.SourceName != "" && base == pkg.Config.Checker.SourceName {
			role = "checker"
		}
		if pkg.Config.Interactor != nil && base == pkg.Config.Interactor.SourceName {
			role = "interactor"
		}
		rf := persist.RevisionFile{
			ID: uuid.New(), RevisionID: revisionID, Role: role, Name: base, ObjectID: obj.ID,
		}
		if err := tx.Create(&rf).Error; err != nil {
			return err
		}
	}
	return nil
}

func (m *Module) putBlob(ctx context.Context, tx *gorm.DB, data []byte, ctype string) (persist.Object, error) {
	sum := sha256.Sum256(data)
	hexSum := hex.EncodeToString(sum[:])
	var existing persist.Object
	if err := tx.Where("sha256 = ? AND size_bytes = ?", hexSum, int64(len(data))).First(&existing).Error; err == nil {
		return existing, nil
	}
	key := "sha256/" + hexSum
	if err := m.Blobs.Put(ctx, key, bytes.NewReader(data), int64(len(data)), ctype); err != nil {
		return persist.Object{}, err
	}
	obj := persist.Object{
		ID: uuid.New(), SHA256: hexSum, SizeBytes: int64(len(data)),
		StorageKey: key, ContentType: ctype, CreatedAt: m.Clock.Now().UTC(),
	}
	if err := tx.Create(&obj).Error; err != nil {
		if err := tx.Where("sha256 = ? AND size_bytes = ?", hexSum, int64(len(data))).First(&existing).Error; err == nil {
			return existing, nil
		}
		return persist.Object{}, err
	}
	return obj, nil
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

func contentType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".cpp", ".cc", ".c", ".h", ".txt", ".in", ".out", ".ans", ".md":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}
