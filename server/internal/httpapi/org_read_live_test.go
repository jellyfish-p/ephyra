package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/identity"
	"github.com/jellyfish-p/ephyra/server/internal/judging"
	"github.com/jellyfish-p/ephyra/server/internal/objectstore"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/problem"
	"github.com/jellyfish-p/ephyra/server/internal/testutil"
)

func decodeBody[T any](t *testing.T, body []byte) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode %T: %v (%s)", out, err, body)
	}
	return out
}

func organizationReadAPI(t *testing.T) (*API, testutil.ContestWorld) {
	t.Helper()
	db := testutil.Database(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	world := testutil.SeedRunnableContest(t, db, now)
	fixed := clock.Fixed{T: now}
	return &API{
		Identity: &identity.Module{DB: db, Clock: fixed},
		Problem:  &problem.Module{DB: db, Clock: fixed},
		Judging:  judging.New(db, fixed, objectstore.NewMemory(), "http://localhost"),
	}, world
}

// A member sees their own membership and the site's organization list, but must
// not reach resource administration for an organization they do not manage.
func TestOrganizationReadsLive(t *testing.T) {
	api, world := organizationReadAPI(t)
	r := orgReadRouter(api, world.Principal)

	mine := decodeBody[struct {
		Organizations []identity.MyOrganizationView `json:"organizations"`
	}](t, getContestRead(t, r, "/api/me/organizations", http.StatusOK).Body.Bytes())

	if len(mine.Organizations) != 1 {
		t.Fatalf("expected exactly the seeded membership, got %+v", mine.Organizations)
	}
	got := mine.Organizations[0]
	if got.ID != world.OrgID || got.Role != string(domain.OrgRoleMember) || !got.Member {
		t.Fatalf("membership %+v", got)
	}

	all := decodeBody[struct {
		Organizations []identity.OrganizationView `json:"organizations"`
	}](t, getContestRead(t, r, "/api/organizations", http.StatusOK).Body.Bytes())
	if len(all.Organizations) != 1 || all.Organizations[0].ID != world.OrgID {
		t.Fatalf("organization list %+v", all.Organizations)
	}

	// Managing pools and routing is an administrator action, not a member one.
	orgPath := "/api/organizations/" + world.OrgID.String()
	getContestRead(t, r, orgPath+"/pools", http.StatusForbidden)
	getContestRead(t, r, orgPath+"/routing", http.StatusForbidden)
	// A system pool belongs to the site, so a plain member may not enumerate it.
	getContestRead(t, r, "/api/pools/"+world.PoolID.String()+"/nodes", http.StatusForbidden)
}

func TestOrganizationAdminReadsLive(t *testing.T) {
	api, world := organizationReadAPI(t)
	now := world.Now

	mustReadDB(t, api.Identity.DB.Model(&persist.OrganizationMembership{}).
		Where("organization_id = ? AND account_id = ?", world.OrgID, world.AccountID).
		Update("role", string(domain.OrgRoleAdmin)).Error)

	orgPoolID := uuid.New()
	mustReadDB(t, api.Identity.DB.Create(&persist.JudgePool{
		ID: orgPoolID, Kind: string(domain.PoolOrganization), OrganizationID: &world.OrgID,
		Name: "org", Enabled: true, CreatedAt: now,
	}).Error)

	r := orgReadRouter(api, world.Principal)
	orgPath := "/api/organizations/" + world.OrgID.String()

	pools := decodeBody[struct {
		Pools []judging.PoolView `json:"pools"`
	}](t, getContestRead(t, r, orgPath+"/pools", http.StatusOK).Body.Bytes())

	if len(pools.Pools) != 1 || pools.Pools[0].ID != orgPoolID {
		t.Fatalf("organization admin must see exactly the organization pool, got %+v", pools.Pools)
	}
	if pools.Pools[0].Kind != string(domain.PoolOrganization) || !pools.Pools[0].Enabled {
		t.Fatalf("pool view %+v", pools.Pools[0])
	}
	if pools.Pools[0].OrganizationID == nil || *pools.Pools[0].OrganizationID != world.OrgID {
		t.Fatalf("pool organization id %+v", pools.Pools[0].OrganizationID)
	}

	routing := decodeBody[struct {
		Entries []judging.RoutingEntry `json:"entries"`
	}](t, getContestRead(t, r, orgPath+"/routing", http.StatusOK).Body.Bytes())
	if len(routing.Entries) != 1 || routing.Entries[0].PoolID != world.PoolID || routing.Entries[0].Priority != 1 {
		t.Fatalf("routing %+v", routing.Entries)
	}
}

// Only a site admin may enumerate a system pool's nodes, and the payload must
// describe liveness without ever carrying credential material.
func TestSystemPoolNodeReadsLive(t *testing.T) {
	api, world := organizationReadAPI(t)
	siteAdmin := access.Principal{
		AccountID: world.AccountID, Email: "a@example.com", DisplayName: "A",
		EmailVerified: true, SiteAdmin: true,
	}
	r := orgReadRouter(api, siteAdmin)

	w := getContestRead(t, r, "/api/pools/"+world.PoolID.String()+"/nodes", http.StatusOK)
	nodes := decodeBody[struct {
		Nodes []judging.NodeView `json:"nodes"`
	}](t, w.Body.Bytes())

	if len(nodes.Nodes) != 1 {
		t.Fatalf("expected the seeded node, got %+v", nodes.Nodes)
	}
	node := nodes.Nodes[0]
	if node.ID != world.NodeID || node.Name != "n1" || !node.Connected || !node.Enabled {
		t.Fatalf("node view %+v", node)
	}
	if node.Revoked {
		t.Fatalf("seeded node must not read as revoked: %+v", node)
	}
	if len(node.AdvertisedProfiles) != 1 || node.AdvertisedProfiles[0] != world.Profile {
		t.Fatalf("advertised profiles %+v", node.AdvertisedProfiles)
	}

	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	entries, _ := raw["nodes"].([]any)
	first, _ := entries[0].(map[string]any)
	if _, leaked := first["credential"]; leaked {
		t.Fatalf("node list leaked a credential: %s", w.Body.String())
	}
}
