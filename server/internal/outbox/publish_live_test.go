package outbox

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/testutil"
)

func TestFlushPublishesOnce(t *testing.T) {
	db := testutil.Database(t)
	rdb := testutil.Redis(t)
	streamName := "ephyra.test." + uuid.NewString()
	ev := persist.OutboxEvent{
		ID: uuid.New(), Topic: "judge.task.ready", Payload: []byte(`{"task_id":"x"}`), CreatedAt: time.Now().UTC(),
	}
	if err := db.Create(&ev).Error; err != nil {
		t.Fatal(err)
	}

	pub := &Publisher{DB: db, Redis: rdb, Stream: streamName}
	if err := pub.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := pub.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := db.Model(&persist.OutboxEvent{}).Where("published_at IS NOT NULL").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("published rows %d", n)
	}
	count, err := rdb.XLen(t.Context(), streamName).Result()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("stream entries %d", count)
	}
}
