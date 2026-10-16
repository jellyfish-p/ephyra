package scoring

import (
	"testing"
	"time"

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
