package httpapi

import (
	"context"
	"log/slog"
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
	Ready      func(context.Context) error
}

func (a *API) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestID)
	r.Use(a.session)
	r.Use(a.csrf)
	r.GET("/api/health", func(c *gin.Context) { c.JSON(http.StatusOK, okResponse{OK: true}) })
	r.GET("/api/ready", a.ready)
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
	p, csrfSecret, err := a.Identity.Resolve(c.Request.Context(), raw)
	if err != nil {
		c.Next()
		return
	}
	c.Set("principal", p)
	c.Set("csrf", csrfSecret)
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

func requestID(c *gin.Context) {
	id := c.GetHeader("X-Request-ID")
	if id == "" {
		id = uuid.NewString()
	}
	c.Set("request_id", id)
	c.Writer.Header().Set("X-Request-ID", id)
	c.Next()
}

func (a *API) ready(c *gin.Context) {
	if a.Ready == nil {
		c.JSON(http.StatusOK, okResponse{OK: true})
		return
	}
	if err := a.Ready(c.Request.Context()); err != nil {
		slog.ErrorContext(c.Request.Context(), "ready check failed", "err", err, "request_id", c.GetString("request_id"))
		c.JSON(http.StatusServiceUnavailable, okResponse{OK: false})
		return
	}
	c.JSON(http.StatusOK, okResponse{OK: true})
}

func writeErr(c *gin.Context, err error) {
	if e, ok := apperr.As(err); ok {
		if e.Code == apperr.Internal {
			slog.ErrorContext(c.Request.Context(), "internal error",
				"err", err, "request_id", c.GetString("request_id"),
				"method", c.Request.Method, "path", c.Request.URL.Path)
		}
		c.JSON(statusFor(e.Code), gin.H{"error": gin.H{"code": e.Code, "message": e.Msg}})
		return
	}
	slog.ErrorContext(c.Request.Context(), "unhandled error",
		"err", err, "request_id", c.GetString("request_id"),
		"method", c.Request.Method, "path", c.Request.URL.Path)
	c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": apperr.Internal, "message": "internal error"}})
}

func statusFor(code apperr.Code) int {
	switch code {
	case apperr.Unauthorized:
		return http.StatusUnauthorized
	case apperr.Forbidden, apperr.Unverified:
		return http.StatusForbidden
	case apperr.NotFound:
		return http.StatusNotFound
	case apperr.Conflict:
		return http.StatusConflict
	case apperr.Invalid:
		return http.StatusBadRequest
	case apperr.TooMany:
		return http.StatusTooManyRequests
	case apperr.Gone:
		return http.StatusGone
	case apperr.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
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
