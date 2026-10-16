package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (a *API) orgRoutes(r *gin.Engine) {
	r.POST("/api/organizations", a.createOrg)
	r.GET("/api/organizations/:id", a.getOrg)
	r.POST("/api/organizations/:id/join", a.joinOrg)
	r.POST("/api/organizations/:orgId/join-requests/:requestId/decision", a.decideJoin)
}

func (a *API) createOrg(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	var in struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	org, err := a.Identity.CreateOrganization(c.Request.Context(), p, in.Slug, in.Name)
	if err != nil {
		writeErr(c, err)
		return
	}
	_ = a.Judging.EnsureDefaultRouting(c.Request.Context(), org.ID)
	c.JSON(http.StatusCreated, gin.H{"id": org.ID, "slug": org.Slug, "name": org.Name})
}

func (a *API) getOrg(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	org, err := a.Identity.Organization(c.Request.Context(), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": org.ID, "slug": org.Slug, "name": org.Name})
}

func (a *API) joinOrg(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	if err := a.Identity.ApplyToJoin(c.Request.Context(), p, id); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
}

func (a *API) decideJoin(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	rid, err := uuid.Parse(c.Param("requestId"))
	if err != nil {
		writeErr(c, err)
		return
	}
	var in struct {
		Approve bool `json:"approve"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	if err := a.Identity.DecideJoin(c.Request.Context(), p, rid, in.Approve); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
