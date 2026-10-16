package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
)

func TestWriteErrHidesUnknownMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/boom", func(c *gin.Context) {
		writeErr(c, fmt.Errorf("pq: password authentication failed"))
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", w.Code)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != string(apperr.Internal) || body.Error.Message != "internal error" {
		t.Fatalf("leaked %+v body %s", body, w.Body.String())
	}
}

func TestWriteErrKeepsAppMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/gone", func(c *gin.Context) {
		writeErr(c, apperr.New(apperr.NotFound, "contest not found"))
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/gone", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d", w.Code)
	}
}

func TestReadyReportsFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	api := &API{Ready: func(context.Context) error { return fmt.Errorf("db down") }}
	r := gin.New()
	r.GET("/api/ready", api.ready)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/ready", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", w.Code)
	}
}

func TestHealthIsLiveness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	api := &API{Ready: func(context.Context) error { return fmt.Errorf("db down") }}
	r := api.Router()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}
