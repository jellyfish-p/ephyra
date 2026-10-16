package judging

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

func TestOrderPendingWorkByReceiveSeq(t *testing.T) {
	orgA, orgB := uuid.New(), uuid.New()
	lateA := PendingWork{TaskID: uuid.New(), OrgID: orgA, ReceiveSeq: 30}
	earlyA := PendingWork{TaskID: uuid.New(), OrgID: orgA, ReceiveSeq: 4}
	midB := PendingWork{TaskID: uuid.New(), OrgID: orgB, ReceiveSeq: 10}
	got := OrderPendingWork([]PendingWork{lateA, midB, earlyA})
	if len(got) != 3 || got[0].ReceiveSeq != 4 || got[1].ReceiveSeq != 10 || got[2].ReceiveSeq != 30 {
		t.Fatalf("expected receive_seq order 4,10,30 got %+v", got)
	}
	sameOrg := OrderPendingWork([]PendingWork{lateA, earlyA})
	if sameOrg[0].TaskID != earlyA.TaskID {
		t.Fatal("within an organization earlier received work must dispatch first")
	}
}

func TestDispatchSendFailureWaitsForAdmin(t *testing.T) {
	att, task, kind := DispatchSendFailure()
	if att != domain.AttemptInvalidated || task != domain.TaskAwaitingAdmin || kind != "send_failed" {
		t.Fatalf("got %s %s %s", att, task, kind)
	}
}

func TestHeartbeatInvalidationsOnlyReportedActive(t *testing.T) {
	live := uuid.New().String()
	stale := uuid.New().String()
	got := HeartbeatInvalidations([]string{live, uuid.New().String()}, []string{live, stale})
	if len(got) != 1 || got[0] != live {
		t.Fatalf("got %v", got)
	}
}

func TestHeartbeatDoesNotCreateOrRedispatch(t *testing.T) {
	if HeartbeatMayCreateAttempt(true) {
		t.Fatal("omitting an attempt id must not create a new attempt")
	}
	for _, st := range []domain.TaskState{domain.TaskAwaitingAdmin, domain.TaskSchedulingError, domain.TaskDispatched} {
		if HeartbeatMayRedispatch(st) {
			t.Fatalf("heartbeat must not redispatch %s", st)
		}
	}
}

func TestRedispatchDoesNotRewriteSubmission(t *testing.T) {
	p := PlanRedispatch()
	if p.NewSubmission || p.RewriteReceiveSeq || !p.NewAttemptAtDispatch || !p.InvalidateDispatched {
		t.Fatalf("got %+v", p)
	}
	if p.TaskState != domain.TaskPendingSchedule {
		t.Fatal(p.TaskState)
	}
}

func TestRejudgeSnapshotExcludesLaterSubmissions(t *testing.T) {
	if !InRejudgeSnapshot(4, 4) || !InRejudgeSnapshot(1, 4) {
		t.Fatal("snapshot members must be included")
	}
	if InRejudgeSnapshot(5, 4) {
		t.Fatal("a later submission must not join the old batch")
	}
	if NewSubmissionJoinsOpenBatch() {
		t.Fatal("new submissions bind target_revision_id on their own task")
	}
}
