package judging

import (
	"context"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
)

func (m *Module) EnsureDefaultRouting(ctx context.Context, orgID uuid.UUID) error {
	var n int64
	if err := m.DB.WithContext(ctx).Model(&persist.RoutingPolicyVersion{}).Where("organization_id = ?", orgID).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	var sys persist.JudgePool
	if err := m.DB.WithContext(ctx).Where("kind = ?", domain.PoolSystem).First(&sys).Error; err != nil {
		return err
	}
	ver := persist.RoutingPolicyVersion{ID: uuid.New(), OrganizationID: orgID, VersionN: 1, CreatedAt: m.now()}
	if err := m.DB.WithContext(ctx).Create(&ver).Error; err != nil {
		return err
	}
	ent := persist.RoutingPolicyEntry{PolicyVersionID: ver.ID, PoolID: sys.ID, Priority: 1}
	return m.DB.WithContext(ctx).Create(&ent).Error
}
