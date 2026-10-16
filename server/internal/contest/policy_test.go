package contest

import (
	"testing"
	"time"

	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

func sample(start time.Time) Snapshot {
	return Snapshot{
		Rule: domain.RuleACM, EntryMode: domain.EntryIndividual,
		StartsAt: start, EndsAt: start.Add(2 * time.Hour), RegisterStartsAt: start.Add(-time.Hour),
		AllowExternal: true, MaxTeamSize: 3,
	}
}

func TestLateRegisterAndNoExtraTime(t *testing.T) {
	start := time.Unix(100, 0)
	s := sample(start)
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

func TestEmailUnverifiedCannotRegister(t *testing.T) {
	s := sample(time.Unix(100, 0))
	err := CanRegister(s, s.StartsAt, true, false)
	if err == nil || !apperr.Is(err, apperr.Unverified) {
		t.Fatalf("got %v", err)
	}
}

func TestRosterFreezeAfterStart(t *testing.T) {
	start := time.Unix(100, 0)
	accepted := start.Add(-time.Minute)
	if err := CanMutateRoster(&accepted, start, start.Add(time.Minute)); err == nil {
		t.Fatal("accepted roster must freeze after start")
	}
	if err := CanMutateRoster(&accepted, start, start.Add(-time.Second)); err != nil {
		t.Fatal("may change roster before start")
	}
	if err := CanMutateRoster(nil, start, start.Add(time.Minute)); err != nil {
		t.Fatal("unaccepted roster is not frozen")
	}
}

func TestCannotChangeEntryMode(t *testing.T) {
	if err := CanChangeEntryMode(); err == nil || !apperr.Is(err, apperr.Conflict) {
		t.Fatalf("got %v", err)
	}
}

func TestCannotExpandSourcePublicAfterLock(t *testing.T) {
	s := Snapshot{SourcePublicAfter: false, SourceVisibilityLocked: true}
	if err := CanExpandSourcePublic(s); err == nil || !apperr.Is(err, apperr.Conflict) {
		t.Fatalf("got %v", err)
	}
	s.SourcePublicAfter = true
	if err := CanExpandSourcePublic(s); err != nil {
		t.Fatal("already public is not an expansion")
	}
	unlocked := Snapshot{SourcePublicAfter: false, SourceVisibilityLocked: false}
	if err := CanExpandSourcePublic(unlocked); err != nil {
		t.Fatal(err)
	}
}

func TestSourceLockWhenRegistrationOpens(t *testing.T) {
	start := time.Unix(100, 0)
	s := Snapshot{RegisterStartsAt: start}
	if SourceVisibilityShouldLock(s, start.Add(-time.Second)) {
		t.Fatal("before window")
	}
	if !SourceVisibilityShouldLock(s, start) {
		t.Fatal("at window")
	}
}

func TestTeamSize(t *testing.T) {
	s := Snapshot{MaxTeamSize: 3}
	if err := TeamSizeOK(s, 0); err == nil {
		t.Fatal("empty")
	}
	if err := TeamSizeOK(s, 3); err != nil {
		t.Fatal(err)
	}
	if err := TeamSizeOK(s, 4); err == nil {
		t.Fatal("over")
	}
}

func TestExternalCannotSeeOtherOrgResources(t *testing.T) {
	outsider := Access{}
	if err := CanViewContest(false, true, outsider); err == nil {
		t.Fatal("closed contest")
	}
	if CanListOrgContestCatalog(outsider) {
		t.Fatal("catalog")
	}
	if err := CanViewProblemBank(outsider); err == nil {
		t.Fatal("bank")
	}
	if err := CanViewContestProblems(outsider); err == nil {
		t.Fatal("problems")
	}

	ext := Access{Registered: true, Accepted: true}
	if err := CanViewContest(false, true, ext); err != nil {
		t.Fatal(err)
	}
	if CanListOrgContestCatalog(ext) {
		t.Fatal("external must not list the org catalog")
	}
	if err := CanViewProblemBank(ext); err == nil {
		t.Fatal("external must not see the problem bank")
	}
	if err := CanViewContestProblems(ext); err != nil {
		t.Fatal(err)
	}
	if !ContestantLettersOnly(ext) {
		t.Fatal("letters only")
	}
}

func TestOrgMemberSeesBankNotLettersOnly(t *testing.T) {
	a := Access{OrgMember: true}
	if !CanListOrgContestCatalog(a) {
		t.Fatal("catalog")
	}
	if err := CanViewProblemBank(a); err != nil {
		t.Fatal(err)
	}
	if ContestantLettersOnly(a) {
		t.Fatal("members are not letter-only")
	}
}
