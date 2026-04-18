package cli

import (
	"github.com/lox/notion-cli/internal/profile"
)

// active holds the profile resolved by main.go for the current invocation.
// Helpers in this package that create token stores or load config read
// from it so commands don't need to thread the profile through every call.
var active = profile.Profile{Name: profile.DefaultName, Source: profile.SourceDefault}

// SetActiveProfile records the profile selected for this invocation.
func SetActiveProfile(p profile.Profile) {
	active = p
}

// ActiveProfile returns the profile selected for this invocation, or the
// implicit default profile if none was set.
func ActiveProfile() profile.Profile {
	return active
}
