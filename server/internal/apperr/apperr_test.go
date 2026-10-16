package apperr

import (
	"fmt"
	"testing"
)

func TestAsUnwraps(t *testing.T) {
	inner := New(NotFound, "contest not found")
	err := fmt.Errorf("load: %w", inner)
	got, ok := As(err)
	if !ok || got.Code != NotFound || got.Msg != "contest not found" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
	if !Is(err, NotFound) {
		t.Fatal("Is")
	}
}

func TestAsUnknown(t *testing.T) {
	if _, ok := As(fmt.Errorf("disk full")); ok {
		t.Fatal("plain error is not an apperr")
	}
}
