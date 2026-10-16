package httpapi

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/contest"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

func (a *API) contestRoutes(r *gin.Engine) {
	r.POST("/api/organizations/:id/contests", a.createContest)
	r.GET("/api/organizations/:id/contests", a.listOrgContests)
	r.GET("/api/organizations/:id/problems", a.listOrgProblems)
	r.GET("/api/contests/:id", a.getContest)
	r.PATCH("/api/contests/:id/source-public", a.setSourcePublic)
	r.GET("/api/contests/:id/problems", a.listContestProblems)
	r.POST("/api/contests/:id/problems", a.addContestProblem)
	r.POST("/api/contests/:id/register", a.registerIndividual)
	r.POST("/api/contests/:id/teams", a.createTeam)
	r.POST("/api/entrants/:id/invite", a.invite)
	r.POST("/api/entrants/:id/confirm", a.confirmInvite)
	r.POST("/api/entrants/:id/submit-registration", a.submitTeam)
	r.POST("/api/entrants/:id/decision", a.decideReg)
	r.POST("/api/entrants/:id/withdraw", a.withdraw)
	r.GET("/api/contests/:id/standings", a.standings)
	r.POST("/api/organizations/:id/problems/import", a.importProblem)
	r.POST("/api/problems/:id/revisions", a.addRevision)
}

