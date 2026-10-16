package contest

import (
	"testing"
	"time"

	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

func TestLateRegisterAndNoExtraTime(t *testing.T) {
	start := time.Unix(100, 0)
	s := Snapshot{
		Rule: domain.RuleACM, EntryMode: domain.EntryIndividual,
		StartsAt: start, EndsAt: start.Add(2 * time.Hour), RegisterStartsAt: start.Add(-time.Hour),
		AllowExternal: true,
	}
	now := start.Add(30 * time.Minute)
	if err := CanRegister(s, now, false, true); err != nil {
		t.Fatal(err)
	}
	if err := CanSubmit(s, now, true, true); err != nil {
		t.Fatal(err)
	}
	if err := CanChangeRule(s, now); err == nil {
		t.Fatal("rule frozen")
	}
}

func TestExternalDenied(t *testing.T) {
	start := time.Unix(100, 0)
	s := Snapshot{StartsAt: start, EndsAt: start.Add(time.Hour), RegisterStartsAt: start.Add(-time.Hour)}
	if err := CanRegister(s, start, false, true); err == nil {
		t.Fatal("external")
	}
}
