package access

import "github.com/google/uuid"

type Principal struct {
	AccountID     uuid.UUID
	Email         string
	DisplayName   string
	EmailVerified bool
	SiteAdmin     bool
}

func (p Principal) Zero() bool { return p.AccountID == uuid.Nil }

type Audience string

const (
	AudienceSelf            Audience = "self"
	AudienceTeammate        Audience = "teammate"
	AudienceContestAdmin    Audience = "contest_admin"
	AudienceOrgAdmin        Audience = "org_admin"
	AudienceOtherContestant Audience = "other_contestant"
	AudienceLoggedIn        Audience = "logged_in"
	AudienceAnonymous       Audience = "anonymous"
)
