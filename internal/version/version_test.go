package version

import (
	"os"
	"runtime/debug"
	"testing"
)

func TestModuleVersion(t *testing.T) {
	cases := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{"nil", nil, ""},
		{"empty", &debug.BuildInfo{}, ""},
		{"devel", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, ""},
		{"release", &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, "v1.2.3"},
		{"prerelease", &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3-rc.1"}}, "v1.2.3-rc.1"},
		{"pseudo", &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20261006120000-ecd10f8abcde+dirty"}}, "v0.0.0-20261006120000-ecd10f8abcde+dirty"},
	}
	for _, c := range cases {
		if got := moduleVersion(c.info); got != c.want {
			t.Errorf("%s: moduleVersion = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestDefaults checks the values an unstamped build reports. A stamped test
// run (CASEBOOK_EXPECT_VERSION set) skips it because the stamp replaces them.
func TestDefaults(t *testing.T) {
	if os.Getenv("CASEBOOK_EXPECT_VERSION") != "" {
		t.Skip("stamped run: TestStamped checks the values")
	}
	if Version != "dev" {
		t.Errorf("Version = %q, want %q", Version, "dev")
	}
	if Commit != "" {
		t.Errorf("Commit = %q, want empty", Commit)
	}
	if Marker != "" {
		t.Errorf("Marker = %q, want empty", Marker)
	}
	if Stamped() {
		t.Error("Stamped() = true for an unstamped build")
	}
}

func TestString(t *testing.T) {
	oldVersion, oldCommit := Version, Commit
	t.Cleanup(func() { Version, Commit = oldVersion, oldCommit })

	Version, Commit = "v1.2.3", ""
	if got := String(); got != "v1.2.3" {
		t.Errorf("String() without commit = %q, want %q", got, "v1.2.3")
	}
	Version, Commit = "v1.2.3", "abc1234"
	if got := String(); got != "v1.2.3 (abc1234)" {
		t.Errorf("String() with commit = %q, want %q", got, "v1.2.3 (abc1234)")
	}
}

// TestStamped proves the -ldflags string from scripts/version.sh reaches the
// real symbols. The Go linker silently ignores a -X flag whose symbol path
// is wrong, so only a test run with the flags applied can catch that. It
// runs only when CASEBOOK_EXPECT_VERSION is set (CI and scripts/test-scripts.sh).
func TestStamped(t *testing.T) {
	want := os.Getenv("CASEBOOK_EXPECT_VERSION")
	if want == "" {
		t.Skip("CASEBOOK_EXPECT_VERSION not set")
	}
	if Version != want {
		t.Errorf("Version = %q, want %q (check the -X path in scripts/lib.sh)", Version, want)
	}
	if wantCommit := os.Getenv("CASEBOOK_EXPECT_COMMIT"); Commit != wantCommit {
		t.Errorf("Commit = %q, want %q", Commit, wantCommit)
	}
	if wantMarker := MarkerPrefix + want; Marker != wantMarker {
		t.Errorf("Marker = %q, want %q (check the -X path in scripts/lib.sh)", Marker, wantMarker)
	}
	if !Stamped() {
		t.Error("Stamped() = false for a stamped build")
	}
}
