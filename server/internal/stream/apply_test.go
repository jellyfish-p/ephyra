package stream

import "testing"

func TestStreamNeverCreatesAttempt(t *testing.T) {
	for _, first := range []bool{true, false} {
		got := ApplyEvent(first)
		if got.CreateAttempt {
			t.Fatalf("firstSeen=%v must not create a judging attempt", first)
		}
		if !got.RunSchedule {
			t.Fatalf("firstSeen=%v should still notify the scheduler", first)
		}
	}
}
