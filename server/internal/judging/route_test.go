package judging

import (
	"testing"

	"github.com/google/uuid"
)

func TestHighPriorityBusyDoesNotFallThrough(t *testing.T) {
	hi, lo := uuid.New(), uuid.New()
	dec, err := SelectPool([]PoolSnapshot{
		{ID: hi, Priority: 10, Enabled: true, HasProfile: true, HealthyNodes: 1, FreeSlots: 0},
		{ID: lo, Priority: 1, Enabled: true, HasProfile: true, HealthyNodes: 1, FreeSlots: 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !dec.Wait || dec.PoolID != uuid.Nil {
		t.Fatalf("must wait on high priority, got %+v", dec)
	}
}

func TestUnavailableHighFallsThrough(t *testing.T) {
	hi, lo := uuid.New(), uuid.New()
	dec, err := SelectPool([]PoolSnapshot{
		{ID: hi, Priority: 10, Enabled: true, Offline: true, HasProfile: true, HealthyNodes: 0, FreeSlots: 0},
		{ID: lo, Priority: 1, Enabled: true, HasProfile: true, HealthyNodes: 1, FreeSlots: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.PoolID != lo {
		t.Fatalf("got %s", dec.PoolID)
	}
}

func TestAllUnavailableIsError(t *testing.T) {
	_, err := SelectPool([]PoolSnapshot{
		{ID: uuid.New(), Priority: 5, Enabled: false, HasProfile: true, HealthyNodes: 1, FreeSlots: 3},
		{ID: uuid.New(), Priority: 1, Enabled: true, HasProfile: false, HealthyNodes: 1, FreeSlots: 3},
	})
	if err == nil {
		t.Fatal("expected scheduling error")
	}
}

func TestSamePriorityPrefersMoreSlots(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	dec, err := SelectPool([]PoolSnapshot{
		{ID: a, Priority: 5, Enabled: true, HasProfile: true, HealthyNodes: 1, FreeSlots: 1},
		{ID: b, Priority: 5, Enabled: true, HasProfile: true, HealthyNodes: 1, FreeSlots: 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec.PoolID != b {
		t.Fatal(dec.PoolID)
	}
}
