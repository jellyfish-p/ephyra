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
		"submission_id": res.Submission.ID,
		"receive_seq":   res.Submission.ReceiveSeq,
		"received_at":   res.Submission.ReceivedAt,
		"task_id":       res.Task.ID,
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
	sub, err := a.Submission.Get(c.Request.Context(), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	view := a.Scoring.FeedbackLevel(c.Request.Context(), p, sub)
	out := gin.H{
		"id": sub.ID, "contest_id": sub.ContestID, "entrant_id": sub.EntrantID,
		"submitter_account_id": sub.SubmitterAccountID, "language_id": sub.LanguageID,
		"received_at": sub.ReceivedAt, "receive_seq": sub.ReceiveSeq,
		"revision_id": sub.JudgeDataRevisionID, "feedback": view.Feedback,
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
	sub, err := a.Submission.Get(c.Request.Context(), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	if err := a.Scoring.CanReadSource(c.Request.Context(), p, sub); err != nil {
		writeErr(c, err)
		return
	}
	obj, err := a.Judging.Object(c.Request.Context(), sub.SourceObjectID)
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
