package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/contest"
	"github.com/jellyfish-p/ephyra/server/internal/objectstore"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/scoring"
	"github.com/jellyfish-p/ephyra/server/internal/submission"
	"github.com/jellyfish-p/ephyra/server/internal/testutil"
	"gorm.io/gorm"
)

func contestReadFixture(t *testing.T) (*gorm.DB, testutil.ContestWorld, *API) {
	t.Helper()
	db := testutil.Database(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	world := testutil.SeedRunnableContest(t, db, now)
	fixed := clock.Fixed{T: now}
	blobs := objectstore.NewMemory()
	return db, world, &API{
		Contest:    &contest.Module{DB: db, Clock: fixed},
		Scoring:    &scoring.Module{DB: db, Clock: fixed},
		Submission: &submission.Module{DB: db, Clock: fixed, Blobs: blobs}, Blobs: blobs,
	}
}

func mustReadDB(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func getContestRead(t *testing.T, r *gin.Engine, path string, status int) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Code != status {
		t.Fatalf("GET %s: status %d, want %d: %s", path, w.Code, status, w.Body.String())
	}
	return w
}

type recordingStatementStore struct {
	objectstore.Store
	keys []string
}

func (s *recordingStatementStore) Get(ctx context.Context, key string) (io.ReadCloser, int64, string, error) {
	s.keys = append(s.keys, key)
	return s.Store.Get(ctx, key)
}

func addStatementFile(t *testing.T, db *gorm.DB, blobs objectstore.Store, revisionID uuid.UUID, role, name, body string, readable bool) persist.RevisionFile {
	t.Helper()
	obj := persist.Object{ID: uuid.New(), SHA256: uuid.NewString(), StorageKey: uuid.NewString(), SizeBytes: int64(len(body)), ContentType: "text/plain"}
	mustReadDB(t, db.Create(&obj).Error)
	if readable {
		mustReadDB(t, blobs.Put(t.Context(), obj.StorageKey, strings.NewReader(body), int64(len(body)), obj.ContentType))
	}
	file := persist.RevisionFile{ID: uuid.New(), RevisionID: revisionID, Role: role, Name: name, ObjectID: obj.ID}
	mustReadDB(t, db.Create(&file).Error)
	return file
}

func TestContestProblemDetailUsesScoringRevisionAndNeverExposesJudgeFiles(t *testing.T) {
	db, world, api := contestReadFixture(t)
	var cp persist.ContestProblem
	mustReadDB(t, db.First(&cp, "contest_id = ?", world.ContestID).Error)
	mustReadDB(t, db.Model(&persist.Problem{}).Where("id = ?", cp.ProblemID).Update("title", "Two Sum").Error)
	target := persist.JudgeDataRevision{
		ID: uuid.New(), ProblemID: cp.ProblemID, RevisionN: 2, CreatedBy: world.AccountID,
		NormalizedConfig: json.RawMessage(strings.Replace(testutil.MinimalConfigJSON, "1000", "9999", 1)),
	}
	mustReadDB(t, db.Create(&target).Error)
	mustReadDB(t, db.Model(&cp).Update("target_revision_id", target.ID).Error)
	store := &recordingStatementStore{Store: api.Blobs}
	api.Blobs = store
	addStatementFile(t, db, store, target.ID, "data", "statement.md", "WRONG_TARGET_STATEMENT", true)
	for _, file := range []struct{ role, name string }{
		{"checker", "00-checker.md"}, {"interactor", "01-interactor.MD"},
		{"data", "1.in"}, {"data", "1.out"}, {"data", "checker.cpp"},
		{"data", "statement.md.bak"}, {"data", "config.yaml"},
	} {
		addStatementFile(t, db, store, cp.ScoringRevisionID, file.role, file.name, "SECRET_"+file.name, true)
	}
	fallback := addStatementFile(t, db, store, cp.ScoringRevisionID, "data", "A.MD", "# Fallback", true)
	addStatementFile(t, db, store, cp.ScoringRevisionID, "data", "z.md", "# Last", true)
	preferred := addStatementFile(t, db, store, cp.ScoringRevisionID, "data", "statement.md", "# Two Sum\n", true)
	r := contestReadRouter(api, world.Principal)
	path := "/api/contests/" + world.ContestID.String() + "/problems/%20a%20"
	w := getContestRead(t, r, path, http.StatusOK)
	var got contest.ProblemDetail
	mustReadDB(t, json.Unmarshal(w.Body.Bytes(), &got))
	if got.StatementMarkdown == nil || *got.StatementMarkdown != "# Two Sum\n" || got.TimeLimitMS != 1000 || got.ScoringRevisionID != cp.ScoringRevisionID || got.TargetRevisionID != target.ID || got.Title != "Two Sum" || got.Letter != "A" {
		t.Fatalf("detail: %+v", got)
	}
	var shape map[string]any
	mustReadDB(t, json.Unmarshal(w.Body.Bytes(), &shape))
	if len(shape) != 11 || shape["samples"] != nil || strings.Contains(w.Body.String(), "SECRET_") || strings.Contains(w.Body.String(), "WRONG_TARGET") {
		t.Fatalf("unsafe detail: %s", w.Body.String())
	}
	var obj persist.Object
	mustReadDB(t, db.First(&obj, "id = ?", preferred.ObjectID).Error)
	if !reflect.DeepEqual(store.keys, []string{obj.StorageKey}) {
		t.Fatalf("must fetch only selected data markdown, fetched %v", store.keys)
	}

	// Without the exact preferred name, select lexicographically first .md,
	// including uppercase extensions, while ignoring checker/interactor .md.
	mustReadDB(t, db.Delete(&preferred).Error)
	store.keys = nil
	w = getContestRead(t, r, path, http.StatusOK)
	mustReadDB(t, json.Unmarshal(w.Body.Bytes(), &got))
	if got.StatementMarkdown == nil || *got.StatementMarkdown != "# Fallback" {
		t.Fatalf("fallback: %s", w.Body.String())
	}
	obj = persist.Object{}
	mustReadDB(t, db.First(&obj, "id = ?", fallback.ObjectID).Error)
	if !reflect.DeepEqual(store.keys, []string{obj.StorageKey}) {
		t.Fatalf("unsafe fallback fetch: %v", store.keys)
	}

	// A missing preferred object is optional, not a request failure.
	addStatementFile(t, db, store, cp.ScoringRevisionID, "data", "statement.md", "missing", false)
	w = getContestRead(t, r, path, http.StatusOK)
	mustReadDB(t, json.Unmarshal(w.Body.Bytes(), &got))
	if got.StatementMarkdown != nil {
		t.Fatal("unreadable preferred statement must be null")
	}
	mustReadDB(t, db.Where("revision_id = ? AND role = ? AND LOWER(name) LIKE ?", cp.ScoringRevisionID, "data", "%.md").Delete(&persist.RevisionFile{}).Error)
	store.keys = nil
	w = getContestRead(t, r, path, http.StatusOK)
	mustReadDB(t, json.Unmarshal(w.Body.Bytes(), &got))
	if got.StatementMarkdown != nil || len(store.keys) != 0 {
		t.Fatal("non-markdown and checker/interactor files must not become statements")
	}
	w = getContestRead(t, r, "/api/contests/"+world.ContestID.String()+"/problems/Z", http.StatusNotFound)
	if !strings.Contains(w.Body.String(), "contest problem not found") {
		t.Fatalf("error: %s", w.Body.String())
	}
}

func TestContestReadsUseGetVisibility(t *testing.T) {
	db, world, api := contestReadFixture(t)
	outsider := access.Principal{AccountID: uuid.New()}
	r := contestReadRouter(api, outsider)
	for _, suffix := range []string{"/problems/A", "/my-entrant"} {
		getContestRead(t, r, "/api/contests/"+world.ContestID.String()+suffix, http.StatusForbidden)
		getContestRead(t, r, "/api/contests/"+uuid.NewString()+suffix, http.StatusNotFound)
	}
	mustReadDB(t, db.Model(&persist.Contest{}).Where("id = ?", world.ContestID).Update("allow_external", true).Error)
	getContestRead(t, r, "/api/contests/"+world.ContestID.String()+"/problems/A", http.StatusOK)
	w := getContestRead(t, r, "/api/contests/"+world.ContestID.String()+"/my-entrant", http.StatusOK)
	if w.Body.String() != `{"entrant":null}` {
		t.Fatalf("not registered: %s", w.Body.String())
	}
}

func TestMyEntrantIncludesIndividualAndTeamRoster(t *testing.T) {
	db, world, api := contestReadFixture(t)
	var member persist.EntrantMember
	mustReadDB(t, db.First(&member, "contest_id = ? AND account_id = ?", world.ContestID, world.AccountID).Error)
	mustReadDB(t, db.Model(&member).Update("role", "captain").Error)
	r := contestReadRouter(api, world.Principal)
	path := "/api/contests/" + world.ContestID.String() + "/my-entrant"
	w := getContestRead(t, r, path, http.StatusOK)
	var response struct {
		Entrant *contest.MyEntrantView `json:"entrant"`
	}
	mustReadDB(t, json.Unmarshal(w.Body.Bytes(), &response))
	en := response.Entrant
	if en == nil || en.ID != member.EntrantID || en.ContestID != world.ContestID || en.Kind != "individual" || en.Status != "accepted" || en.AcceptedAt == nil || en.RosterFrozenAt != nil || len(en.Members) != 1 || en.Members[0].AccountID != world.AccountID || en.Members[0].Role != "captain" || !en.Members[0].Confirmed {
		t.Fatalf("individual: %s", w.Body.String())
	}
	if en.RegisteredAt != world.Now.Add(-time.Hour) || *en.AcceptedAt != world.Now.Add(-time.Hour) {
		t.Fatalf("entrant timestamps: %+v", en)
	}
	// The roster name is joined from accounts, not copied from the entrant.
	mustReadDB(t, db.Model(&persist.Account{}).Where("id = ?", world.AccountID).Update("display_name", "Alice").Error)
	teammate := persist.Account{ID: uuid.New(), Email: "team@example.com", EmailNormalized: "team@example.com", PasswordHash: "x", DisplayName: "Bob"}
	mustReadDB(t, db.Create(&teammate).Error)
	mustReadDB(t, db.Create(&persist.EntrantMember{ID: uuid.New(), ContestID: world.ContestID, EntrantID: member.EntrantID, AccountID: teammate.ID, Role: "member"}).Error)
	mustReadDB(t, db.Model(&persist.Entrant{}).Where("id = ?", member.EntrantID).Updates(map[string]any{
		"kind": "team", "display_name": "Team", "status": "pending", "accepted_at": nil,
	}).Error)
	w = getContestRead(t, r, path, http.StatusOK)
	mustReadDB(t, json.Unmarshal(w.Body.Bytes(), &response))
	en = response.Entrant
	if en.Kind != "team" || en.DisplayName != "Team" || en.Status != "pending" || en.AcceptedAt != nil || en.RosterFrozenAt != nil || len(en.Members) != 2 {
		t.Fatalf("team: %s", w.Body.String())
	}
	for _, mem := range en.Members {
		if mem.AccountID == world.AccountID && (mem.DisplayName != "Alice" || !mem.Confirmed || mem.Role != "captain") {
			t.Fatalf("captain: %+v", mem)
		}
		if mem.AccountID == teammate.ID && (mem.DisplayName != "Bob" || mem.Confirmed || mem.Role != "member") {
			t.Fatalf("invitee: %+v", mem)
		}
	}
	// An unconfirmed member still sees their own pending entrant, like Get.
	invitee := contestReadRouter(api, access.Principal{AccountID: teammate.ID})
	getContestRead(t, invitee, path, http.StatusOK)
}

func TestContestSubmissionListFiltersAndHidesNonParticipantRows(t *testing.T) {
	_, world, api := contestReadFixture(t)
	first, err := api.Submission.Submit(t.Context(), world.Principal, world.ContestID, "A", "cpp23", []byte("int main(){}"))
	mustReadDB(t, err)
	second, err := api.Submission.Submit(t.Context(), world.Principal, world.ContestID, "A", "cpp23", []byte("int main(){return 0;}"))
	mustReadDB(t, err)
	r := contestReadRouter(api, world.Principal)
	path := "/api/contests/" + world.ContestID.String() + "/submissions"
	w := getContestRead(t, r, path+"?letter=%20a%20&limit=1", http.StatusOK)
	var response struct {
		Submissions []scoring.SubmissionPublic `json:"submissions"`
	}
	mustReadDB(t, json.Unmarshal(w.Body.Bytes(), &response))
	if len(response.Submissions) != 1 || response.Submissions[0].ID != second.SubmissionID {
		t.Fatalf("latest row: %s", w.Body.String())
	}
	view, _, err := api.Scoring.ViewSubmission(t.Context(), world.Principal, second.SubmissionID)
	mustReadDB(t, err)
	wantJSON, err := json.Marshal(view)
	mustReadDB(t, err)
	gotJSON, err := json.Marshal(response.Submissions[0])
	mustReadDB(t, err)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("list projection differs from single read: %s vs %s", gotJSON, wantJSON)
	}
	w = getContestRead(t, r, path+"?entrant_id="+view.EntrantID.String()+"&before_seq="+strconv.FormatInt(second.ReceiveSeq, 10), http.StatusOK)
	mustReadDB(t, json.Unmarshal(w.Body.Bytes(), &response))
	if len(response.Submissions) != 1 || response.Submissions[0].ID != first.SubmissionID {
		t.Fatalf("cursor filter: %s", w.Body.String())
	}
	w = getContestRead(t, r, path+"?entrant_id="+uuid.NewString(), http.StatusOK)
	if w.Body.String() != `{"submissions":[]}` {
		t.Fatalf("empty array: %s", w.Body.String())
	}
	getContestRead(t, r, path+"?letter=Z", http.StatusNotFound)
	outsider := contestReadRouter(api, access.Principal{AccountID: uuid.New()})
	w = getContestRead(t, outsider, path, http.StatusOK)
	if w.Body.String() != `{"submissions":[]}` {
		t.Fatalf("non-participant must not receive even hidden-row metadata: %s", w.Body.String())
	}
	getContestRead(t, r, "/api/contests/"+uuid.NewString()+"/submissions", http.StatusNotFound)
}
