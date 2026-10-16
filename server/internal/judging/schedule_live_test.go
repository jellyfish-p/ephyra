package judging

import (
	"sync"
	"testing"
	"time"

	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/objectstore"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/submission"
	"github.com/jellyfish-p/ephyra/server/internal/testutil"
)

func TestScheduleAssignsAtMostOneSlot(t *testing.T) {
	db := testutil.Database(t)
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	world := testutil.SeedRunnableContest(t, db, now)
	blobs := objectstore.NewMemory()
	sub := &submission.Module{DB: db, Clock: clock.Fixed{T: now}, Blobs: blobs}
	jg := New(db, clock.Fixed{T: now}, blobs, "http://localhost")

	for i := 0; i < 2; i++ {
		src := []byte("int main(){ return " + string(rune('0'+i)) + "; }")
		if _, err := sub.Submit(t.Context(), world.Principal, world.ContestID, world.Letter, "cpp23", src); err != nil {
			t.Fatal(err)
		}
	}

	conn := NewConn(world.NodeID, []string{world.Profile}, 1)
	jg.Hub.Register(conn)
	defer jg.Hub.Unregister(world.NodeID, conn)
	go func() {
		for {
			select {
			case <-conn.Messages():
			case <-conn.Done():
				return
			}
		}
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	errCh := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			errCh <- jg.Schedule(t.Context())
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	var dispatched int64
	if err := db.Model(&persist.JudgingAttempt{}).Where("state = ?", domain.AttemptDispatched).Count(&dispatched).Error; err != nil {
		t.Fatal(err)
	}
	if dispatched != 1 {
		t.Fatalf("dispatched attempts %d", dispatched)
	}
	var pending int64
	if err := db.Model(&persist.JudgeTask{}).Where("state = ?", domain.TaskPendingSchedule).Count(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("pending tasks %d", pending)
	}
}
