package outbox

import "github.com/jellyfish-p/ephyra/server/internal/persist"

// StreamFields publishes a pointer to an already-persisted outbox row.
// The stream is a delivery hint, not a place to create judging attempts.
func StreamFields(ev persist.OutboxEvent) map[string]any {
	return map[string]any{
		"event_id": ev.ID.String(),
		"topic":    ev.Topic,
		"payload":  string(ev.Payload),
	}
}
