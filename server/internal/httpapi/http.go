package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/contest"
	"github.com/jellyfish-p/ephyra/server/internal/identity"
	"github.com/jellyfish-p/ephyra/server/internal/judging"
	"github.com/jellyfish-p/ephyra/server/internal/objectstore"
	"github.com/jellyfish-p/ephyra/server/internal/problem"
	"github.com/jellyfish-p/ephyra/server/internal/scoring"
	"github.com/jellyfish-p/ephyra/server/internal/submission"
)

const (
	sessionCookie = "ephyra_session"
	csrfCookie    = "ephyra_csrf"
	csrfHeader    = "X-CSRF-Token"
)

type API struct {
	Identity   *identity.Module
	Contest    *contest.Module
	Problem    *problem.Module
	Submission *submission.Module
	Judging    *judging.Module
	Scoring    *scoring.Module
	Blobs      objectstore.Store
	Secure     bool
}

func (a *API) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(a.session)
	r.Use(a.csrf)
	r.GET("/api/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	a.authRoutes(r)
	a.orgRoutes(r)
	a.contestRoutes(r)
	a.problemRoutes(r)
	a.submitRoutes(r)
	a.eventRoutes(r)
	a.judgeRoutes(r)
	return r
}

func (a *API) session(c *gin.Context) {
	raw, _ := c.Cookie(sessionCookie)
	if raw == "" {
		c.Next()
		return
	}
	p, sess, err := a.Identity.Resolve(c.Request.Context(), raw)
	if err != nil {
		c.Next()
		return
	}
	c.Set("principal", p)
	c.Set("csrf", sess.CSRFSecret)
	c.Next()
}

func principal(c *gin.Context) access.Principal {
	v, ok := c.Get("principal")
	if !ok {
		return access.Principal{}
	}
	return v.(access.Principal)
}

func (a *API) csrf(c *gin.Context) {
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
		c.Next()
		return
	}
	if c.Request.URL.Path == "/api/judge/files/"+c.Param("token") {
		c.Next()
		return
	}
	header := c.GetHeader(csrfHeader)
	if header == "" {
		writeErr(c, apperr.New(apperr.Unauthorized, "missing CSRF token"))
		c.Abort()
		return
	}
	if secret, ok := c.Get("csrf"); ok {
		if header != secret.(string) {
			writeErr(c, apperr.New(apperr.Unauthorized, "invalid CSRF token"))
			c.Abort()
			return
		}
		c.Next()
		return
	}
	cookie, err := c.Cookie(csrfCookie)
	if err != nil || cookie == "" || cookie != header {
		writeErr(c, apperr.New(apperr.Unauthorized, "invalid CSRF token"))
		c.Abort()
		return
	}
	c.Next()
}

func (a *API) setSession(c *gin.Context, info identity.SessionInfo) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: sessionCookie, Value: info.RawToken, Path: "/", HttpOnly: true,
		Secure: a.Secure, SameSite: http.SameSiteLaxMode, Expires: info.ExpiresAt,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name: csrfCookie, Value: info.CSRFSecret, Path: "/", HttpOnly: false,
		Secure: a.Secure, SameSite: http.SameSiteLaxMode, Expires: info.ExpiresAt,
	})
}

func (a *API) clearSession(c *gin.Context) {
	past := time.Unix(0, 0)
	http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookie, Path: "/", HttpOnly: true, Secure: a.Secure, Expires: past, MaxAge: -1})
	http.SetCookie(c.Writer, &http.Cookie{Name: csrfCookie, Path: "/", Secure: a.Secure, Expires: past, MaxAge: -1})
}

func writeErr(c *gin.Context, err error) {
	code := http.StatusInternalServerError
	msg := err.Error()
	ec := apperr.Internal
	if e, ok := apperr.As(err); ok {
		ec = e.Code
		msg = e.Msg
		switch e.Code {
		case apperr.Unauthorized:
			code = http.StatusUnauthorized
		case apperr.Forbidden, apperr.Unverified:
			code = http.StatusForbidden
		case apperr.NotFound:
			code = http.StatusNotFound
		case apperr.Conflict:
			code = http.StatusConflict
		case apperr.Invalid:
			code = http.StatusBadRequest
		case apperr.TooMany:
			code = http.StatusTooManyRequests
		case apperr.Gone:
			code = http.StatusGone
		case apperr.Unavailable:
			code = http.StatusServiceUnavailable
		}
	}
	c.JSON(code, gin.H{"error": gin.H{"code": ec, "message": msg}})
}

func requireUser(c *gin.Context) (access.Principal, bool) {
	p := principal(c)
	if p.Zero() {
		writeErr(c, apperr.New(apperr.Unauthorized, "not signed in"))
		return p, false
	}
	return p, true
}

func parseUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		writeErr(c, apperr.New(apperr.Invalid, "invalid id"))
		return uuid.Nil, false
	}
	return id, true
}
