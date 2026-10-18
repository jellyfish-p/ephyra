package httpapi

import (
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/contest"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/identity"
	"github.com/jellyfish-p/ephyra/server/internal/judging"
	"github.com/jellyfish-p/ephyra/server/internal/openapi"
	"github.com/jellyfish-p/ephyra/server/internal/problem"
	"github.com/jellyfish-p/ephyra/server/internal/scoring"
)

// OpenAPIDocument describes the API this package serves.
//
// Paths and methods are read from the real router rather than listed here, so a
// route that is registered but not described below fails the build. What the
// table adds is the part reflection cannot recover: the payload type behind each
// gin.H envelope, the query parameters, and the success status.
func OpenAPIDocument() (*openapi.Document, error) {
	engine := (&API{}).Router()
	registered := engine.Routes()
	routes := make([]openapi.Route, 0, len(registered))
	for _, r := range registered {
		routes = append(routes, openapi.Route{Method: r.Method, Path: r.Path, Handler: r.Handler})
	}

	return openapi.Build(openapi.BuildInput{
		Title:   "Ephyra Judge API",
		Version: "1.0.0",
		Description: "Online judge API. Authentication is a session cookie plus a CSRF header, " +
			"both same-origin: fetch /api/auth/csrf, send the readable ephyra_csrf cookie back as " +
			"X-CSRF-Token on every unsafe method. Fields are omitted rather than nulled when the " +
			"caller is not allowed to see them, so a client must branch on presence.",
		Servers: []openapi.Server{{
			URL:         "/",
			Description: "Same origin. The frontend proxies /api to the backend, so no CORS is involved.",
		}},
		Tags:   openAPITags,
		Routes: routes,
		Specs:  openAPIRoutes,
		Enums:  openAPIEnums,
		Fields: openAPIFields,
	})
}

var openAPITags = []openapi.Tag{
	{Name: "auth", Description: "Accounts, sessions and the CSRF handshake."},
	{Name: "organizations", Description: "Organizations, their members, pools and problem bank."},
	{Name: "contests", Description: "Contests, their problems, entrants and standings."},
	{Name: "submissions", Description: "Submitting source and reading results."},
	{Name: "judging", Description: "Judge pools, nodes, routing and rejudging."},
	{Name: "events", Description: "Live contest updates."},
	{Name: "system", Description: "Liveness and readiness probes."},
}

// openAPIEnums names the typed string constants the API writes, read from their
// source so a new constant shows up in the document without an edit here.
var openAPIEnums = []openapi.EnumSource{
	{File: "internal/apperr/apperr.go", Type: "Code", Target: apperr.Code("")},
	{File: "internal/domain/domain.go", Type: "Rule", Target: domain.Rule("")},
	{File: "internal/domain/domain.go", Type: "EntryMode", Target: domain.EntryMode("")},
	{File: "internal/domain/domain.go", Type: "Verdict", Target: domain.Verdict("")},
	{File: "internal/domain/domain.go", Type: "LanguageID", Target: domain.LanguageID("")},
	{File: "internal/domain/domain.go", Type: "ContestPhase", Target: domain.ContestPhase("")},
	{File: "internal/domain/domain.go", Type: "OrgRole", Target: domain.OrgRole("")},
	{File: "internal/domain/domain.go", Type: "PoolKind", Target: domain.PoolKind("")},
	{File: "internal/domain/domain.go", Type: "TaskState", Target: domain.TaskState("")},
	{File: "internal/domain/domain.go", Type: "AttemptState", Target: domain.AttemptState("")},
	{File: "internal/domain/domain.go", Type: "EntrantStatus", Target: domain.EntrantStatus("")},
	{File: "internal/domain/domain.go", Type: "ProblemKind", Target: domain.ProblemKind("")},
	{File: "internal/domain/domain.go", Type: "CheckerKind", Target: domain.CheckerKind("")},
}