func (a *API) createContest(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	orgID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var in struct {
		Slug                       string    `json:"slug"`
		Title                      string    `json:"title"`
		Rule                       string    `json:"rule"`
		EntryMode                  string    `json:"entry_mode"`
		StartsAt                   time.Time `json:"starts_at"`
		EndsAt                     time.Time `json:"ends_at"`
		RegisterStartsAt           time.Time `json:"register_starts_at"`
		AllowExternal              bool      `json:"allow_external"`
		RegistrationRequiresReview bool      `json:"registration_requires_review"`
		MaxTeamSize                int       `json:"max_team_size"`
		ACMPenaltyMinutes          int       `json:"acm_penalty_minutes"`
		FreezeMinutesBeforeEnd     *int      `json:"freeze_minutes_before_end"`
		IOIPublicScoreboard        bool      `json:"ioi_public_scoreboard"`
		CaseFeedback               string    `json:"case_feedback"`
		SourcePublicAfter          bool      `json:"source_public_after"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	rule, okR := domain.ParseRule(in.Rule)
	mode, okM := domain.ParseEntryMode(in.EntryMode)
	if !okR || !okM {
		writeErr(c, apperr.New(apperr.Invalid, "invalid rule or entry mode"))
		return
	}
	ct, err := a.Contest.Create(c.Request.Context(), p, contest.CreateInput{
		OrganizationID: orgID, Slug: in.Slug, Title: in.Title, Rule: rule, EntryMode: mode,
		StartsAt: in.StartsAt, EndsAt: in.EndsAt, RegisterStartsAt: in.RegisterStartsAt,
		AllowExternal: in.AllowExternal, RegistrationRequiresReview: in.RegistrationRequiresReview,
		MaxTeamSize: in.MaxTeamSize, ACMPenaltyMinutes: in.ACMPenaltyMinutes,
		FreezeMinutesBeforeEnd: in.FreezeMinutesBeforeEnd, IOIPublicScoreboard: in.IOIPublicScoreboard,
		CaseFeedback: domain.FeedbackGranularity(in.CaseFeedback), SourcePublicAfter: in.SourcePublicAfter,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, contestDTO(ct))
}

func (a *API) listOrgContests(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	orgID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	list, err := a.Contest.ListByOrganization(c.Request.Context(), p, orgID)
	if err != nil {
		writeErr(c, err)
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, ct := range list {
		out = append(out, contestDTO(ct))
	}
	c.JSON(http.StatusOK, gin.H{"contests": out})
}

func (a *API) listOrgProblems(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	orgID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	list, err := a.Contest.ListOrgProblems(c.Request.Context(), p, orgID)
	if err != nil {
		writeErr(c, err)
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, pbm := range list {
		out = append(out, gin.H{"id": pbm.ID, "slug": pbm.Slug, "title": pbm.Title})
	}
	c.JSON(http.StatusOK, gin.H{"problems": out})
}

func (a *API) getContest(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	ct, err := a.Contest.Get(c.Request.Context(), p, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, contestDTO(ct))
}

func (a *API) setSourcePublic(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var in struct {
		SourcePublicAfter bool `json:"source_public_after"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	if err := a.Contest.SetSourcePublicAfter(c.Request.Context(), p, id, in.SourcePublicAfter); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) listContestProblems(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	list, err := a.Contest.ListProblems(c.Request.Context(), p, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, cp := range list {
		item := gin.H{"letter": cp.Letter}
		if cp.Title != "" {
			item["title"] = cp.Title
		}
		if cp.ProblemID != nil {
			item["problem_id"] = *cp.ProblemID
		}
		if cp.ScoringRevisionID != nil {
			item["scoring_revision_id"] = *cp.ScoringRevisionID
		}
		if cp.TargetRevisionID != nil {
			item["target_revision_id"] = *cp.TargetRevisionID
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"problems": out})
}

func contestDTO(ct contest.View) gin.H {
	return gin.H{
		"id": ct.ID, "organization_id": ct.OrganizationID, "slug": ct.Slug, "title": ct.Title,
		"rule": ct.Rule, "entry_mode": ct.EntryMode, "starts_at": ct.StartsAt, "ends_at": ct.EndsAt,
		"register_starts_at": ct.RegisterStartsAt, "allow_external": ct.AllowExternal,
		"source_public_after": ct.SourcePublicAfter, "source_visibility_locked": ct.SourceVisibilityLocked,
		"published_at": ct.PublishedAt,
	}
}

func (a *API) addContestProblem(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var in struct {
		ProblemID  uuid.UUID `json:"problem_id"`
		RevisionID uuid.UUID `json:"revision_id"`
		Letter     string    `json:"letter"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	cp, err := a.Contest.AddProblem(c.Request.Context(), p, id, in.ProblemID, in.RevisionID, in.Letter)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": cp.ID, "letter": cp.Letter, "problem_id": cp.ProblemID, "revision_id": cp.RevisionID})
}

func (a *API) registerIndividual(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	en, err := a.Contest.RegisterIndividual(c.Request.Context(), p, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"entrant_id": en.ID, "status": en.Status})
}

func (a *API) createTeam(c *gin.Context) {
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
	en, err := a.Contest.CreateTeam(c.Request.Context(), p, id, in.Name)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"entrant_id": en.ID, "status": en.Status})
}

func (a *API) invite(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var in struct {
		AccountID uuid.UUID `json:"account_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	if err := a.Contest.Invite(c.Request.Context(), p, id, in.AccountID); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) confirmInvite(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	if err := a.Contest.ConfirmInvite(c.Request.Context(), p, id); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) submitTeam(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	if err := a.Contest.SubmitTeam(c.Request.Context(), p, id); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) decideReg(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var in struct {
		Approve bool `json:"approve"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeErr(c, err)
		return
	}
	if err := a.Contest.DecideRegistration(c.Request.Context(), p, id, in.Approve); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) withdraw(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	if err := a.Contest.Withdraw(c.Request.Context(), p, id); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) standings(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	rows, err := a.Scoring.Standings(c.Request.Context(), principal(c), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"standings": rows})
}

func (a *API) importProblem(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	orgID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	file, err := c.FormFile("zip")
	if err != nil {
		writeErr(c, apperr.New(apperr.Invalid, "zip is required"))
		return
	}
	f, err := file.Open()
	if err != nil {
		writeErr(c, err)
		return
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, domain.MaxUncompressedZip+1))
	if err != nil {
		writeErr(c, err)
		return
	}
	res, err := a.Problem.ImportHydro(c.Request.Context(), p, orgID, c.PostForm("slug"), c.PostForm("title"), body)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"problem_id": res.ProblemID, "revision_id": res.RevisionID, "revision_n": res.RevisionN})
}

func (a *API) addRevision(c *gin.Context) {
	p, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	file, err := c.FormFile("zip")
	if err != nil {
		writeErr(c, apperr.New(apperr.Invalid, "zip is required"))
		return
	}
	f, err := file.Open()
	if err != nil {
		writeErr(c, err)
		return
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, domain.MaxUncompressedZip+1))
	if err != nil {
		writeErr(c, err)
		return
	}
	rev, err := a.Problem.AddRevision(c.Request.Context(), p, id, body)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"revision_id": rev.ID, "revision_n": rev.RevisionN})
}

func (a *API) problemRoutes(r *gin.Engine) {}
