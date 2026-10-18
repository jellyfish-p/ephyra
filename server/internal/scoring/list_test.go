package scoring

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

func TestListSubmissionsRequiresPrincipal(t *testing.T) {
	_, err := (&Module{}).ListSubmissions(t.Context(), access.Principal{}, uuid.New(), SubmissionFilter{})
	if e, ok := apperr.As(err); !ok || e.Code != apperr.Unauthorized {
		t.Fatalf("anonymous list: %v", err)
	}
}

func TestNonParticipantFeedbackRightsAreNotBoardVisibility(t *testing.T) {
	now := time.Unix(1000, 0)
	for _, rule := range []domain.Rule{domain.RuleOI, domain.RuleIOI, domain.RuleACM} {
		t.Run(string(rule), func(t *testing.T) {
			view := Visible(VisibilityInput{
				Rule: rule, Phase: domain.PhaseRunning, Audience: access.AudienceLoggedIn,
				Now: now, EndsAt: now.Add(time.Hour), IOIPublicBoard: true,
			})
			// Even where ShowOnBoard is true, existing policy does not grant
			// this audience submission feedback. Lists must use ShouldEmit.
			if ShouldEmit(view) {
				t.Fatalf("non-participant must not receive hidden submission rows: %+v", view)
			}
		})
	}
	published := Visible(VisibilityInput{Rule: domain.RuleOI, Phase: domain.PhasePublished, Audience: access.AudienceLoggedIn})
	if !ShouldEmit(published) || published.Feedback != FeedbackScore {
		t.Fatal("published OI results should retain existing score-only feedback")
	}
}
