package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/scoring"
)

const (
	ssePingEvery = 15 * time.Second
	ssePollEvery = time.Second
)

func (a *API) eventRoutes(r *gin.Engine) {
	r.GET("/api/contests/:id/events", a.contestEvents)
}

func (a *API) contestEvents(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	if _, err := a.Contest.Get(c.Request.Context(), p, id); err != nil {
		writeErr(c, err)
		return
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		writeErr(c, apperr.New(apperr.Internal, "streaming unsupported"))
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()

	updates, cancel := a.Scoring.SubscribeUpdates()
	defer cancel()

	ping := time.NewTicker(ssePingEvery)
	defer ping.Stop()
	poll := time.NewTicker(ssePollEvery)
	defer poll.Stop()

	seen := map[uuid.UUID]string{}
	initialized := false
	ctx := c.Request.Context()

	emit := func(submissionID uuid.UUID) {
		sub, err := a.Submission.Get(ctx, submissionID)
		if err != nil {
			return
		}
		if sub.ContestID != id {
			return
		}
		dto, view, err := a.Scoring.ViewSubmission(ctx, p, sub)
		if err != nil || !scoring.ShouldEmit(view) {
			return
		}
		_ = writeSSEData(c.Writer, "submission", dto)
		flusher.Flush()
	}

	pollOnce := func() {
		fps, err := a.Scoring.WatchContest(ctx, id)
		if err != nil {
			return
		}
		for sid, fp := range fps {
			prev, ok := seen[sid]
			seen[sid] = fp
			if !initialized {
				continue
			}
			if !ok || prev != fp {
				emit(sid)
			}
		}
		initialized = true
	}

	pollOnce()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ping.C:
			writeSSEComment(c.Writer, "ping")
			flusher.Flush()
		case u, ok := <-updates:
			if !ok {
				return
			}
			if u.ContestID != id {
				continue
			}
			emit(u.SubmissionID)
		case <-poll.C:
			pollOnce()
		}
	}
}

func writeSSEComment(w http.ResponseWriter, comment string) {
	fmt.Fprintf(w, ": %s\n\n", comment)
}

func writeSSEData(w http.ResponseWriter, event string, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body)
	return err
}
