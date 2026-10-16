package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/identity"
	"github.com/jellyfish-p/ephyra/server/internal/mailer"
	"github.com/jellyfish-p/ephyra/server/internal/objectstore"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/rate"
	"github.com/jellyfish-p/ephyra/server/internal/scoring"
	"github.com/jellyfish-p/ephyra/server/internal/submission"
	"github.com/jellyfish-p/ephyra/server/internal/testutil"
)

func TestMeAndSourceRequireAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.Database(t)
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	world := testutil.SeedRunnableContest(t, db, now)
	id := &identity.Module{DB: db, Clock: clock.Fixed{T: now}, Mail: mailer.Log{}, Limit: rate.New(), PublicURL: "http://localhost"}
	blobs := objectstore.NewMemory()
	sub := &submission.Module{DB: db, Clock: clock.Fixed{T: now}, Blobs: blobs}
	sc := &scoring.Module{DB: db, Clock: clock.Fixed{T: now}}
	api := &API{Identity: id, Submission: sub, Scoring: sc, Blobs: blobs}
	r := api.Router()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/me", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous me %d", w.Code)
	}

	got, err := sub.Submit(t.Context(), world.Principal, world.ContestID, world.Letter, "cpp23", []byte("int main(){}"))
	if err != nil {
		t.Fatal(err)
	}

	if err := id.Register(t.Context(), "other@example.com", "password1", "Other", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&persist.Account{}).Where("email_normalized = ?", "other@example.com").
		Update("email_verified_at", now).Error; err != nil {
		t.Fatal(err)
	}
	info, err := id.Login(t.Context(), "other@example.com", "password1", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/submissions/"+got.SubmissionID.String()+"/source", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: info.RawToken})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("other contestant source %d body %s", w.Code, w.Body.String())
	}
}

func TestLoginSessionCanReadOwnSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.Database(t)
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	id := &identity.Module{DB: db, Clock: clock.Fixed{T: now}, Mail: mailer.Log{}, Limit: rate.New(), PublicURL: "http://localhost"}
	if err := id.Register(t.Context(), "owner@example.com", "password1", "Owner", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&persist.Account{}).Where("email_normalized = ?", "owner@example.com").
		Update("email_verified_at", now).Error; err != nil {
		t.Fatal(err)
	}
	info, err := id.Login(t.Context(), "owner@example.com", "password1", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	api := &API{Identity: id}
	r := api.Router()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: info.RawToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("me %d %s", w.Code, w.Body.String())
	}
}
