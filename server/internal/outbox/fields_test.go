package outbox

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
)

func TestStreamFieldsReferencePersistedEvent(t *testing.T) {
	id := uuid.New()
	ev := persist.OutboxEvent{ID: id, Topic: "judge.task.ready", Payload: []byte(`{"task_id":"x"}`)}
	fields := StreamFields(ev)
	if fields["event_id"] != id.String() {
		t.Fatalf("got %v", fields["event_id"])
	}
	if fields["topic"] != "judge.task.ready" {
		t.Fatal(fields["topic"])
	}
	if _, ok := fields["attempt_id"]; ok {
		t.Fatal("outbox must not invent an attempt id")
	}
}
