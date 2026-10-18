package judging

import (
	"time"

	"github.com/google/uuid"
)

// BlobRef is the storage handle callers need to open a blob. It is not a GORM model.
type BlobRef struct {
	StorageKey  string
	SHA256      string
	ContentType string
}

// PoolView is the admin JSON for a judging pool. It is not a GORM model.
type PoolView struct {
	ID             uuid.UUID  `json:"id"`
	Kind           string     `json:"kind"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Name           string     `json:"name"`
	Enabled        bool       `json:"enabled"`
}

// NodeView is the admin JSON for a registered judging node. It never carries a
// credential: plaintext credentials are returned only at register/rotate time.
type NodeView struct {
	ID                 uuid.UUID  `json:"id"`
	PoolID             uuid.UUID  `json:"pool_id"`
	Name               string     `json:"name"`
	Enabled            bool       `json:"enabled"`
	Connected          bool       `json:"connected"`
	Revoked            bool       `json:"revoked"`
	MaxSlots           int        `json:"max_slots"`
	LastHeartbeatAt    *time.Time `json:"last_heartbeat_at"`
	AdvertisedProfiles []string   `json:"advertised_profiles"`
}

// TaskView is the administrator view of a judge task with its latest attempt.
// A node that claimed a task and then vanished is resolved by an administrator,
// so the attempt's state and lease are part of the view.
type TaskView struct {
	ID           uuid.UUID    `json:"id"`
	SubmissionID uuid.UUID    `json:"submission_id"`
	State        string       `json:"state"`
	Purpose      string       `json:"purpose"`
	RevisionID   uuid.UUID    `json:"revision_id"`
	CreatedAt    time.Time    `json:"created_at"`
	Attempt      *AttemptView `json:"attempt"`
}

// AttemptView never carries the raw judge result, only dispatch bookkeeping.
type AttemptView struct {
	ID           uuid.UUID  `json:"id"`
	Generation   int        `json:"generation"`
	NodeID       uuid.UUID  `json:"node_id"`
	PoolID       uuid.UUID  `json:"pool_id"`
	State        string     `json:"state"`
	DispatchedAt time.Time  `json:"dispatched_at"`
	LeaseUntil   time.Time  `json:"lease_until"`
	FinishedAt   *time.Time `json:"finished_at"`
	SystemFault  bool       `json:"system_fault"`
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
