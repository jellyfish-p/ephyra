package judging

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// The frontend both reads and writes routing entries, so the JSON shape has to
// round-trip through the same snake_case keys in both directions.
func TestRoutingEntryJSONRoundTrip(t *testing.T) {
	poolID := uuid.New()
	body, err := json.Marshal([]RoutingEntry{{PoolID: poolID, Priority: 3}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(body) != `[{"pool_id":"`+poolID.String()+`","priority":3}]` {
		t.Fatalf("unexpected encoding: %s", body)
	}

	var decoded []RoutingEntry
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded) != 1 || decoded[0].PoolID != poolID || decoded[0].Priority != 3 {
		t.Fatalf("round trip lost data: %+v", decoded)
	}
}

// A node view must never carry credential material, only liveness metadata.
func TestNodeViewNeverExposesCredential(t *testing.T) {
	body, err := json.Marshal(NodeView{ID: uuid.New(), Name: "judge-01", MaxSlots: 2})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, leaked := range []string{"credential", "Credential", "CredentialHash", "credential_hash"} {
		if _, ok := decoded[leaked]; ok {
			t.Fatalf("node view leaked %q: %s", leaked, body)
		}
	}
	for _, key := range []string{
		"id", "pool_id", "name", "enabled", "connected", "revoked",
		"max_slots", "last_heartbeat_at", "advertised_profiles",
	} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing %q in %s", key, body)
		}
	}
}
