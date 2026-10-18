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
		c.JSON(http.StatusOK, okResponse{OK: true})
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
	c.JSON(http.StatusOK, okResponse{OK: true})
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

func (a *API) register(c *gin.Context) {
	var in registerRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	if err := a.Identity.Register(c.Request.Context(), in.Email, in.Password, in.DisplayName, c.ClientIP()); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, okResponse{OK: true})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *API) login(c *gin.Context) {
	var in loginRequest
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
	c.JSON(http.StatusOK, accountResponse{
		AccountID: info.Account.ID, Email: info.Account.Email,
		DisplayName: info.Account.DisplayName, EmailVerified: info.Account.EmailVerified,
		SiteAdmin: info.Account.SiteAdmin,
	})
}

func (a *API) logout(c *gin.Context) {
	raw, _ := c.Cookie(sessionCookie)
	_ = a.Identity.Logout(c.Request.Context(), raw)
	a.clearSession(c)
	c.JSON(http.StatusOK, okResponse{OK: true})
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

func (a *API) verifyEmail(c *gin.Context) {
	var in verifyEmailRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	if err := a.Identity.VerifyEmail(c.Request.Context(), in.Token); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, okResponse{OK: true})
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

func (a *API) forgot(c *gin.Context) {
	var in forgotPasswordRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	_ = a.Identity.RequestPasswordReset(c.Request.Context(), in.Email, c.ClientIP())
	c.JSON(http.StatusOK, okResponse{OK: true})
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (a *API) reset(c *gin.Context) {
	var in resetPasswordRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	if err := a.Identity.ResetPassword(c.Request.Context(), in.Token, in.Password); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, okResponse{OK: true})
}

func (a *API) me(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, accountResponse{
		AccountID: p.AccountID, Email: p.Email, DisplayName: p.DisplayName,
		EmailVerified: p.EmailVerified, SiteAdmin: p.SiteAdmin,
	})
}
