package identity

import (
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

// AllowManageOrg is the organization-admin check after the org has been loaded.
// Site admins may manage any organization; callers must not trust a client org_id.
func AllowManageOrg(p access.Principal, member bool, role domain.OrgRole) error {
	if p.Zero() {
		return apperr.New(apperr.Unauthorized, "not signed in")
	}
	if p.SiteAdmin {
		return nil
	}
	if !member {
		return apperr.New(apperr.Forbidden, "not an organization member")
	}
	if !role.CanManageOrg() {
		return apperr.New(apperr.Forbidden, "insufficient organization role")
	}
	return nil
}

// AllowViewOrgRoster allows listing members after the org has been loaded.
func AllowViewOrgRoster(p access.Principal, member bool) error {
	if p.Zero() {
		return apperr.New(apperr.Unauthorized, "not signed in")
	}
	if p.SiteAdmin || member {
		return nil
	}
	return apperr.New(apperr.Forbidden, "not an organization member")
}

// AllowListJoinRequests is the admin queue check after the org has been loaded.
func AllowListJoinRequests(p access.Principal, member bool, role domain.OrgRole) error {
	return AllowManageOrg(p, member, role)
}
