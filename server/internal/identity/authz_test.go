package identity

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

func TestSiteAdminManagesAnyOrg(t *testing.T) {
	p := access.Principal{AccountID: uuid.New(), SiteAdmin: true}
	if err := AllowManageOrg(p, false, ""); err != nil {
		t.Fatal(err)
	}
	if err := AllowViewOrgRoster(p, false); err != nil {
		t.Fatal(err)
	}
	if err := AllowListJoinRequests(p, false, ""); err != nil {
		t.Fatal(err)
	}
}

func TestOwnerAndAdminCanManage(t *testing.T) {
	p := access.Principal{AccountID: uuid.New()}
	if err := AllowManageOrg(p, true, domain.OrgRoleOwner); err != nil {
		t.Fatal(err)
	}
	if err := AllowManageOrg(p, true, domain.OrgRoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := AllowManageOrg(p, true, domain.OrgRoleMember); err == nil || !apperr.Is(err, apperr.Forbidden) {
		t.Fatalf("member must not manage: %v", err)
	}
}

func TestNonMemberDenied(t *testing.T) {
	p := access.Principal{AccountID: uuid.New()}
	if err := AllowManageOrg(p, false, domain.OrgRoleAdmin); err == nil || !apperr.Is(err, apperr.Forbidden) {
		t.Fatalf("got %v", err)
	}
	if err := AllowViewOrgRoster(p, false); err == nil || !apperr.Is(err, apperr.Forbidden) {
		t.Fatalf("got %v", err)
	}
}

func TestMemberCanViewRoster(t *testing.T) {
	p := access.Principal{AccountID: uuid.New()}
	if err := AllowViewOrgRoster(p, true); err != nil {
		t.Fatal(err)
	}
}

func TestUnsignedDenied(t *testing.T) {
	if err := AllowManageOrg(access.Principal{}, true, domain.OrgRoleOwner); err == nil || !apperr.Is(err, apperr.Unauthorized) {
		t.Fatalf("got %v", err)
	}
}
