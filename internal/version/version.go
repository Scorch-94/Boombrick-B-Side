// Package version reports the Boombrick build version.
package version

// Version is overridden at build time:
//
//	-ldflags "-X github.com/ujjwal313/boombrick/internal/version.Version=1.0.0"
var Version = "dev"

// String returns the name and version shown in Settings and the log.
func String() string {
	return "Boombrick " + Version
}

// Repo is the GitHub repository Settings checks for Boombrick updates, as
// owner/name. Set at build time to the fork that built it, so an update
// never swaps this build for upstream's:
//
//	-ldflags "-X github.com/ujjwal313/boombrick/internal/version.Repo=you/boombrick"
var Repo = "ujjwal313/boombrick"
