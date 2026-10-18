package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/contest"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/judging"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/problem"
)

// Choosing the revision a contest pins is an administrator action.
func TestProblemRevisionReadsLive(t *testing.T) {
	api, world := organizationReadAPI(t)

	asMember := orgReadRouter(api, world.Principal)
	getContestRead(t, asMember, "/api/problems/"+world.ProblemID.String()+"/revisions", http.StatusForbidden)

	mustReadDB(t, api.Identity.DB.Model(&persist.OrganizationMembership{}).
		Where("organization_id = ? AND account_id = ?", world.OrgID, world.AccountID).
		Update("role", string(domain.OrgRoleAdmin)).Error)

	w := getContestRead(t, orgReadRouter(api, world.Principal),
		"/api/problems/"+world.ProblemID.String()+"/revisions", http.StatusOK)
	revs := decodeBody[struct {
		Revisions []problem.RevisionSummary `json:"revisions"`
	}](t, w.Body.Bytes())

	if len(revs.Revisions) != 1 {
		t.Fatalf("expected the seeded revision, got %+v", revs.Revisions)
	}
	if revs.Revisions[0].RevisionN != 1 || revs.Revisions[0].RevisionID == uuid.Nil {
		t.Fatalf("revision summary %+v", revs.Revisions[0])
	}
}

// A contest that requires registration review needs an administrator-facing
// entrant list; without it the pending queue cannot be worked through.
func TestContestEntrantReadsLive(t *testing.T) {
	api, world := organizationReadAPI(t)
	path := "/api/contests/" + world.ContestID.String() + "/entrants"

	getContestRead(t, orgReadRouter(api, world.Principal), path, http.StatusForbidden)

	mustReadDB(t, api.Identity.DB.Model(&persist.OrganizationMembership{}).
		Where("organization_id = ? AND account_id = ?", world.OrgID, world.AccountID).
		Update("role", string(domain.OrgRoleAdmin)).Error)

	r := orgReadRouter(api, world.Principal)
	ens := decodeBody[struct {
		Entrants []contest.MyEntrantView `json:"entrants"`
	}](t, getContestRead(t, r, path, http.StatusOK).Body.Bytes())

	if len(ens.Entrants) != 1 {
		t.Fatalf("expected the seeded entrant, got %+v", ens.Entrants)
	}
	entrant := ens.Entrants[0]
	if entrant.ContestID != world.ContestID || entrant.Status != string(domain.EntrantAccepted) {
		t.Fatalf("entrant view %+v", entrant)
	}
	if len(entrant.Members) != 1 || entrant.Members[0].AccountID != world.AccountID || !entrant.Members[0].Confirmed {
		t.Fatalf("roster %+v", entrant.Members)
	}

	pending := decodeBody[struct {
		Entrants []contest.MyEntrantView `json:"entrants"`
	}](t, getContestRead(t, r, path+"?status="+string(domain.EntrantPending), http.StatusOK).Body.Bytes())
	if len(pending.Entrants) != 0 {
		t.Fatalf("status filter ignored: %+v", pending.Entrants)
	}
}

// A task whose node vanished is the administrator's to redispatch, so the admin
// view has to surface the task and its latest attempt.
func TestContestTaskReadsLive(t *testing.T) {
	api, world := organizationReadAPI(t)
	now := world.Now

	var cp persist.ContestProblem
	mustReadDB(t, api.Identity.DB.First(&cp, "contest_id = ?", world.ContestID).Error)

	taskID, attemptID := uuid.New(), uuid.New()
	mustReadDB(t, api.Identity.DB.Create(&persist.JudgeTask{
		ID: taskID, SubmissionID: uuid.New(), ContestID: world.ContestID,
		JudgeDataRevisionID: cp.ScoringRevisionID, ExecutionProfileID: world.Profile,
		Purpose: "initial", State: string(domain.TaskAwaitingAdmin), CreatedAt: now,
	}).Error)
	mustReadDB(t, api.Identity.DB.Create(&persist.JudgingAttempt{
		ID: attemptID, TaskID: taskID, Generation: 1, NodeID: world.NodeID, PoolID: world.PoolID,
		JudgeDataRevisionID: cp.ScoringRevisionID, ExecutionProfileID: world.Profile,
		State: string(domain.AttemptDispatched), DispatchedAt: now, LeaseUntil: now.Add(time.Minute),
		SystemFault: true, SystemFaultKind: "node_lost",
	}).Error)

	path := "/api/contests/" + world.ContestID.String() + "/tasks"
	getContestRead(t, orgReadRouter(api, world.Principal), path, http.StatusForbidden)

	mustReadDB(t, api.Identity.DB.Model(&persist.OrganizationMembership{}).
		Where("organization_id = ? AND account_id = ?", world.OrgID, world.AccountID).
		Update("role", string(domain.OrgRoleAdmin)).Error)

	r := orgReadRouter(api, world.Principal)
	tasks := decodeBody[struct {
		Tasks []judging.TaskView `json:"tasks"`
	}](t, getContestRead(t, r, path, http.StatusOK).Body.Bytes())

	if len(tasks.Tasks) != 1 {
		t.Fatalf("expected the seeded task, got %+v", tasks.Tasks)
	}
	task := tasks.Tasks[0]
	if task.ID != taskID || task.State != string(domain.TaskAwaitingAdmin) {
		t.Fatalf("task view %+v", task)
	}
	if task.Attempt == nil || task.Attempt.ID != attemptID || !task.Attempt.SystemFault {
		t.Fatalf("attempt view %+v", task.Attempt)
	}

	// The state filter must narrow the list, and an unmatched state must be empty.
	filtered := decodeBody[struct {
		Tasks []judging.TaskView `json:"tasks"`
	}](t, getContestRead(t, r, path+"?state="+string(domain.TaskCompleted), http.StatusOK).Body.Bytes())
	if len(filtered.Tasks) != 0 {
		t.Fatalf("state filter ignored: %+v", filtered.Tasks)
	}
}
