package submission

import (
	"testing"
	"time"

	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/objectstore"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/testutil"
)

func TestSubmitWritesSubmissionTaskAndOutboxAtomically(t *testing.T) {
	db := testutil.Database(t)
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	world := testutil.SeedRunnableContest(t, db, now)
	mod := &Module{DB: db, Clock: clock.Fixed{T: now}, Blobs: objectstore.NewMemory()}

	got, err := mod.Submit(t.Context(), world.Principal, world.ContestID, world.Letter, "cpp23", []byte("int main(){}"))
	if err != nil {
		t.Fatal(err)
	}
	if got.SubmissionID.String() == "" || got.TaskID.String() == "" || got.ReceiveSeq == 0 {
		t.Fatalf("%+v", got)
	}

	var subs, tasks, events int64
	if err := db.Model(&persist.Submission{}).Count(&subs).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&persist.JudgeTask{}).Count(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&persist.OutboxEvent{}).Count(&events).Error; err != nil {
		t.Fatal(err)
	}
	if subs != 1 || tasks != 1 || events != 1 {
		t.Fatalf("counts sub=%d task=%d outbox=%d", subs, tasks, events)
	}

	var task persist.JudgeTask
	if err := db.First(&task, "id = ?", got.TaskID).Error; err != nil {
		t.Fatal(err)
	}
	if task.SubmissionID != got.SubmissionID {
		t.Fatalf("task %s not bound to submission %s", task.ID, got.SubmissionID)
	}
	var ev persist.OutboxEvent
	if err := db.First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	if ev.PublishedAt != nil {
		t.Fatal("outbox must still be unpublished")
	}
	if ev.Topic != "judge.task.ready" {
		t.Fatalf("topic %s", ev.Topic)
	}
}
