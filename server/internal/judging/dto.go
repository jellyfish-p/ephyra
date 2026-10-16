package judging

import "github.com/google/uuid"

// PoolView is the admin JSON for a created pool. It is not a GORM model.
type PoolView struct {
	ID   uuid.UUID `json:"id"`
	Kind string    `json:"kind"`
	Name string    `json:"name"`
}

// NodeRegisteredView returns plaintext credential only at register time.
type NodeRegisteredView struct {
	ID         uuid.UUID `json:"id"`
	Credential string    `json:"credential"`
}

// CredentialView returns plaintext credential only at rotate time.
type CredentialView struct {
	Credential string `json:"credential"`
}

// RejudgeBatchView is the admin JSON for a created rejudge batch.
type RejudgeBatchView struct {
	ID    uuid.UUID `json:"batch_id"`
	State string    `json:"state"`
}

// AckView is a credential-free admin acknowledgement.
type AckView struct {
	OK bool `json:"ok"`
}

func Ok() AckView { return AckView{OK: true} }
