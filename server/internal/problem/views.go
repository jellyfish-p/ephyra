package problem

import (
	"time"

	"github.com/google/uuid"
)

type ImportResult struct {
	ProblemID  uuid.UUID `json:"problem_id"`
	RevisionID uuid.UUID `json:"revision_id"`
	RevisionN  int       `json:"revision_n"`
}

type RevisionRef struct {
	ID        uuid.UUID `json:"revision_id"`
	RevisionN int       `json:"revision_n"`
}

// RevisionSummary is a revision's metadata for an organization manager choosing
// which version a contest should use. It never carries judge data.
type RevisionSummary struct {
	RevisionID uuid.UUID `json:"revision_id"`
	RevisionN  int       `json:"revision_n"`
	CreatedAt  time.Time `json:"created_at"`
}
