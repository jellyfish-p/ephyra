package rate

import (
	"testing"
	"time"
)

func TestAllowWindow(t *testing.T) {
	l := New()
	t0 := time.Unix(1_700_000_000, 0)
	window := time.Minute

	if !l.allow("k", 2, window, t0) {
		t.Fatal("first")
	}
	if !l.allow("k", 2, window, t0.Add(time.Second)) {
		t.Fatal("second")
	}
	if l.allow("k", 2, window, t0.Add(2*time.Second)) {
		t.Fatal("third inside window")
	}
	if !l.allow("other", 1, window, t0.Add(2*time.Second)) {
		t.Fatal("other key")
	}
	if !l.allow("k", 2, window, t0.Add(window+time.Second)) {
		t.Fatal("after window")
	}
}
