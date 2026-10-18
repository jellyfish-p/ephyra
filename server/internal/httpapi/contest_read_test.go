package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
)

func contestReadRouter(api *API, p access.Principal) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if !p.Zero() {
			c.Set("principal", p)
		}
		c.Next()
	})
	api.contestRoutes(r)
	api.submitRoutes(r)
	return r
}

func TestContestReadRoutesRequireSession(t *testing.T) {
	r := (&API{}).Router()
	for _, suffix := range []string{"/problems/A", "/my-entrant", "/submissions"} {
		t.Run(suffix, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/contests/"+uuid.NewString()+suffix, nil))
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestContestReadRoutesRejectInvalidContestID(t *testing.T) {
	r := contestReadRouter(&API{}, access.Principal{AccountID: uuid.New()})
	for _, suffix := range []string{"/problems/A", "/my-entrant", "/submissions"} {
		t.Run(suffix, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/contests/not-a-uuid"+suffix, nil))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestSubmissionFilterParsing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	entrant := uuid.New()
	for _, tc := range []struct {
		query string
		limit int
	}{
		{"", 50}, {"?limit=0", 1}, {"?limit=-10", 1}, {"?limit=201", 200},
		{"?limit=12&entrant_id=" + entrant.String() + "&before_seq=9223372036854775807", 12},
		{"?before_seq=0", 50},
	} {
		t.Run(tc.query, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/"+tc.query, nil)
			f, err := submissionFilter(c)
			if err != nil || f.Limit != tc.limit {
				t.Fatalf("filter %+v error %v", f, err)
			}
			if tc.limit == 12 && (f.EntrantID == nil || *f.EntrantID != entrant || f.BeforeSeq == nil || *f.BeforeSeq != int64(9223372036854775807)) {
				t.Fatalf("lost optional filters: %+v", f)
			}
			if tc.query == "?before_seq=0" && (f.BeforeSeq == nil || *f.BeforeSeq != 0) {
				t.Fatal("zero cursor must not be treated as absent")
			}
			if tc.query == "" && (f.EntrantID != nil || f.BeforeSeq != nil || f.ContestProblemID != nil) {
				t.Fatal("default filters must be absent")
			}
		})
	}
}

func TestListSubmissionsRejectsInvalidFilters(t *testing.T) {
	r := contestReadRouter(&API{}, access.Principal{AccountID: uuid.New()})
	for _, query := range []string{"entrant_id=bad", "limit=bad", "before_seq=bad", "before_seq=9223372036854775808"} {
		t.Run(query, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/contests/"+uuid.NewString()+"/submissions?"+query, nil))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
			_, err := submissionFilter(c)
			if e, ok := apperr.As(err); !ok || e.Code != apperr.Invalid {
				t.Fatalf("expected invalid application error: %v", err)
			}
		})
	}
}
