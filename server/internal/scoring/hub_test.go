package scoring

import (
	"testing"

	"github.com/google/uuid"
)

func TestHubDeliversUpdate(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe()
	defer cancel()
	u := Update{ContestID: uuid.New(), SubmissionID: uuid.New()}
	h.Publish(u)
	got := <-ch
	if got != u {
		t.Fatalf("got %+v want %+v", got, u)
	}
}
