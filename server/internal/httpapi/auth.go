package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jellyfish-p/ephyra/server/internal/sec"
)

func (a *API) authRoutes(r *gin.Engine) {
	r.GET("/api/auth/csrf", a.csrfToken)
	r.POST("/api/auth/register", a.register)
	r.POST("/api/auth/login", a.login)
	r.POST("/api/auth/logout", a.logout)
	r.POST("/api/auth/verify-email", a.verifyEmail)
	r.POST("/api/auth/forgot-password", a.forgot)
	r.POST("/api/auth/reset-password", a.reset)
	r.GET("/api/me", a.me)
}

func (a *API) csrfToken(c *gin.Context) {
	if _, err := c.Cookie(csrfCookie); err == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	tok, err := sec.RandomToken(32)
	if err != nil {
		writeErr(c, err)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name: csrfCookie, Value: tok, Path: "/", HttpOnly: false,
		Secure: a.Secure, SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(12 * time.Hour),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) register(c *gin.Context) {
	var in struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	if err := a.Identity.Register(c.Request.Context(), in.Email, in.Password, in.DisplayName, c.ClientIP()); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true})
}

func (a *API) login(c *gin.Context) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	info, err := a.Identity.Login(c.Request.Context(), in.Email, in.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeErr(c, err)
		return
	}
	a.setSession(c, info)
	c.JSON(http.StatusOK, gin.H{
		"account_id": info.Account.ID, "email": info.Account.Email,
		"display_name": info.Account.DisplayName, "email_verified": info.Account.EmailVerified,
		"site_admin": info.Account.SiteAdmin,
	})
}

func (a *API) logout(c *gin.Context) {
	raw, _ := c.Cookie(sessionCookie)
	_ = a.Identity.Logout(c.Request.Context(), raw)
	a.clearSession(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) verifyEmail(c *gin.Context) {
	var in struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	if err := a.Identity.VerifyEmail(c.Request.Context(), in.Token); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) forgot(c *gin.Context) {
	var in struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	_ = a.Identity.RequestPasswordReset(c.Request.Context(), in.Email, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) reset(c *gin.Context) {
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	if err := a.Identity.ResetPassword(c.Request.Context(), in.Token, in.Password); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) me(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"account_id": p.AccountID, "email": p.Email, "display_name": p.DisplayName,
		"email_verified": p.EmailVerified, "site_admin": p.SiteAdmin,
	})
}