// openAPIFields attaches enums to the response fields that carry them as a bare
// string. Several of these values are written as literals in the handlers rather
// than declared as a type, which is why some list their values directly.
var openAPIFields = []openapi.FieldSpec{
	// The error envelope is shared by every failure, so its code union is the one
	// clients branch on most.
	{
		Owner: openapi.ErrorBody{}, Field: "code", EnumType: apperr.Code(""),
		Description: "Machine-readable failure kind; the message is for humans.",
	},

	{Owner: contestResponse{}, Field: "rule", EnumType: domain.Rule("")},
	{Owner: contestResponse{}, Field: "entry_mode", EnumType: domain.EntryMode("")},
	{Owner: entrantCreatedResponse{}, Field: "status", EnumType: domain.EntrantStatus("")},

	{Owner: contest.ProblemDetail{}, Field: "kind", EnumType: domain.ProblemKind("")},
	{Owner: contest.ProblemDetail{}, Field: "checker_kind", EnumType: domain.CheckerKind("")},
	{Owner: contest.MyEntrantView{}, Field: "kind", EnumType: domain.EntryMode("")},
	{Owner: contest.MyEntrantView{}, Field: "status", EnumType: domain.EntrantStatus("")},

	{Owner: scoring.SubmissionPublic{}, Field: "language_id", EnumType: domain.LanguageID("")},
	{Owner: scoring.SubmissionPublic{}, Field: "verdict", EnumType: domain.Verdict("")},
	{Owner: scoring.SubtaskPublic{}, Field: "verdict", EnumType: domain.Verdict("")},
	{Owner: scoring.CasePublic{}, Field: "verdict", EnumType: domain.Verdict("")},
	{Owner: contest.EntrantMemberView{}, Field: "role", Values: []string{"captain", "member"}},
	{
		Owner: scoring.SubmissionPublic{}, Field: "feedback",
		Values:      []string{"none", "compile", "score", "full"},
		Description: "How much detail this viewer may see: nothing, the compile output, per-subtask scores, or everything including per-case results.",
	},

	{Owner: judging.TaskView{}, Field: "state", EnumType: domain.TaskState("")},
	{
		Owner: judging.TaskView{}, Field: "purpose",
		Values:      []string{"initial", "rejudge"},
		Description: "Why the task exists: the original submission, or a rejudge that replaced it.",
	},
	{Owner: judging.AttemptView{}, Field: "state", EnumType: domain.AttemptState("")},
	{Owner: judging.PoolView{}, Field: "kind", EnumType: domain.PoolKind("")},
	{Owner: judging.RejudgeBatchView{}, Field: "state", Values: []string{"running", "switched"}},

	{Owner: identity.Member{}, Field: "role", EnumType: domain.OrgRole("")},
	{Owner: identity.JoinRequest{}, Field: "status", Values: []string{"pending", "approved", "rejected"}},
	{Owner: membershipResponse{}, Field: "role", EnumType: domain.OrgRole("")},
}

