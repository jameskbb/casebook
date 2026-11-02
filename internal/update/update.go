// Package update asks GitHub for the latest Casebook release and compares it
// with the running build. It is the one place in Casebook that opens a
// network connection, and it only runs when a person chooses "Check for
// updates" in the desktop app or runs `casebook update` (ADR 0022). Nothing
// runs it on a timer or at startup, it never downloads or installs anything,
// and the request carries no version, identifier or data: one GET to GitHub's
// public releases API with a fixed User-Agent.
//
// Only internal/cli and desktop may import this package (TestPackageBoundaries
// in internal/app enforces it): the engine, the MCP server, the hooks and
// doctor stay network-free, so `casebook mcp` serves AI clients with no
// network code path at all.
package update

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Repo is the GitHub repository whose releases are checked.
const Repo = "jameskbb/casebook"

// ReleasesURL is the page listing every release, for a person to open.
const ReleasesURL = "https://github.com/" + Repo + "/releases"

// DefaultBaseURL is GitHub's REST API; tests point BaseURL at a local server.
const DefaultBaseURL = "https://api.github.com"

// UserAgent is sent with every request. GitHub rejects requests without one.
// It names the program and nothing else: no version, no identifier.
const UserAgent = "Casebook-update-check (+https://github.com/" + Repo + ")"

// DefaultTimeout bounds one check when Options.Client is nil.
const DefaultTimeout = 10 * time.Second

// Flavor selects which download a check points at.
type Flavor string

const (
	// FlavorDesktop is the desktop app: the Windows portable zip for the
	// running architecture, or the macOS universal disk image.
	FlavorDesktop Flavor = "desktop"
	// FlavorCLI is the command line tool alone: the casebook-cli archive
	// for the running operating system and architecture.
	FlavorCLI Flavor = "cli"
)

// State says how the running build compares with the latest release.
type State string

const (
	// StateUpdateAvailable: the latest release is newer than Current.
	StateUpdateAvailable State = "update_available"
	// StateUpToDate: Current is the latest release.
	StateUpToDate State = "up_to_date"
	// StateAhead: Current is newer than the latest release (a build from
	// source after the last tag, or a prerelease of the next version).
	StateAhead State = "ahead"
	// StateUnknown: Current is not a release version (dev, (devel), or an
	// unparseable value), so no comparison is possible; Latest is still
	// reported.
	StateUnknown State = "unknown"
)

// Options configures one check. The zero value is not usable: Current and
// Flavor must be set. Every other field has a default.
type Options struct {
	// Current is the running build's version (internal/version.Version):
	// v0.4.0, or dev for an unstamped build.
	Current string
	// Flavor picks the download named in the result.
	Flavor Flavor
	// GOOS and GOARCH default to the running program's values.
	GOOS, GOARCH string
	// BaseURL defaults to DefaultBaseURL.
	BaseURL string
	// Client defaults to an http.Client with DefaultTimeout.
	Client *http.Client
	// Now defaults to time.Now; it stamps Result.CheckedAt.
	Now func() time.Time
}

// Asset is one downloadable file of a release.
type Asset struct {
	// Name is the file name, for example Casebook-v0.5.0-windows-amd64-portable.zip.
	Name string `json:"name"`
	// URL is the browser download address.
	URL string `json:"url"`
	// SizeBytes is the file size GitHub reports.
	SizeBytes int64 `json:"size_bytes"`
}

// Result is what a successful check learned. Its JSON is the `casebook
// update --json` output and the desktop binding's return value.
type Result struct {
	// Current is Options.Current as given.
	Current string `json:"current"`
	// Latest is the latest release's tag, for example v0.5.0. GitHub's
	// "latest" release is the newest published release that is not a
	// prerelease or a draft.
	Latest string `json:"latest"`
	// State compares Current with Latest.
	State State `json:"state"`
	// ReleaseURL is the latest release's page.
	ReleaseURL string `json:"release_url"`
	// PublishedAt is when the latest release was published (UTC).
	PublishedAt time.Time `json:"published_at"`
	// Asset is the download for this flavor, operating system and
	// architecture, or nil when the release has no such file (for example
	// the desktop flavor on Linux, which has no desktop package).
	Asset *Asset `json:"asset,omitempty"`
	// CheckedAt is when the check ran (UTC).
	CheckedAt time.Time `json:"checked_at"`
}

// Sentinel errors. Check wraps each with a message a person can read, so
// err.Error() is safe to print or show as is.
var (
	// ErrUnavailable: GitHub could not be reached, or answered with a
	// server error or something that is not a release.
	ErrUnavailable = errors.New("could not reach GitHub")
	// ErrNoRelease: GitHub answered 404. The repository is private or does
	// not exist, or it has no published release.
	ErrNoRelease = errors.New("GitHub lists no release for " + Repo)
	// ErrRateLimited: GitHub answered 403 or 429 with a rate limit message.
	ErrRateLimited = errors.New("GitHub is rate limiting update checks from this address")
)

// Check asks GitHub for the latest release of Repo and compares it with
// o.Current. It makes exactly one GET request, to
// <BaseURL>/repos/jameskbb/casebook/releases/latest, reads at most 1 MiB of
// the response, and never follows anything else. The context cancels or
// bounds the request in addition to the client's timeout.
func Check(ctx context.Context, o Options) (Result, error) {
	return check(ctx, o)
}

// AssetName is the file name a release publishes for flavor on goos/goarch,
// as the release workflow names them (.github/workflows/release.yml), or ""
// when no such file exists (the desktop flavor on Linux). version is the
// release tag, for example v0.5.0.
func AssetName(version string, flavor Flavor, goos, goarch string) string {
	return assetName(version, flavor, goos, goarch)
}

// CompareVersions orders two release versions of the form
// vMAJOR.MINOR.PATCH with an optional -prerelease suffix (semantic
// versioning: a prerelease sorts before its release, and prerelease
// identifiers compare dot by dot, numerically when both are numbers). It
// returns -1, 0 or 1, or an error when either is not a release version
// (dev, (devel), an empty string, or build metadata after a plus sign).
func CompareVersions(a, b string) (int, error) {
	return compareVersions(a, b)
}
