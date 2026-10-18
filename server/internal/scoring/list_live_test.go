package scoring

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/objectstore"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/submission"
	"github.com/jellyfish-p/ephyra/server/internal/testutil"
	"gorm.io/gorm"
)

func listFixture(t *testing.T) (*gorm.DB, testutil.ContestWorld, *Module, persist.Submission, uuid.UUID) {
	t.Helper()
	db := testutil.Database(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	world := testutil.SeedRunnableContest(t, db, now)
	mod := &Module{DB: db, Clock: clock.Fixed{T: now}}
	submit := &submission.Module{DB: db, Clock: mod.Clock, Blobs: objectstore.NewMemory()}
	got, err := submit.Submit(t.Context(), world.Principal, world.ContestID, "A", "cpp23", []byte("int main(){}"))
	mustListDB(t, err)
	var sub persist.Submission
	mustListDB(t, db.First(&sub, "id = ?", got.SubmissionID).Error)
	return db, world, mod, sub, got.TaskID
}

func mustListDB(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func listAccount(t *testing.T, db *gorm.DB, name string) access.Principal {
	t.Helper()
	id := uuid.New()
	mail := id.String() + "@example.com"
	mustListDB(t, db.Create(&persist.Account{ID: id, Email: mail, EmailNormalized: mail, PasswordHash: "x", DisplayName: name}).Error)
	return access.Principal{AccountID: id, DisplayName: name, EmailVerified: true}
}

func TestListSubmissionsSharesSingleReadProjection(t *testing.T) {
	db, world, mod, sub, taskID := listFixture(t)
	// Prefer the target revision result just as ViewSubmission does, marking it
	// not effective while retaining the scoring revision's visibility policy.
	rev := persist.JudgeDataRevision{ID: uuid.New(), ProblemID: world.ProblemID, RevisionN: 2, CreatedBy: world.AccountID, NormalizedConfig: json.RawMessage(testutil.MinimalConfigJSON)}
	mustListDB(t, db.Create(&rev).Error)
	mustListDB(t, db.Model(&persist.ContestProblem{}).Where("id = ?", sub.ContestProblemID).Update("target_revision_id", rev.ID).Error)
	attempt := persist.JudgingAttempt{
		ID: uuid.New(), TaskID: taskID, Generation: 1, NodeID: world.NodeID, PoolID: world.PoolID,
		JudgeDataRevisionID: rev.ID, ExecutionProfileID: world.Profile, State: "finished",
		Result: json.RawMessage(`{"Cases":[{"CaseID":"1","Verdict":"accepted","ExpectedOutput":"SECRET_OUTPUT","CheckerLog":"SECRET_CHECKER"}]}`),
	}
	mustListDB(t, db.Create(&attempt).Error)
	mustListDB(t, db.Create(&persist.SubmissionResult{
		SubmissionID: sub.ID, RevisionID: rev.ID, AttemptID: attempt.ID,
		Verdict: string(domain.VerdictAccepted), ScoreHundredths: 5000,
		SubtaskScores: json.RawMessage(`[{"ID":"s1","Score":"50.00","Verdict":"accepted"}]`),
	}).Error)
	mustListDB(t, db.Model(&persist.Contest{}).Where("id = ?", world.ContestID).Update("case_feedback", string(domain.FeedbackCases)).Error)
	teammate := listAccount(t, db, "Teammate")
	mustListDB(t, db.Create(&persist.EntrantMember{ID: uuid.New(), ContestID: world.ContestID, EntrantID: sub.EntrantID, AccountID: teammate.AccountID, Role: "member", ConfirmedAt: &world.Now}).Error)
	other := listAccount(t, db, "Other entrant")
	otherEntrant := persist.Entrant{ID: uuid.New(), ContestID: world.ContestID, Kind: "individual", DisplayName: "Other", Status: "accepted", RegisteredAt: world.Now}
	mustListDB(t, db.Create(&otherEntrant).Error)
	mustListDB(t, db.Create(&persist.EntrantMember{ID: uuid.New(), ContestID: world.ContestID, EntrantID: otherEntrant.ID, AccountID: other.AccountID, Role: "captain", ConfirmedAt: &world.Now}).Error)
	outsider := listAccount(t, db, "Non-participant")
	orgAdmin := listAccount(t, db, "Org admin")
	mustListDB(t, db.Create(&persist.OrganizationMembership{ID: uuid.New(), OrganizationID: world.OrgID, AccountID: orgAdmin.AccountID, Role: "admin"}).Error)
	siteAdmin := access.Principal{AccountID: uuid.New(), SiteAdmin: true}

	for _, phase := range []struct {
		name      string
		rule      domain.Rule
		published bool
		public    bool
	}{
		{"running OI", domain.RuleOI, false, false},
		{"running IOI public board", domain.RuleIOI, false, true},
		{"running ACM", domain.RuleACM, false, false},
		{"published OI", domain.RuleOI, true, false},
	} {
		t.Run(phase.name, func(t *testing.T) {
			var published *time.Time
			if phase.published {
				published = &world.Now
			}
			mustListDB(t, db.Model(&persist.Contest{}).Where("id = ?", world.ContestID).Updates(map[string]any{
				"rule": string(phase.rule), "published_at": published, "ioi_public_scoreboard": phase.public,
			}).Error)
			for _, viewer := range []struct {
				name  string
				p     access.Principal
				own   bool
				admin bool
			}{
				{"owner", world.Principal, true, false}, {"teammate", teammate, true, false},
				{"other entrant", other, false, false}, {"non-participant", outsider, false, false},
				{"org admin", orgAdmin, false, true}, {"site admin", siteAdmin, false, true},
			} {
				t.Run(viewer.name, func(t *testing.T) {
					got, err := mod.ListSubmissions(t.Context(), viewer.p, world.ContestID, SubmissionFilter{})
					mustListDB(t, err)
					want, view, err := mod.ViewSubmission(t.Context(), viewer.p, sub.ID)
					mustListDB(t, err)
					visible := viewer.own || viewer.admin || phase.published
					if visible != ShouldEmit(view) {
						t.Fatalf("unexpected feedback policy: %+v", view)
					}
					if !visible {
						if got == nil || len(got) != 0 {
							t.Fatalf("hidden rows or null array: %+v", got)
						}
						return
					}
					if len(got) != 1 {
						t.Fatalf("list projection %+v", got)
					}
					// The list carries entrant_display_name so a board need not resolve
					// one name per row; the single read does not. Everything
					// visibility-controlled still has to match.
					if got[0].EntrantDisplayName == "" {
						t.Fatal("list row must carry the entrant display name")
					}
					got[0].EntrantDisplayName = ""
					if !reflect.DeepEqual(got[0], want) {
						t.Fatalf("list projection %+v differs from individual %+v", got, want)
					}
					if !got[0].NotEffective || got[0].RevisionID != rev.ID {
						t.Fatal("must use existing rejudge result selection")
					}
					if viewer.own && phase.rule == domain.RuleOI && !phase.published {
						if got[0].CompileFailed == nil || got[0].Score != nil || got[0].Verdict != "" || len(got[0].Cases) != 0 {
							t.Fatalf("compile-only result leak: %+v", got[0])
						}
					} else if got[0].Score == nil || got[0].Score.String() != "50.00" {
						t.Fatalf("visible score: %+v", got[0])
					}
				})
			}
		})
	}
}

func TestListSubmissionsPaginatesVisibleRowsAndScopesFilters(t *testing.T) {
	db, world, mod, first, _ := listFixture(t)
	second := first
	second.ID, second.ReceiveSeq = uuid.New(), 0
	mustListDB(t, db.Create(&second).Error)
	other := listAccount(t, db, "Other")
	entrant := persist.Entrant{ID: uuid.New(), ContestID: world.ContestID, Kind: "individual", DisplayName: "Other", Status: "accepted", RegisteredAt: world.Now}
	mustListDB(t, db.Create(&entrant).Error)
	mustListDB(t, db.Create(&persist.EntrantMember{ID: uuid.New(), ContestID: world.ContestID, EntrantID: entrant.ID, AccountID: other.AccountID, Role: "captain"}).Error)
	hidden := make([]persist.Submission, 205)
	for i := range hidden {
		hidden[i] = first
		hidden[i].ID, hidden[i].ReceiveSeq = uuid.New(), 0
		hidden[i].EntrantID, hidden[i].SubmitterAccountID = entrant.ID, other.AccountID
	}
	mustListDB(t, db.Create(&hidden).Error)
	got, err := mod.ListSubmissions(t.Context(), world.Principal, world.ContestID, SubmissionFilter{Limit: 1})
	mustListDB(t, err)
	if len(got) != 1 || got[0].ID != second.ID {
		t.Fatalf("hidden newer batches must not consume visible page limit: %+v", got)
	}
	got, err = mod.ListSubmissions(t.Context(), world.Principal, world.ContestID, SubmissionFilter{Limit: 1, BeforeSeq: &second.ReceiveSeq})
	mustListDB(t, err)
	if len(got) != 1 || got[0].ID != first.ID {
		t.Fatalf("cursor must be strict: %+v", got)
	}
	zero := int64(0)
	got, err = mod.ListSubmissions(t.Context(), world.Principal, world.ContestID, SubmissionFilter{BeforeSeq: &zero})
	mustListDB(t, err)
	if got == nil || len(got) != 0 {
		t.Fatalf("before zero: %+v", got)
	}
	admin := access.Principal{AccountID: uuid.New(), SiteAdmin: true}
	for _, tc := range []struct{ limit, want int }{{0, 50}, {-1, 1}, {500, 200}} {
		got, err = mod.ListSubmissions(t.Context(), admin, world.ContestID, SubmissionFilter{Limit: tc.limit})
		mustListDB(t, err)
		if len(got) != tc.want {
			t.Fatalf("limit %d returned %d, want %d", tc.limit, len(got), tc.want)
		}
		for i := 1; i < len(got); i++ {
			if got[i-1].ReceiveSeq <= got[i].ReceiveSeq {
				t.Fatal("must order receive_seq descending")
			}
		}
	}
	got, err = mod.ListSubmissions(t.Context(), admin, world.ContestID, SubmissionFilter{EntrantID: &first.EntrantID, ContestProblemID: &first.ContestProblemID})
	mustListDB(t, err)
	if len(got) != 2 || got[0].ID != second.ID || got[1].ID != first.ID {
		t.Fatalf("combined filters: %+v", got)
	}
	missing := uuid.New()
	got, err = mod.ListSubmissions(t.Context(), admin, world.ContestID, SubmissionFilter{ContestProblemID: &missing})
	mustListDB(t, err)
	if len(got) != 0 {
		t.Fatalf("problem filter ignored: %+v", got)
	}
	// Another contest in the same organization must never enter the list.
	var otherContest persist.Contest
	mustListDB(t, db.First(&otherContest, "id = ?", world.ContestID).Error)
	otherContest.ID, otherContest.Slug = uuid.New(), "other-contest"
	mustListDB(t, db.Create(&otherContest).Error)
	otherProblem := persist.ContestProblem{ID: uuid.New(), ContestID: otherContest.ID, ProblemID: first.ProblemID, Letter: "A", ScoringRevisionID: first.JudgeDataRevisionID, TargetRevisionID: first.JudgeDataRevisionID}
	mustListDB(t, db.Create(&otherProblem).Error)
	entrant.ID, entrant.ContestID = uuid.New(), otherContest.ID
	mustListDB(t, db.Create(&entrant).Error)
	foreign := first
	foreign.ID, foreign.ReceiveSeq = uuid.New(), 0
	foreign.ContestID, foreign.ContestProblemID, foreign.EntrantID = otherContest.ID, otherProblem.ID, entrant.ID
	mustListDB(t, db.Create(&foreign).Error)
	got, err = mod.ListSubmissions(t.Context(), admin, world.ContestID, SubmissionFilter{Limit: 1})
	mustListDB(t, err)
	if len(got) != 1 || got[0].ContestID != world.ContestID || got[0].ID == foreign.ID {
		t.Fatalf("cross-contest leak: %+v", got)
	}
}
