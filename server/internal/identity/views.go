package identity

import (
	"time"

	"github.com/google/uuid"
)

type AccountView struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	DisplayName   string    `json:"display_name"`
	EmailVerified bool      `json:"email_verified"`
	SiteAdmin     bool      `json:"site_admin"`
}

type OrganizationView struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

type Member struct {
	AccountID   uuid.UUID `json:"account_id"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
}

type JoinRequest struct {
	ID          uuid.UUID `json:"id"`
	AccountID   uuid.UUID `json:"account_id"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type Membership struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	Member         bool      `json:"member"`
	Role           string    `json:"role,omitempty"`
}