var openAPIRoutes = []openapi.RouteSpec{
	// --- system -------------------------------------------------------------

	{
		Method: "GET", Path: "/api/health", Tag: "system", Public: true,
		Summary:  "Liveness probe",
		Response: okResponse{},
	},
	{
		Method: "GET", Path: "/api/ready", Tag: "system", Public: true,
		Summary: "Readiness probe, reporting 503 while a dependency is unreachable",
		// 503 is the point of this route, so it is documented rather than left
		// to the shared default.
		Response: okResponse{}, Status: 200,
	},

	// --- auth ---------------------------------------------------------------

	{
		Method: "GET", Path: "/api/auth/csrf", Tag: "auth", Public: true,
		Summary: "Ensure the readable CSRF cookie exists",
		// Public on purpose: the cookie has to be obtainable before there is a
		// session to authenticate with.
		Response: okResponse{},
	},
	{
		Method: "POST", Path: "/api/auth/register", Tag: "auth", Public: true, Status: 201,
		Summary:  "Create an account",
		Request:  registerRequest{},
		Response: accountResponse{},
	},
	{
		Method: "POST", Path: "/api/auth/login", Tag: "auth", Public: true,
		Summary:  "Start a session",
		Request:  loginRequest{},
		Response: accountResponse{},
	},
	{
		Method: "POST", Path: "/api/auth/logout", Tag: "auth", Public: true,
		Summary: "End the session, if there is one",
		// Idempotent and deliberately public, so a stale cookie can always be
		// cleared.
		Response: okResponse{},
	},
	{
		Method: "POST", Path: "/api/auth/verify-email", Tag: "auth", Public: true,
		Summary:  "Confirm an email address with a token",
		Request:  verifyEmailRequest{},
		Response: okResponse{},
	},
	{
		Method: "POST", Path: "/api/auth/forgot-password", Tag: "auth", Public: true,
		Summary: "Request a password reset token",
		// Always succeeds, whether or not the address is registered.
		Request:  forgotPasswordRequest{},
		Response: okResponse{},
	},
	{
		Method: "POST", Path: "/api/auth/reset-password", Tag: "auth", Public: true,
		Summary:  "Set a new password with a reset token",
		Request:  resetPasswordRequest{},
		Response: okResponse{},
	},
	{
		Method: "GET", Path: "/api/me", Tag: "auth",
		Summary:  "The signed-in account",
		Response: accountResponse{},
	},
	{
		Method: "GET", Path: "/api/me/organizations", Tag: "organizations",
		Summary:     "The organizations the caller belongs to, with their role",
		Response:    identity.MyOrganizationView{},
		EnvelopeKey: "organizations",
	},

	// --- organizations ------------------------------------------------------

	{
		Method: "GET", Path: "/api/organizations", Tag: "organizations",
		Summary:     "List every organization",
		Response:    identity.OrganizationView{},
		EnvelopeKey: "organizations",
	},
	{
		Method: "GET", Path: "/api/organizations/:id", Tag: "organizations", Public: true,
		Summary: "Read one organization by id",
		// Public so a join link works before signing in.
		Response: orgSummaryResponse{},
	},
	{
		Method: "GET", Path: "/api/organizations/:id/members", Tag: "organizations",
		Summary:     "List the members of an organization",
		Response:    identity.Member{},
		EnvelopeKey: "members",
	},
	{
		Method: "GET", Path: "/api/organizations/:id/membership", Tag: "organizations",
		Summary:  "The caller's own membership, if any",
		Response: membershipResponse{},
	},
	{
		Method: "GET", Path: "/api/organizations/:id/problems", Tag: "organizations",
		Summary:     "List the problem bank of an organization",
		Response:    contest.BankProblem{},
		EnvelopeKey: "problems",
	},
	{
		Method: "GET", Path: "/api/organizations/:id/pools", Tag: "organizations",
		Summary:     "List the judge pools owned by an organization",
		Response:    judging.PoolView{},
		EnvelopeKey: "pools",
	},
	{
		Method: "GET", Path: "/api/organizations/:id/join-requests", Tag: "organizations",
		Summary:     "List pending join requests, for an organization manager",
		Response:    identity.JoinRequest{},
		EnvelopeKey: "join_requests",
	},
	{
		Method: "GET", Path: "/api/organizations/:id/contests", Tag: "organizations",
		Summary:     "List the contests an organization runs",
		Response:    contestResponse{},
		EnvelopeKey: "contests",
	},
	{
		Method: "GET", Path: "/api/organizations/:id/routing", Tag: "organizations",
		Summary:     "Read the problem-kind to pool routing of an organization",
		Response:    judging.RoutingEntry{},
		EnvelopeKey: "entries",
	},
	{
		Method: "POST", Path: "/api/organizations", Tag: "organizations", Status: 201,
		Summary:  "Create an organization; the caller becomes its owner",
		Request:  createOrgRequest{},
		Response: orgSummaryResponse{},
	},
	{
		Method: "POST", Path: "/api/organizations/:id/join", Tag: "organizations", Status: 202,
		Summary:  "Ask to join an organization",
		Response: okResponse{},
	},
	{
		Method: "POST", Path: "/api/organizations/:id/join-requests/:requestId/decision", Tag: "organizations",
		Summary:  "Approve or reject a join request",
		Request:  decideJoinRequest{},
		Response: okResponse{},
	},
	{
		Method: "POST", Path: "/api/organizations/:id/contests", Tag: "organizations", Status: 201,
		Summary:  "Create a contest inside an organization",
		Request:  createContestRequest{},
		Response: contestResponse{},
	},
	{
		Method: "POST", Path: "/api/organizations/:id/problems/import", Tag: "organizations", Status: 201,
		Summary: "Import a problem from a Hydro-style zip",
		// A zip cannot be expressed as JSON, so this one takes a multipart body.
		FormFields: map[string]any{"slug": "", "title": ""},
		FormFiles:  []string{"zip"},
		Response:   importResultResponse{},
	},
	{
		Method: "POST", Path: "/api/organizations/:id/routing", Tag: "organizations",
		Summary:  "Replace the problem-kind to pool routing",
		Request:  setRoutingRequest{},
		Response: judging.AckView{},
	},

	// --- contests -----------------------------------------------------------

	{
		Method: "GET", Path: "/api/contests/:id", Tag: "contests",
		Summary:  "Read one contest",
		Response: contestResponse{},
	},
	{
		Method: "GET", Path: "/api/contests/:id/problems", Tag: "contests",
		Summary:     "List the problems of a contest, with their letters",
		Response:    contest.ProblemView{},
		EnvelopeKey: "problems",
	},
	{
		Method: "GET", Path: "/api/contests/:id/problems/:letter", Tag: "contests",
		Summary: "Read one problem's statement",
		// Only the statement is ever exposed here: test data, checkers and
		// interactors are never part of this payload.
		Response: contest.ProblemDetail{},
	},
	{
		Method: "GET", Path: "/api/contests/:id/standings", Tag: "contests", Public: true,
		Summary:     "Read the scoreboard",
		Response:    scoring.Standing{},
		EnvelopeKey: "standings",
	},
	{
		Method: "GET", Path: "/api/contests/:id/submissions", Tag: "contests",
		Summary:     "List submissions in a contest",
		Response:    scoring.SubmissionPublic{},
		EnvelopeKey: "submissions",
		Query:       map[string]any{"entrant_id": "", "letter": "", "limit": 0, "before_seq": int64(0)},
	},
	{
		Method: "GET", Path: "/api/contests/:id/entrants", Tag: "contests",
		Summary:     "List the entrants of a contest",
		Response:    contest.MyEntrantView{},
		EnvelopeKey: "entrants",
		Query:       map[string]any{"status": ""},
	},
	{
		Method: "GET", Path: "/api/contests/:id/events", Tag: "events",
		Summary: "Stream live contest updates over SSE",
		// Server-sent events, projected per viewer, so no JSON schema applies.
		ContentType: "text/event-stream",
	},
	{
		Method: "GET", Path: "/api/contests/:id/my-entrant", Tag: "contests",
		Summary:  "The caller's own entrant in a contest, or null",
		Response: myEntrantResponse{},
	},
	{
		Method: "GET", Path: "/api/contests/:id/tasks", Tag: "judging",
		Summary:     "List judging tasks, for a contest admin",
		Response:    judging.TaskView{},
		EnvelopeKey: "tasks",
		Query:       map[string]any{"state": "", "limit": 0},
	},
	{
		Method: "POST", Path: "/api/contests/:id/problems", Tag: "contests", Status: 201,
		Summary:  "Attach a problem from the bank to a contest",
		Request:  addContestProblemRequest{},
		Response: contestProblemCreatedResponse{},
	},
	{
		Method: "POST", Path: "/api/contests/:id/problems/:letter/submissions", Tag: "submissions", Status: 202,
		Summary: "Submit source for judging",
		// The source arrives as an upload, not as a JSON string.
		FormFields: map[string]any{"language": ""},
		FormFiles:  []string{"source"},
		Response:   submitAcceptedResponse{},
	},
	{
		Method: "POST", Path: "/api/contests/:id/register", Tag: "contests", Status: 201,
		Summary:  "Register the caller as an individual entrant",
		Response: entrantCreatedResponse{},
	},
	{
		Method: "POST", Path: "/api/contests/:id/teams", Tag: "contests", Status: 201,
		Summary:  "Create a team entrant",
		Request:  createTeamRequest{},
		Response: entrantCreatedResponse{},
	},
	{
		Method: "PATCH", Path: "/api/contests/:id/source-public", Tag: "contests",
		Summary:  "Change whether sources become public after the contest",
		Request:  setSourcePublicRequest{},
		Response: contestResponse{},
	},

	// --- entrants -----------------------------------------------------------

	{
		Method: "POST", Path: "/api/entrants/:id/invite", Tag: "contests",
		Summary:  "Invite an account onto a team",
		Request:  inviteRequest{},
		Response: okResponse{},
	},
	{
		Method: "POST", Path: "/api/entrants/:id/confirm", Tag: "contests",
		Summary:  "Accept a team invitation",
		Response: okResponse{},
	},
	{
		Method: "POST", Path: "/api/entrants/:id/submit-registration", Tag: "contests",
		Summary:  "Submit a team registration for review",
		Response: okResponse{},
	},
	{
		Method: "POST", Path: "/api/entrants/:id/decision", Tag: "contests",
		Summary:  "Approve or reject a registration, for a contest admin",
		Request:  decideRegRequest{},
		Response: okResponse{},
	},
	{
		Method: "POST", Path: "/api/entrants/:id/withdraw", Tag: "contests",
		Summary:  "Withdraw an entrant",
		Response: okResponse{},
	},

	// --- submissions --------------------------------------------------------

	{
		Method: "GET", Path: "/api/submissions/:id", Tag: "submissions",
		Summary: "Read one submission",
		// Result fields appear only as far as this viewer's feedback level
		// allows, so the payload is not a fixed shape per caller.
		Response: scoring.SubmissionPublic{},
	},
	{
		Method: "GET", Path: "/api/submissions/:id/source", Tag: "submissions",
		Summary:     "Download the submitted source",
		ContentType: "application/octet-stream",
	},

	// --- problems -----------------------------------------------------------

	{
		Method: "GET", Path: "/api/problems/:id/revisions", Tag: "contests",
		Summary:     "List the revisions of a problem, newest first",
		Response:    problem.RevisionSummary{},
		EnvelopeKey: "revisions",
	},
	{
		Method: "POST", Path: "/api/problems/:id/revisions", Tag: "contests", Status: 201,
		Summary:   "Add a revision from a Hydro-style zip",
		FormFiles: []string{"zip"},
		Response:  revisionCreatedResponse{},
	},

	// --- judging ------------------------------------------------------------

	{
		Method: "POST", Path: "/api/contest-problems/:id/rejudge", Tag: "judging", Status: 202,
		Summary: "Start a rejudge against a newer revision",
		// Two-phase on purpose: this creates the batch, and switching to it is a
		// separate call, so the old scoreboard stays up until the new scores are
		// ready.
		Request:  rejudgeRequest{},
		Response: judging.RejudgeBatchView{},
	},
	{
		Method: "POST", Path: "/api/rejudge-batches/:id/switch", Tag: "judging",
		Summary:  "Publish a finished rejudge batch",
		Response: judging.AckView{},
	},
	{
		Method: "POST", Path: "/api/pools", Tag: "judging", Status: 201,
		Summary:  "Create a judge pool",
		Request:  createPoolRequest{},
		Response: judging.PoolView{},
	},
	{
		Method: "GET", Path: "/api/pools/:id/nodes", Tag: "judging",
		Summary:     "List the nodes in a pool",
		Response:    judging.NodeView{},
		EnvelopeKey: "nodes",
	},
	{
		Method: "POST", Path: "/api/pools/:id/nodes", Tag: "judging", Status: 201,
		Summary:  "Register a judge node in a pool",
		Request:  registerNodeRequest{},
		Response: judging.NodeView{},
	},
	{
		Method: "POST", Path: "/api/nodes/:id/rotate", Tag: "judging",
		Summary: "Rotate a node credential",
		// The only response that carries a plaintext credential, and only at
		// rotation time.
		Response: judging.CredentialView{},
	},
	{
		Method: "POST", Path: "/api/nodes/:id/revoke", Tag: "judging",
		Summary:  "Revoke a judge node",
		Response: judging.AckView{},
	},
	{
		Method: "POST", Path: "/api/tasks/:id/redispatch", Tag: "judging",
		Summary:  "Re-queue a task that failed to schedule",
		Response: judging.AckView{},
	},
	{
		Method: "GET", Path: "/api/judge/files/:token", Tag: "judging", Public: true,
		Summary: "Download a judging artifact with a signed token",
		// Authenticated by the token itself, which is why it needs no session.
		ContentType: "application/octet-stream",
		PathParams:  map[string]*openapi.Schema{"token": {Type: "string"}},
	},
}
