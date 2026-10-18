package contest

import (
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
)

type View struct {
	ID                     uuid.UUID  `json:"id"`
	OrganizationID         uuid.UUID  `json:"organization_id"`
	Slug                   string     `json:"slug"`
	Title                  string     `json:"title"`
	Rule                   string     `json:"rule"`
	EntryMode              string     `json:"entry_mode"`
	StartsAt               time.Time  `json:"starts_at"`
	EndsAt                 time.Time  `json:"ends_at"`
	RegisterStartsAt       time.Time  `json:"register_starts_at"`
	AllowExternal          bool       `json:"allow_external"`
	SourcePublicAfter      bool       `json:"source_public_after"`
	SourceVisibilityLocked bool       `json:"source_visibility_locked"`
	PublishedAt            *time.Time `json:"published_at"`
}

type ProblemView struct {
	ID                uuid.UUID  `json:"id"`
	Letter            string     `json:"letter"`
	Title             string     `json:"title,omitempty"`
	ProblemID         *uuid.UUID `json:"problem_id,omitempty"`
	ScoringRevisionID *uuid.UUID `json:"scoring_revision_id,omitempty"`
	TargetRevisionID  *uuid.UUID `json:"target_revision_id,omitempty"`
}

type BankProblem struct {
	ID    uuid.UUID `json:"id"`
	Slug  string    `json:"slug"`
	Title string    `json:"title"`
}

type AddedProblem struct {
	ID         uuid.UUID `json:"id"`
	Letter     string    `json:"letter"`
	ProblemID  uuid.UUID `json:"problem_id"`
	RevisionID uuid.UUID `json:"revision_id"`
}

type EntrantView struct {
	ID     uuid.UUID `json:"id"`
	Status string    `json:"status"`
}

// ProblemDetail contains only statement and public configuration, never judge data.
type ProblemDetail struct {
	Letter            string           `json:"letter"`
	Title             string           `json:"title"`
	ProblemID         uuid.UUID        `json:"problem_id"`
	ScoringRevisionID uuid.UUID        `json:"scoring_revision_id"`
	TargetRevisionID  uuid.UUID        `json:"target_revision_id"`
	Kind              string           `json:"kind"`
	CheckerKind       string           `json:"checker_kind"`
	TimeLimitMS       int64            `json:"time_limit_ms"`
	MemoryLimitBytes  int64            `json:"memory_limit_bytes"`
	Subtasks          []ProblemSubtask `json:"subtasks"`
	StatementMarkdown *string          `json:"statement_markdown"`
}

type ProblemSubtask struct {
	ID    string      `json:"id"`
	Score exact.Score `json:"score"`
}

type MyEntrantView struct {
	ID             uuid.UUID           `json:"id"`
	ContestID      uuid.UUID           `json:"contest_id"`
	Kind           string              `json:"kind"`
	DisplayName    string              `json:"display_name"`
	Status         string              `json:"status"`
	RegisteredAt   time.Time           `json:"registered_at"`
	AcceptedAt     *time.Time          `json:"accepted_at"`
	RosterFrozenAt *time.Time          `json:"roster_frozen_at"`
	Members        []EntrantMemberView `json:"members"`
}

type EntrantMemberView struct {
	AccountID   uuid.UUID `json:"account_id"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	Confirmed   bool      `json:"confirmed"`
}

func viewOf(c persist.Contest) View {
	return View{
		ID: c.ID, OrganizationID: c.OrganizationID, Slug: c.Slug, Title: c.Title,
		Rule: c.Rule, EntryMode: c.EntryMode, StartsAt: c.StartsAt, EndsAt: c.EndsAt,
		RegisterStartsAt: c.RegisterStartsAt, AllowExternal: c.AllowExternal,
		SourcePublicAfter: c.SourcePublicAfter, SourceVisibilityLocked: c.SourceVisibilityLocked,
		PublishedAt: c.PublishedAt,
	}
}
