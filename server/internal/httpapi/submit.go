package httpapi

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

func (a *API) submitRoutes(r *gin.Engine) {
	r.POST("/api/contests/:id/problems/:letter/submissions", a.submit)
	r.GET("/api/submissions/:id", a.getSubmission)
	r.GET("/api/submissions/:id/source", a.getSource)
}

func (a *API) submit(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	letter := c.Param("letter")
	lang := c.PostForm("language")
	file, err := c.FormFile("source")
	if err != nil {
		writeErr(c, apperr.New(apperr.Invalid, "source is required"))
		return
	}
	f, err := file.Open()
	if err != nil {
		writeErr(c, err)
		return
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, domain.MaxSourceBytes+1))
	if err != nil {
		writeErr(c, err)
		return
	}
	res, err := a.Submission.Submit(c.Request.Context(), p, id, letter, lang, body)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"submission_id": res.SubmissionID,
		"receive_seq":   res.ReceiveSeq,
		"received_at":   res.ReceivedAt,
		"task_id":       res.TaskID,
	})
}

func (a *API) getSubmission(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	out, _, err := a.Scoring.ViewSubmission(c.Request.Context(), p, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (a *API) getSource(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	obj, err := a.Scoring.AuthorizedSource(c.Request.Context(), p, id)
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
	c.DataFromReader(http.StatusOK, n, ctype, rc, nil)
}
