package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/judging"
)

func (a *API) judgeRoutes(r *gin.Engine) {
	r.GET("/api/judge/files/:token", a.judgeFile)
	r.POST("/api/pools", a.createPool)
	r.GET("/api/organizations/:id/pools", a.listPools)
	r.POST("/api/pools/:id/nodes", a.registerNode)
	r.GET("/api/pools/:id/nodes", a.listNodes)
	r.POST("/api/nodes/:id/rotate", a.rotateNode)
	r.POST("/api/nodes/:id/revoke", a.revokeNode)
	r.POST("/api/organizations/:id/routing", a.setRouting)
	r.GET("/api/organizations/:id/routing", a.getRouting)
	r.POST("/api/tasks/:id/redispatch", a.redispatch)
	r.GET("/api/contests/:id/tasks", a.listTasks)
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

func (a *API) listPools(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	pools, err := a.Judging.ListPools(c.Request.Context(), p, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"pools": pools})
}

func (a *API) listNodes(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	nodes, err := a.Judging.ListNodes(c.Request.Context(), p, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
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

func (a *API) getRouting(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	entries, err := a.Judging.Routing(c.Request.Context(), p, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

func (a *API) listTasks(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	limit := 0
	if raw, ok := c.GetQuery("limit"); ok {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeErr(c, apperr.New(apperr.Invalid, "invalid limit"))
			return
		}
		limit = parsed
	}
	tasks, err := a.Judging.ListTasks(c.Request.Context(), p, id, c.Query("state"), limit)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
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
