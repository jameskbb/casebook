// Package version holds the build-stamped version shared by both Casebook
// executables: the Casebook desktop app and the casebook CLI (which also
// serves MCP). Both are stamped at build time with -ldflags -X by
// scripts/version.sh and the release workflow, so they always report the
// same value. An unstamped build reports the main module version that Go
// records in its build information when there is one (go install
// github.com/jameskbb/casebook/cmd/casebook@v1.2.3 reports v1.2.3), and dev
// otherwise (go run, go test, or a build with -buildvcs=false).
//
// The stamping flags are exactly:
//
//	-X github.com/jameskbb/casebook/internal/version.Version=<version>
//	-X github.com/jameskbb/casebook/internal/version.Commit=<short sha>
//	-X github.com/jameskbb/casebook/internal/version.Marker=casebook-version=<version>
//
// This package imports only the standard library so any package in the
// module may use it.
package version

import "runtime/debug"

// MarkerPrefix starts the Marker value of a stamped build.
const MarkerPrefix = "casebook-version="

// Version is the release version, for example v1.2.3. It is dev unless the
// build stamps it or Go's build information names a module version (see
// init).
var Version = "dev"

// Commit is the short git commit the binary was built from. It is empty
// unless the build stamps it.
var Commit = ""

// Marker is MarkerPrefix followed by the version in a stamped build, for
// example "casebook-version=v1.2.3", and empty otherwise. Release checks
// search each executable for this exact text. Only the -X flag can put it
// there: Go's automatic build information records module and VCS versions
// (which can equal the release version) but never this prefix.
var Marker = ""

// stamped is computed during package initialization, which every executable
// that imports this package runs. Reading Marker here keeps the stamped text
// linked into the binary even when nothing calls Stamped.
var stamped = Marker != "" && Marker == MarkerPrefix+Version

// Stamped reports whether the build stamped Marker and Version with the same
// version, as scripts/version.sh ldflags does.
func Stamped() bool {
	return stamped
}

// String returns the version for --version and About output: Version alone
// when Commit is empty, otherwise Version followed by the commit in
// parentheses, for example "v1.2.3 (abc1234)".
func String() string {
	if Commit == "" {
		return Version
	}
	return Version + " (" + Commit + ")"
}

// init fills Version from Go's build information when the build did not
// stamp it. It runs after the package variables above are initialized, so
// stamped (and with it Stamped and the release marker checks) still reflects
// only the -X flags, and before any importing package initializes, so
// copies such as internal/cli.Version see the final value.
func init() {
	if Marker != "" || Version != "dev" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if v := moduleVersion(info); v != "" {
		Version = v
	}
}

// moduleVersion returns the main module version from Go's build information,
// or "" when it carries none. go install module@version records the module
// version (v1.2.3, or a pseudo-version for a commit); a go build inside a
// repository checkout may record a pseudo-version, with a +dirty suffix for
// uncommitted changes; go run, go test and -buildvcs=false builds record
// "(devel)" or nothing.
func moduleVersion(info *debug.BuildInfo) string {
	if info == nil {
		return ""
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return ""
	}
	return v
}
