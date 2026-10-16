package problem

import "github.com/google/uuid"

type ImportResult struct {
	ProblemID  uuid.UUID `json:"problem_id"`
	RevisionID uuid.UUID `json:"revision_id"`
	RevisionN  int       `json:"revision_n"`
}

type RevisionRef struct {
	ID        uuid.UUID `json:"revision_id"`
	RevisionN int       `json:"revision_n"`
}
