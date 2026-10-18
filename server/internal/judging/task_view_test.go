package judging

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Administrators redispatch tasks whose node was lost, so the view has to carry
// the attempt's dispatch state while never exposing the raw judge result.
func TestTaskViewExposesAttemptBookkeepingWithoutResults(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	finished := now.Add(2 * time.Minute)
	task := TaskView{
		ID: uuid.New(), SubmissionID: uuid.New(), State: "awaiting_admin", Purpose: "initial",
		RevisionID: uuid.New(), CreatedAt: now,
		Attempt: &AttemptView{
			ID: uuid.New(), Generation: 1, NodeID: uuid.New(), PoolID: uuid.New(),
			State: "dispatched", DispatchedAt: now, LeaseUntil: now.Add(time.Minute),
			FinishedAt: &finished, SystemFault: true,
		},
	}
	body, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{
		"id", "submission_id", "state", "purpose", "revision_id", "created_at", "attempt",
	} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing %q in %s", key, body)
		}
	}
	attempt, ok := decoded["attempt"].(map[string]any)
	if !ok {
		t.Fatalf("attempt %#v", decoded["attempt"])
	}
	for _, key := range []string{
		"id", "generation", "node_id", "pool_id", "state",
		"dispatched_at", "lease_until", "finished_at", "system_fault",
	} {
		if _, ok := attempt[key]; !ok {
			t.Fatalf("missing attempt key %q in %s", key, body)
		}
	}
	// The raw judge result lives on the attempt row but is not part of the admin view.
	for _, leaked := range []string{"result", "Result", "verdict", "score_hundredths"} {
		if _, ok := attempt[leaked]; ok {
			t.Fatalf("attempt leaked %q: %s", leaked, body)
		}
	}
}

func TestTaskViewAttemptIsNullable(t *testing.T) {
	body, err := json.Marshal(TaskView{ID: uuid.New(), State: "pending_schedule"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded struct {
		Attempt *AttemptView `json:"attempt"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Attempt != nil {
		t.Fatalf("unassigned task must report a null attempt: %s", body)
	}
}
