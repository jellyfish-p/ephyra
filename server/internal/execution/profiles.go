package execution

import "github.com/jellyfish-p/ephyra/server/internal/domain"

// Profile identifies a verified compiler/library/sandbox combination.
// IDs are not "has g++"; a node must advertise the exact profile id.
type Profile struct {
	ID         string
	LanguageID domain.LanguageID
	CompilerID string
	Triple     string
}

const (
	CPP23 = "cpp23.gcc.linux-amd64.v0"
	C17   = "c17.gcc.linux-amd64.v0"
	C23   = "c23.gcc.linux-amd64.v0"
)

// v0 profiles are placeholders until a specific toolchain release is locked.
var catalog = []Profile{
	{ID: CPP23, LanguageID: domain.LangCPP23, CompilerID: "gcc-unverified", Triple: "x86_64-linux-gnu"},
	{ID: C17, LanguageID: domain.LangC17, CompilerID: "gcc-unverified", Triple: "x86_64-linux-gnu"},
	{ID: C23, LanguageID: domain.LangC23, CompilerID: "gcc-unverified", Triple: "x86_64-linux-gnu"},
}

func All() []Profile { return append([]Profile(nil), catalog...) }

func Lookup(id string) (Profile, bool) {
	for _, p := range catalog {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

func DefaultFor(lang domain.LanguageID) (Profile, bool) {
	for _, p := range catalog {
		if p.LanguageID == lang {
			return p, true
		}
	}
	return Profile{}, false
}
