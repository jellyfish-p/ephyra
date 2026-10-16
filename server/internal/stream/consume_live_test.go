package stream

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/outbox"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/testutil"
)

func TestConsumeIsIdempotentAcrossRestart(t *testing.T) {
	db := testutil.Database(t)
	rdb := testutil.Redis(t)
	streamName := "ephyra.test." + uuid.NewString()
	ev := persist.OutboxEvent{
		ID: uuid.New(), Topic: "judge.task.ready", Payload: []byte(`{"task_id":"x"}`), CreatedAt: time.Now().UTC(),
	}
	if err := db.Create(&ev).Error; err != nil {
		t.Fatal(err)
	}
	pub := &outbox.Publisher{DB: db, Redis: rdb, Stream: streamName}
	if err := pub.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}

	var runs atomic.Int32
	group := "g-" + uuid.NewString()
	start := func(ctx context.Context, name string) {
		cons := &Consumer{
			DB: db, Redis: rdb, Stream: streamName, Group: group, Consumer: name,
			OnReady: func(context.Context) error {
				runs.Add(1)
				return nil
			},
		}
		_ = cons.Run(ctx)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	go start(ctx, "c1")
	deadline := time.Now().Add(2 * time.Second)
	for runs.Load() < 1 {
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("consumer never applied the event")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()

	ctx2, cancel2 := context.WithTimeout(t.Context(), time.Second)
	defer cancel2()
	start(ctx2, "c2")
	if runs.Load() != 1 {
		t.Fatalf("duplicate apply after restart %d", runs.Load())
	}
}
