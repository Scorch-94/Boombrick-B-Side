// Package version reports the Boombrick build version.
package version

// Version is overridden at build time:
//
//	-ldflags "-X github.com/ujjwal313/boombrick/internal/version.Version=1.0.0"
var Version = "dev"

// Name is this fork's name: what NextUI lists in Tools, its pak and data
// folders, and its log. It differs from upstream's "Boombrick" so the two
// install side by side, each with its own library and settings.
const Name = "Boombrick B-Side"

// Slug is Name as release files are named: "Boombrick-B-Side".
const Slug = "Boombrick-B-Side"

// String returns the name and version shown in Settings and the log.
func String() string {
	return Name + " " + Version
}

// Repo is the GitHub repository Settings checks for Boombrick updates, as
// owner/name. Set at build time to the fork that built it, so an update
// never swaps this build for upstream's:
//
//	-ldflags "-X github.com/ujjwal313/boombrick/internal/version.Repo=you/boombrick"
//
// Builds made elsewhere check this fork's.
var Repo = "Scorch-94/Boombrick-B-Side"
