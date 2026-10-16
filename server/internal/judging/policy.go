package judging

import "github.com/jellyfish-p/ephyra/server/internal/domain"

// DispatchSendFailure is the post-commit action when a dispatch message
// cannot be written to the node's stream. Postgres remains the authority:
// the attempt is invalidated and the task waits for an admin. A new
// attempt is never created from the send failure itself.
func DispatchSendFailure() (attempt domain.AttemptState, task domain.TaskState, faultKind string) {
	return domain.AttemptInvalidated, domain.TaskAwaitingAdmin, "send_failed"
}

// HeartbeatInvalidations returns attempt ids this node still reports as
// active whose database state is already invalidated. Omitted dispatched
// ids are ignored: they do not create a new attempt and do not redispatch
// awaiting_admin or scheduling_error work.
func HeartbeatInvalidations(reportedActive, dbInvalidated []string) []string {
	inv := make(map[string]struct{}, len(dbInvalidated))
	for _, id := range dbInvalidated {
		inv[id] = struct{}{}
	}
	out := make([]string, 0)
	seen := make(map[string]struct{})
	for _, id := range reportedActive {
		if _, ok := inv[id]; !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// HeartbeatMayCreateAttempt reports whether omitting a dispatched attempt
// from a heartbeat should mint a new judging attempt. ADR-0003 forbids this.
func HeartbeatMayCreateAttempt(omittedDispatched bool) bool {
	return false
}

// HeartbeatMayRedispatch reports whether a heartbeat may move a stuck task
// back to pending_schedule. Lost or failed work waits for an admin.
func HeartbeatMayRedispatch(state domain.TaskState) bool {
	switch state {
	case domain.TaskAwaitingAdmin, domain.TaskSchedulingError:
		return false
	default:
		return false
	}
}

// RedispatchPlan is the admin-only effect of asking to run a task again.
// A new attempt generation is created later at dispatch (max(generation)+1).
// The original submission and its receive_seq / received_at are untouched.
type RedispatchPlan struct {
	TaskState            domain.TaskState
	InvalidateDispatched bool
	NewAttemptAtDispatch bool
	NewSubmission        bool
	RewriteReceiveSeq    bool
}

func PlanRedispatch() RedispatchPlan {
	return RedispatchPlan{
		TaskState:            domain.TaskPendingSchedule,
		InvalidateDispatched: true,
		NewAttemptAtDispatch: true,
		NewSubmission:        false,
		RewriteReceiveSeq:    false,
	}
}

// InRejudgeSnapshot is whether a submission belongs to the batch frozen at
// creation. Later submissions bind the target revision but are not inserted
// into this batch.
func InRejudgeSnapshot(receiveSeq, snapshotMaxReceiveSeq int64) bool {
	return receiveSeq <= snapshotMaxReceiveSeq
}

// NewSubmissionJoinsOpenBatch is always false: a submission received after
// the snapshot uses target_revision_id on its own initial task.
func NewSubmissionJoinsOpenBatch() bool {
	return false
}
