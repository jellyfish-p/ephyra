package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/scoring"
)

func testOwnedRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	api := &API{}
	r := gin.New()
	r.Use(api.session)
	api.submitRoutes(r)
	api.eventRoutes(r)
	return r
}

func TestContestEventsRequiresSession(t *testing.T) {
	r := testOwnedRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/contests/"+uuid.NewString()+"/events", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
	}
}

func TestGetSubmissionRequiresSession(t *testing.T) {
	r := testOwnedRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/submissions/"+uuid.NewString(), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
	}
}

func TestWriteSSECommentIsPing(t *testing.T) {
	w := httptest.NewRecorder()
	writeSSEComment(w, "ping")
	if w.Body.String() != ": ping\n\n" {
		t.Fatalf("%q", w.Body.String())
	}
}

func TestWriteSSEDataUsesProjectedDTO(t *testing.T) {
	w := httptest.NewRecorder()
	dto := scoring.ProjectSubmission(scoring.SubmissionMeta{ID: uuid.New()}, scoring.View{Feedback: scoring.FeedbackCompileOnly}, &scoring.ResultDetail{
		HasResult: true, CompileFailed: true,
	})
	if err := writeSSEData(w, "submission", dto); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.HasPrefix(body, "event: submission\ndata: ") {
		t.Fatalf("framing %q", body)
	}
	raw := strings.TrimPrefix(body, "event: submission\ndata: ")
	raw = strings.TrimSuffix(raw, "\n\n")
	if bytes.Contains([]byte(raw), []byte("checker")) || bytes.Contains([]byte(raw), []byte("expected")) {
		t.Fatal("SSE must not carry checker or expected output")
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got["feedback"] != "compile" {
		t.Fatalf("feedback %v", got["feedback"])
	}
	if _, ok := got["score"]; ok {
		t.Fatal("compile-only SSE must omit score")
	}
}
