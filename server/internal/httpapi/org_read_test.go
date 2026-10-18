package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
)

// orgReadRouter mounts only the routes added for frontend reads, with a fixed
// principal so authorization is exercised without a database.
func orgReadRouter(api *API, p access.Principal) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if !p.Zero() {
			c.Set("principal", p)
		}
		c.Next()
	})
	api.orgRoutes(r)
	api.judgeRoutes(r)
	api.problemRoutes(r)
	return r
}

func TestOrganizationReadRoutesRequireSession(t *testing.T) {
	r := (&API{}).Router()
	id := uuid.NewString()
	for _, path := range []string{
		"/api/organizations",
		"/api/me/organizations",
		"/api/organizations/" + id + "/pools",
		"/api/pools/" + id + "/nodes",
		"/api/organizations/" + id + "/routing",
	} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestOrganizationReadRoutesRejectInvalidID(t *testing.T) {
	r := orgReadRouter(&API{}, access.Principal{AccountID: uuid.New()})
	for _, path := range []string{
		"/api/organizations/not-a-uuid/pools",
		"/api/pools/not-a-uuid/nodes",
		"/api/organizations/not-a-uuid/routing",
	} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
		})
	}
}
