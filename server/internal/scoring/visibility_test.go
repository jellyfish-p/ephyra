package scoring

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

func TestOIHidesTestsDuringContest(t *testing.T) {
	now := time.Unix(100, 0)
	in := VisibilityInput{
		Rule: domain.RuleOI, Phase: domain.PhaseRunning, Now: now,
		EndsAt: now.Add(time.Hour), Granularity: domain.FeedbackCases,
		Audience: access.AudienceSelf, OwnSubmission: true,
	}
	v := Visible(in)
	if v.Feedback != FeedbackCompileOnly {
		t.Fatalf("OI contestant must only see compile status, got %v", v.Feedback)
	}
}

func TestAnonymousNeverSeesSource(t *testing.T) {
	in := VisibilityInput{
		Rule: domain.RuleACM, Phase: domain.PhasePublished, Published: true,
		SourcePublic: true, Audience: access.AudienceAnonymous,
	}
	if Visible(in).ShowSource {
		t.Fatal("anonymous must not read public source")
	}
}

func TestLoggedInSeesPublicSourceAfterPublish(t *testing.T) {
	in := VisibilityInput{
		Rule: domain.RuleACM, Phase: domain.PhasePublished, Published: true,
		SourcePublic: true, Audience: access.AudienceLoggedIn,
	}
	if !Visible(in).ShowSource {
		t.Fatal("logged-in should see opted-in public source")
	}
}

func TestSourceNotExpandedWhenNotOptedIn(t *testing.T) {
	in := VisibilityInput{
		Rule: domain.RuleACM, Phase: domain.PhasePublished, Published: true,
		SourcePublic: false, Audience: access.AudienceLoggedIn,
	}
	if Visible(in).ShowSource {
		t.Fatal("must not auto-publish source")
	}
}

func TestACMFreezeHidesOthers(t *testing.T) {
	now := time.Unix(1000, 0)
	in := VisibilityInput{
		Rule: domain.RuleACM, Phase: domain.PhaseRunning, Now: now,
		EndsAt: now.Add(10 * time.Minute), FreezeFor: 30 * time.Minute,
		Audience: access.AudienceOtherContestant,
	}
	v := Visible(in)
	if v.Feedback != FeedbackNone {
		t.Fatalf("others must not see results during ACM freeze, got %v", v.Feedback)
	}
	if FilterBoard(in, true, access.AudienceOtherContestant) {
		t.Fatal("post-freeze solve must be hidden from others")
	}
	if !FilterBoard(in, true, access.AudienceSelf) {
		t.Fatal("self still sees own post-freeze solve")
	}
	if EmitFields(v).Score || EmitFields(v).Verdict {
		t.Fatal("freeze payload must omit others' score fields")
	}
}

func TestIOISelfSeesScore(t *testing.T) {
	now := time.Unix(100, 0)
	in := VisibilityInput{
		Rule: domain.RuleIOI, Phase: domain.PhaseRunning, Now: now,
		EndsAt: now.Add(time.Hour), Audience: access.AudienceSelf, OwnSubmission: true,
	}
	v := Visible(in)
	if v.Feedback != FeedbackFull {
		t.Fatalf("IOI self must see score, got %v", v.Feedback)
	}
	if !EmitFields(v).Score {
		t.Fatal("IOI self emit mask must include score")
	}
}

func TestOIAdminSeesFull(t *testing.T) {
	now := time.Unix(100, 0)
	in := VisibilityInput{
		Rule: domain.RuleOI, Phase: domain.PhaseRunning, Now: now,
		EndsAt: now.Add(time.Hour), Granularity: domain.FeedbackCases,
		Audience: access.AudienceContestAdmin,
	}
	v := Visible(in)
	if v.Feedback != FeedbackFull {
		t.Fatalf("OI admin must see full, got %v", v.Feedback)
	}
	mask := EmitFields(v)
	if !mask.Score || !mask.Cases || !mask.Subtasks {
		t.Fatalf("OI admin emit mask must be full, got %+v", mask)
	}
}

func TestPublishedOIContestantSeesDetails(t *testing.T) {
	now := time.Unix(100, 0)
	in := VisibilityInput{
		Rule: domain.RuleOI, Phase: domain.PhasePublished, Published: true,
		Now: now, EndsAt: now.Add(-time.Hour), Granularity: domain.FeedbackCases,
		Audience: access.AudienceSelf, OwnSubmission: true,
	}
	v := Visible(in)
	if v.Feedback != FeedbackFull {
		t.Fatalf("published OI contestant must see details, got %v", v.Feedback)
	}
	mask := EmitFields(v)
	if !mask.Score || !mask.Cases || !mask.Subtasks {
		t.Fatalf("published OI details mask %+v", mask)
	}
}

func TestNotEffectiveWhenRevisionDiffers(t *testing.T) {
	in := VisibilityInput{
		Rule: domain.RuleIOI, Phase: domain.PhaseRunning,
		Audience: access.AudienceSelf, OwnSubmission: true,
		SubmissionRevision: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ScoringRevision:    uuid.MustParse("22222222-2222-2222-2222-222222222222"),
	}
	if !Visible(in).NotEffective {
		t.Fatal("personal rejudge must be marked not_effective")
	}
}
