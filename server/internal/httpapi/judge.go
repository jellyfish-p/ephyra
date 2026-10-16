package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/judging"
)

func (a *API) judgeRoutes(r *gin.Engine) {
	r.GET("/api/judge/files/:token", a.judgeFile)
	r.POST("/api/pools", a.createPool)
	r.POST("/api/pools/:id/nodes", a.registerNode)
	r.POST("/api/nodes/:id/rotate", a.rotateNode)
	r.POST("/api/nodes/:id/revoke", a.revokeNode)
	r.POST("/api/organizations/:id/routing", a.setRouting)
	r.POST("/api/tasks/:id/redispatch", a.redispatch)
	r.POST("/api/contest-problems/:id/rejudge", a.rejudge)
	r.POST("/api/rejudge-batches/:id/switch", a.switchBatch)
}

func (a *API) judgeFile(c *gin.Context) {
	obj, err := a.Judging.OpenFile(c.Request.Context(), c.Param("token"))
	if err != nil {
		writeErr(c, err)
		return
	}
	rc, n, ctype, err := a.Blobs.Get(c.Request.Context(), obj.StorageKey)
	if err != nil {
		writeErr(c, err)
		return
	}
	defer rc.Close()
	c.Header("X-Content-SHA256", obj.SHA256)
	c.DataFromReader(http.StatusOK, n, ctype, rc, nil)
}

func (a *API) createPool(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	var in struct {
		Kind           string     `json:"kind"`
		OrganizationID *uuid.UUID `json:"organization_id"`
		Name           string     `json:"name"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	pool, err := a.Judging.CreatePool(c.Request.Context(), p, domain.PoolKind(in.Kind), in.OrganizationID, in.Name)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, pool)
}

func (a *API) registerNode(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	node, err := a.Judging.RegisterNode(c.Request.Context(), p, id, in.Name)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, node)
}

func (a *API) rotateNode(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	cred, err := a.Judging.RotateCredential(c.Request.Context(), p, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, cred)
}

func (a *API) revokeNode(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	if err := a.Judging.RevokeNode(c.Request.Context(), p, id); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, judging.Ok())
}

func (a *API) setRouting(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var in struct {
		Entries []judging.RoutingEntry `json:"entries"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	if err := a.Judging.SetRouting(c.Request.Context(), p, id, in.Entries); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, judging.Ok())
}

func (a *API) redispatch(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	if err := a.Judging.Redispatch(c.Request.Context(), p, id); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, judging.Ok())
}

func (a *API) rejudge(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var in struct {
		RevisionID uuid.UUID `json:"revision_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	batch, err := a.Judging.CreateRejudge(c.Request.Context(), p, id, in.RevisionID)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusAccepted, batch)
}

func (a *API) switchBatch(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	if err := a.Judging.SwitchRejudge(c.Request.Context(), p, id); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, judging.Ok())
}
