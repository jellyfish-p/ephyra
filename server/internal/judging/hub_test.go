package judging

import (
	"sync"
	"testing"

	"github.com/google/uuid"
	judgev1 "github.com/jellyfish-p/ephyra/protocol/judge/v1"
)

func TestDispatchAfterUnregisterDoesNotPanic(t *testing.T) {
	h := NewHub()
	id := uuid.New()
	c := NewConn(id, nil, 1)
	h.Register(c)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 10_000; i++ {
			h.Dispatch(id, &judgev1.ServerMessage{})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			h.Unregister(id, c)
			h.Register(c)
		}
		h.Unregister(id, c)
	}()
	wg.Wait()
}

func TestRegisterReplacementRetiresOldConn(t *testing.T) {
	h := NewHub()
	id := uuid.New()
	old := NewConn(id, nil, 1)
	next := NewConn(id, nil, 2)
	h.Register(old)
	h.Register(next)

	select {
	case <-old.Done():
	default:
		t.Fatal("replaced connection must be marked done")
	}
	if !h.Dispatch(id, &judgev1.ServerMessage{}) {
		t.Fatal("dispatch should reach the replacement")
	}
	if old.TrySend(&judgev1.ServerMessage{}) {
		t.Fatal("retired connection must refuse send")
	}
}

func TestDispatchUnknownNode(t *testing.T) {
	h := NewHub()
	if h.Dispatch(uuid.New(), &judgev1.ServerMessage{}) {
		t.Fatal("missing node")
	}
}
