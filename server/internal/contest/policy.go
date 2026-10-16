package contest

import (
	"time"

	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

type Snapshot struct {
	Rule                       domain.Rule
	EntryMode                  domain.EntryMode
	StartsAt                   time.Time
	EndsAt                     time.Time
	RegisterStartsAt           time.Time
	AllowExternal              bool
	RegistrationRequiresReview bool
	MaxTeamSize                int
	SourcePublicAfter          bool
	SourceVisibilityLocked     bool
	PublishedAt                *time.Time
}

func CanChangeRule(s Snapshot, now time.Time) error {
	if !now.Before(s.StartsAt) {
		return apperr.New(apperr.Conflict, "contest rule is frozen after start")
	}
	return nil
}

func CanChangeEntryMode() error {
	return apperr.New(apperr.Conflict, "entry mode cannot be changed after creation")
}

func RegistrationOpen(s Snapshot, now time.Time) bool {
	return !now.Before(s.RegisterStartsAt) && now.Before(s.EndsAt)
}

func CanRegister(s Snapshot, now time.Time, orgMember bool, emailVerified bool) error {
	if !emailVerified {
		return apperr.New(apperr.Unverified, "verify email before registering")
	}
	if !RegistrationOpen(s, now) {
		return apperr.New(apperr.Conflict, "registration is closed")
	}
	if !orgMember && !s.AllowExternal {
		return apperr.New(apperr.Forbidden, "this contest does not allow external contestants")
	}
	return nil
}

func CanSubmit(s Snapshot, now time.Time, accepted bool, emailVerified bool) error {
	if !emailVerified {
		return apperr.New(apperr.Unverified, "verify email before submitting")
	}
	if !accepted {
		return apperr.New(apperr.Forbidden, "entrant is not accepted")
	}
	if now.Before(s.StartsAt) || !now.Before(s.EndsAt) {
		return apperr.New(apperr.Conflict, "contest is not running")
	}
	return nil
}

func RosterFrozen(acceptedAt *time.Time) bool {
	return acceptedAt != nil
}

func CanMutateRoster(acceptedAt *time.Time, startsAt, now time.Time) error {
	if RosterFrozen(acceptedAt) && !now.Before(startsAt) {
		return apperr.New(apperr.Conflict, "roster is frozen after start")
	}
	return nil
}

func CanExpandSourcePublic(s Snapshot) error {
	if s.SourceVisibilityLocked && !s.SourcePublicAfter {
		return apperr.New(apperr.Conflict, "cannot expand source visibility after registration opened")
	}
	return nil
}

func TeamSizeOK(s Snapshot, n int) error {
	if n < 1 {
		return apperr.New(apperr.Invalid, "team must have at least one member")
	}
	if n > s.MaxTeamSize {
		return apperr.New(apperr.Invalid, "team exceeds max size")
	}
	return nil
}
