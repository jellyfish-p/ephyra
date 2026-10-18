package httpapi

import (
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/contest"
	"github.com/jellyfish-p/ephyra/server/internal/scoring"
)

// The types in this file are named response shapes for the OpenAPI generator.
// They mirror the JSON that the handlers previously produced with inline
// gin.H literals. json tags are byte-for-byte equivalent to the old keys.

type okResponse struct {
	OK bool `json:"ok"`
}

type accountResponse struct {
	AccountID     uuid.UUID `json:"account_id"`
	Email         string    `json:"email"`
	DisplayName   string    `json:"display_name"`
	EmailVerified bool      `json:"email_verified"`
	SiteAdmin     bool      `json:"site_admin"`
}

type orgSummaryResponse struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

type entrantCreatedResponse struct {
	EntrantID uuid.UUID `json:"entrant_id"`
	Status    string    `json:"status"`
}

type contestResponse struct {
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

type submitAcceptedResponse struct {
	SubmissionID uuid.UUID `json:"submission_id"`
	ReceiveSeq   int64     `json:"receive_seq"`
	ReceivedAt   time.Time `json:"received_at"`
	TaskID       uuid.UUID `json:"task_id"`
}

type importResultResponse struct {
	ProblemID  uuid.UUID `json:"problem_id"`
	RevisionID uuid.UUID `json:"revision_id"`
	RevisionN  int       `json:"revision_n"`
}

type revisionCreatedResponse struct {
	RevisionID uuid.UUID `json:"revision_id"`
	RevisionN  int       `json:"revision_n"`
}

type contestProblemCreatedResponse struct {
	ID         uuid.UUID `json:"id"`
	Letter     string    `json:"letter"`
	ProblemID  uuid.UUID `json:"problem_id"`
	RevisionID uuid.UUID `json:"revision_id"`
}

type membershipResponse struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	Member         bool      `json:"member"`
	Role           string    `json:"role"`
}

type myEntrantResponse struct {
	Entrant *contest.MyEntrantView `json:"entrant"`
}

type submissionListResponse struct {
	Submissions []scoring.SubmissionPublic `json:"submissions"`
}
