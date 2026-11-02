package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDataDirCasebookHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvHome, dir)
	got, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Fatalf("DataDir() = %q, want %q", got, dir)
	}
}

func TestDataDirRelativeHomeBecomesAbsolute(t *testing.T) {
	t.Setenv(EnvHome, "rel-casebook")
	got, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("DataDir() = %q, want an absolute path", got)
	}
	wd, _ := os.Getwd()
	if want := filepath.Join(wd, "rel-casebook"); got != want {
		t.Fatalf("DataDir() = %q, want %q", got, want)
	}
}

func TestDataDirLinuxDefaults(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("Linux/other branch only")
	}
	home := t.TempDir()
	xdg := t.TempDir()
	cases := []struct {
		name string
		xdg  string
		want string
	}{
		{"xdg set", xdg, filepath.Join(xdg, "casebook")},
		{"xdg unset", "", filepath.Join(home, ".local", "share", "casebook")},
		{"xdg relative ignored", "relative/xdg", filepath.Join(home, ".local", "share", "casebook")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvHome, "")
			t.Setenv("HOME", home)
			t.Setenv("XDG_DATA_HOME", tc.xdg)
			got, err := DataDir()
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("DataDir() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDBPathResolution(t *testing.T) {
	home := t.TempDir()
	dbDir := t.TempDir()
	cases := []struct {
		name    string
		envHome string
		envDB   string
		want    string
	}{
		{"home only", home, "", filepath.Join(home, DBFileName)},
		{"db wins over home", home, filepath.Join(dbDir, "x.db"), filepath.Join(dbDir, "x.db")},
		{"db alone", "", filepath.Join(dbDir, "y.db"), filepath.Join(dbDir, "y.db")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvHome, tc.envHome)
			t.Setenv(EnvDB, tc.envDB)
			got, err := DBPath()
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("DBPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDBPathRelativeBecomesAbsolute(t *testing.T) {
	t.Setenv(EnvDB, filepath.Join("sub", "rel.db"))
	got, err := DBPath()
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if want := filepath.Join(wd, "sub", "rel.db"); got != want {
		t.Fatalf("DBPath() = %q, want %q", got, want)
	}
}

func TestEnsureDirCreatesOwnerOnly(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "a", "b", "c")
	if err := EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(root, "a"), filepath.Join(root, "a", "b"), dir} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", p)
		}
		if runtime.GOOS != "windows" {
			if perm := info.Mode().Perm(); perm != 0o700 {
				t.Fatalf("%s mode = %o, want 700", p, perm)
			}
		}
	}
	// Idempotent.
	if err := EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureDirLeavesExistingDirAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions only")
	}
	root := t.TempDir()
	existing := filepath.Join(root, "shared")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDir(existing); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDir(filepath.Join(existing, "child")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(existing)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Fatalf("existing dir mode = %o, want 755 (must not be chmodded)", perm)
	}
	child, err := os.Stat(filepath.Join(existing, "child"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := child.Mode().Perm(); perm != 0o700 {
		t.Fatalf("created child mode = %o, want 700", perm)
	}
}

func TestEnsureDirRejectsFile(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "file")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDir(f); err == nil {
		t.Fatal("EnsureDir on a regular file: want error, got nil")
	}
}
