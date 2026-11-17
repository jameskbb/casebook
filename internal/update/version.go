package update

import (
	"errors"
	"fmt"
	"strings"
)

// version is a parsed vMAJOR.MINOR.PATCH[-prerelease]. The numbers stay as
// their decimal strings so an absurdly long component cannot overflow; they
// compare by length first, which is numeric order once leading zeros are
// ruled out.
type version struct {
	core [3]string
	pre  []string
}

// parseVersion accepts exactly the semantic versioning 2.0.0 grammar behind a
// leading v, without build metadata. A Go pseudo-version
// (v0.4.1-0.20261013120000-abcdef123456) is an ordinary prerelease here.
func parseVersion(s string) (version, error) {
	var v version
	rest, ok := strings.CutPrefix(s, "v")
	if !ok {
		return v, fmt.Errorf("%q is not a release version (want vMAJOR.MINOR.PATCH)", s)
	}
	if strings.Contains(rest, "+") {
		return v, fmt.Errorf("%q has build metadata, which a release version does not carry", s)
	}
	core, pre, hasPre := strings.Cut(rest, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return v, fmt.Errorf("%q is not a release version (want vMAJOR.MINOR.PATCH)", s)
	}
	for i, p := range parts {
		if !isNumeric(p) {
			return v, fmt.Errorf("%q is not a release version: %q is not a number without leading zeros", s, p)
		}
		v.core[i] = p
	}
	if !hasPre {
		return v, nil
	}
	for _, id := range strings.Split(pre, ".") {
		if id == "" || !isIdentifier(id) {
			return v, fmt.Errorf("%q has an invalid prerelease identifier %q", s, id)
		}
		if isDigits(id) && !isNumeric(id) {
			return v, fmt.Errorf("%q has a prerelease number with a leading zero: %q", s, id)
		}
		v.pre = append(v.pre, id)
	}
	return v, nil
}

// isDigits reports whether s is one or more ASCII digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isNumeric reports whether s is a semver numeric identifier: digits with no
// leading zero unless it is "0".
func isNumeric(s string) bool {
	return isDigits(s) && (s == "0" || s[0] != '0')
}

// isIdentifier reports whether s uses only [0-9A-Za-z-].
func isIdentifier(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-') {
			return false
		}
	}
	return true
}

// compareNumbers orders two numeric identifiers without converting them.
func compareNumbers(a, b string) int {
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

// compareIdentifiers orders two prerelease identifiers: numbers numerically,
// numbers before alphanumerics, alphanumerics in ASCII order.
func compareIdentifiers(a, b string) int {
	an, bn := isDigits(a), isDigits(b)
	switch {
	case an && bn:
		return compareNumbers(a, b)
	case an:
		return -1
	case bn:
		return 1
	}
	return strings.Compare(a, b)
}

func (v version) compare(w version) int {
	for i := range v.core {
		if c := compareNumbers(v.core[i], w.core[i]); c != 0 {
			return c
		}
	}
	// A release sorts after every prerelease of the same version.
	switch {
	case len(v.pre) == 0 && len(w.pre) == 0:
		return 0
	case len(v.pre) == 0:
		return 1
	case len(w.pre) == 0:
		return -1
	}
	for i := 0; i < len(v.pre) && i < len(w.pre); i++ {
		if c := compareIdentifiers(v.pre[i], w.pre[i]); c != 0 {
			return c
		}
	}
	// The shorter identifier list sorts first when all shared ones match.
	switch {
	case len(v.pre) < len(w.pre):
		return -1
	case len(v.pre) > len(w.pre):
		return 1
	}
	return 0
}

func compareVersions(a, b string) (int, error) {
	va, errA := parseVersion(a)
	vb, errB := parseVersion(b)
	if err := errors.Join(errA, errB); err != nil {
		return 0, err
	}
	return va.compare(vb), nil
}

// assetName implements AssetName. The names mirror RELEASE_ASSETS in
// .github/workflows/release.yml; the desktop flavor points at the portable
// zip on Windows because that is the primary Windows asset.
func assetName(v string, flavor Flavor, goos, goarch string) string {
	// The macOS disk image is universal, so it fits any architecture; every
	// other asset exists for amd64 and arm64 only.
	if flavor == FlavorDesktop && goos == "darwin" {
		return "Casebook-" + v + "-macos-universal.dmg"
	}
	if goarch != "amd64" && goarch != "arm64" {
		return ""
	}
	switch flavor {
	case FlavorDesktop:
		switch goos {
		case "windows":
			return "Casebook-" + v + "-windows-" + goarch + "-portable.zip"
		}
	case FlavorCLI:
		switch goos {
		case "linux", "darwin":
			return "casebook-cli-" + v + "-" + goos + "-" + goarch + ".tar.gz"
		case "windows":
			return "casebook-cli-" + v + "-windows-" + goarch + ".zip"
		}
	}
	return ""
}
