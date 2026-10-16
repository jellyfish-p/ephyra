package testutil

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/execution"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"gorm.io/gorm"
)

const MinimalConfigJSON = `{"version":1,"kind":"batch","checker":{"kind":"exact"},"time_limit_ms":1000,"memory_limit_bytes":268435456,"subtasks":[{"id":"s1","score":"100.00","case_ids":["1"]}],"cases":[{"id":"1","input_name":"1.in","output_name":"1.out"}]}`

type ContestWorld struct {
	Now       time.Time
	AccountID uuid.UUID
	Principal access.Principal
	OrgID     uuid.UUID
	ContestID uuid.UUID
	ProblemID uuid.UUID
	Letter    string
	PoolID    uuid.UUID
	NodeID    uuid.UUID
	Profile   string
}

func SeedRunnableContest(t *testing.T, db *gorm.DB, now time.Time) ContestWorld {
	t.Helper()
	accountID := uuid.New()
	orgID := uuid.New()
	contestID := uuid.New()
	problemID := uuid.New()
	revID := uuid.New()
	cpID := uuid.New()
	entrantID := uuid.New()
	poolID := uuid.New()
	nodeID := uuid.New()
	policyID := uuid.New()
	accepted := now.Add(-time.Hour)
	heartbeat := now
	w := ContestWorld{
		Now:       now,
		AccountID: accountID,
		Principal: access.Principal{AccountID: accountID, Email: "a@example.com", DisplayName: "A", EmailVerified: true},
		OrgID:     orgID,
		ContestID: contestID,
		ProblemID: problemID,
		Letter:    "A",
		PoolID:    poolID,
		NodeID:    nodeID,
		Profile:   execution.CPP23,
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.Create(&persist.Account{
		ID: accountID, Email: "a@example.com", EmailNormalized: "a@example.com",
		PasswordHash: "x", DisplayName: "A", EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now,
	}).Error)
	must(db.Create(&persist.Organization{ID: orgID, Slug: "org-" + orgID.String()[:8], Name: "Org", CreatedAt: now}).Error)
	must(db.Create(&persist.OrganizationMembership{
		ID: uuid.New(), OrganizationID: orgID, AccountID: accountID, Role: string(domain.OrgRoleMember), CreatedAt: now,
	}).Error)
	must(db.Create(&persist.Problem{ID: problemID, OrganizationID: orgID, Slug: "p", Title: "P", CreatedAt: now}).Error)
	must(db.Create(&persist.JudgeDataRevision{
		ID: revID, ProblemID: problemID, RevisionN: 1,
		NormalizedConfig: json.RawMessage(MinimalConfigJSON), CreatedAt: now, CreatedBy: accountID,
	}).Error)
	must(db.Create(&persist.Contest{
		ID: contestID, OrganizationID: orgID, Slug: "c-" + contestID.String()[:8], Title: "C",
		Rule: string(domain.RuleOI), EntryMode: string(domain.EntryIndividual),
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), RegisterStartsAt: now.Add(-2 * time.Hour),
		MaxTeamSize: 1, ACMPenaltyMinutes: domain.DefaultACMPenaltyMinutes, CaseFeedback: string(domain.FeedbackSubtasks),
		CreatedAt: now,
	}).Error)
	must(db.Create(&persist.ContestProblem{
		ID: cpID, ContestID: contestID, ProblemID: problemID, Letter: "A",
		ScoringRevisionID: revID, TargetRevisionID: revID,
	}).Error)
	must(db.Create(&persist.Entrant{
		ID: entrantID, ContestID: contestID, Kind: string(domain.EntryIndividual),
		DisplayName: "A", Status: string(domain.EntrantAccepted), RegisteredAt: accepted, AcceptedAt: &accepted,
	}).Error)
	must(db.Create(&persist.EntrantMember{
		ID: uuid.New(), ContestID: contestID, EntrantID: entrantID, AccountID: accountID, Role: "owner", ConfirmedAt: &accepted,
	}).Error)
	must(db.Create(&persist.JudgePool{
		ID: poolID, Kind: string(domain.PoolSystem), Name: "system", Enabled: true, CreatedAt: now,
	}).Error)
	must(db.Create(&persist.JudgeNode{
		ID: nodeID, PoolID: poolID, Name: "n1", CredentialHash: "x", Enabled: true,
		MaxSlots: 1, LastHeartbeatAt: &heartbeat, Connected: true,
		AdvertisedProfiles: persist.JSONList{execution.CPP23}, CreatedAt: now,
	}).Error)
	must(db.Create(&persist.RoutingPolicyVersion{ID: policyID, OrganizationID: orgID, VersionN: 1, CreatedAt: now}).Error)
	must(db.Create(&persist.RoutingPolicyEntry{PolicyVersionID: policyID, PoolID: poolID, Priority: 1}).Error)
	return w
}
