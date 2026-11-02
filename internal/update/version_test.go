package update_test

import (
	"testing"

	"github.com/jameskbb/casebook/internal/update"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.4.0", "v0.4.0", 0},
		{"v0.4.0", "v0.5.0", -1},
		{"v0.5.0", "v0.4.0", 1},
		{"v0.4.0", "v0.4.1", -1},
		{"v0.4.10", "v0.4.9", 1},
		{"v1.0.0", "v0.99.99", 1},
		{"v2.0.0", "v10.0.0", -1},
		{"v0.0.0", "v0.0.0", 0},
		{"v1.0.0-alpha", "v1.0.0", -1},
		{"v1.0.0", "v1.0.0-rc.1", 1},
		{"v1.0.0-alpha", "v1.0.0-alpha.1", -1},
		{"v1.0.0-alpha.1", "v1.0.0-alpha.beta", -1},
		{"v1.0.0-alpha.beta", "v1.0.0-beta", -1},
		{"v1.0.0-beta", "v1.0.0-beta.2", -1},
		{"v1.0.0-beta.2", "v1.0.0-beta.11", -1},
		{"v1.0.0-beta.11", "v1.0.0-rc.1", -1},
		{"v1.0.0-rc.1", "v1.0.0", -1},
		{"v1.0.0-rc.1", "v1.0.0-rc.1", 0},
		{"v1.0.0-1", "v1.0.0-a", -1},
		{"v1.0.0-a", "v1.0.0-1", 1},
		{"v1.0.0-0", "v1.0.0-0", 0},
		{"v1.0.0-x-y", "v1.0.0-x", 1},
		// A Go pseudo-version is a prerelease of the next patch.
		{"v0.4.1-0.20261013120000-abcdef123456", "v0.4.1", -1},
		{"v0.4.1-0.20261013120000-abcdef123456", "v0.4.0", 1},
	}
	for _, c := range cases {
		got, err := update.CompareVersions(c.a, c.b)
		if err != nil {
			t.Errorf("CompareVersions(%q, %q): %v", c.a, c.b, err)
			continue
		}
		if got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCompareVersionsOrdering(t *testing.T) {
	order := []string{"v1.0.0-alpha", "v1.0.0-alpha.1", "v1.0.0-beta", "v1.0.0-rc.1", "v1.0.0"}
	for i := range order {
		for j := range order {
			got, err := update.CompareVersions(order[i], order[j])
			if err != nil {
				t.Fatalf("CompareVersions(%q, %q): %v", order[i], order[j], err)
			}
			want := 0
			if i < j {
				want = -1
			} else if i > j {
				want = 1
			}
			if got != want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", order[i], order[j], got, want)
			}
		}
	}
}

func TestCompareVersionsRejects(t *testing.T) {
	bad := []string{
		"", "dev", "(devel)", "0.4.0", "V0.4.0", "v0.4", "v0.4.0.1", "v0.4.x", "va.b.c",
		"v01.0.0", "v0.04.0", "v0.4.00", "v0.4.0+build", "v0.4.0-rc.1+build", "v0.4.0-",
		"v0.4.0-rc..1", "v0.4.0-rc.01", "v0.4.0-rc_1", "v-1.0.0", "v+1.0.0", "v 1.0.0",
		"v1.0.0 ", "v.1.0", "v1..0",
	}
	for _, b := range bad {
		if _, err := update.CompareVersions(b, "v0.4.0"); err == nil {
			t.Errorf("CompareVersions(%q, v0.4.0) accepted it", b)
		}
		if _, err := update.CompareVersions("v0.4.0", b); err == nil {
			t.Errorf("CompareVersions(v0.4.0, %q) accepted it", b)
		}
	}
}

func TestAssetName(t *testing.T) {
	cases := []struct {
		flavor       update.Flavor
		goos, goarch string
		want         string
	}{
		{update.FlavorDesktop, "windows", "amd64", "Casebook-v0.5.0-windows-amd64-portable.zip"},
		{update.FlavorDesktop, "windows", "arm64", "Casebook-v0.5.0-windows-arm64-portable.zip"},
		{update.FlavorDesktop, "windows", "386", ""},
		{update.FlavorDesktop, "darwin", "arm64", "Casebook-v0.5.0-macos-universal.dmg"},
		{update.FlavorDesktop, "darwin", "amd64", "Casebook-v0.5.0-macos-universal.dmg"},
		{update.FlavorDesktop, "linux", "amd64", ""},
		{update.FlavorDesktop, "linux", "arm64", ""},
		{update.FlavorCLI, "linux", "amd64", "casebook-cli-v0.5.0-linux-amd64.tar.gz"},
		{update.FlavorCLI, "linux", "arm64", "casebook-cli-v0.5.0-linux-arm64.tar.gz"},
		{update.FlavorCLI, "darwin", "amd64", "casebook-cli-v0.5.0-darwin-amd64.tar.gz"},
		{update.FlavorCLI, "darwin", "arm64", "casebook-cli-v0.5.0-darwin-arm64.tar.gz"},
		{update.FlavorCLI, "windows", "amd64", "casebook-cli-v0.5.0-windows-amd64.zip"},
		{update.FlavorCLI, "windows", "arm64", "casebook-cli-v0.5.0-windows-arm64.zip"},
		{update.FlavorCLI, "linux", "386", ""},
		{update.FlavorCLI, "freebsd", "amd64", ""},
		{update.Flavor("other"), "linux", "amd64", ""},
		{update.Flavor(""), "windows", "amd64", ""},
	}
	for _, c := range cases {
		if got := update.AssetName("v0.5.0", c.flavor, c.goos, c.goarch); got != c.want {
			t.Errorf("AssetName(v0.5.0, %q, %s, %s) = %q, want %q", c.flavor, c.goos, c.goarch, got, c.want)
		}
	}
}
