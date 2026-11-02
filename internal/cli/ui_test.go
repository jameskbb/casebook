package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fakeInfo is a FileInfo for the fake file systems below.
type fakeInfo struct {
	name string
	dir  bool
}

func (f *fakeInfo) Name() string       { return f.name }
func (f *fakeInfo) Size() int64        { return 0 }
func (f *fakeInfo) ModTime() time.Time { return time.Time{} }
func (f *fakeInfo) IsDir() bool        { return f.dir }
func (f *fakeInfo) Sys() any           { return nil }
func (f *fakeInfo) Mode() fs.FileMode {
	if f.dir {
		return fs.ModeDir | 0o755
	}
	return 0o755
}

// fakeFS maps forward-slash paths to entries. A value that is the same
// pointer as another entry's models one file reached by two names (a
// case-insensitive file system resolving Casebook.exe to casebook.exe).
type fakeFS map[string]*fakeInfo

func (m fakeFS) stat(p string) (os.FileInfo, error) {
	if fi, ok := m[filepath.ToSlash(p)]; ok {
		return fi, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrNotExist}
}

func file(name string) *fakeInfo { return &fakeInfo{name: name} }
func dir(name string) *fakeInfo  { return &fakeInfo{name: name, dir: true} }

func TestFindDesktopApp(t *testing.T) {
	winCLI := file("casebook.exe")
	macCLI := file("casebook")
	cases := []struct {
		name       string
		goos, exe  string
		fs         fakeFS
		override   string
		home       string
		wantPath   string
		wantOpen   bool
		wantFound  bool
		wantLooked []string
	}{
		{
			name: "windows sibling",
			goos: "windows", exe: "C:/Program Files/Casebook/casebook.exe",
			fs: fakeFS{
				"C:/Program Files/Casebook/casebook.exe": winCLI,
				"C:/Program Files/Casebook/Casebook.exe": file("Casebook.exe"),
			},
			wantPath: "C:/Program Files/Casebook/Casebook.exe", wantFound: true,
			wantLooked: []string{"C:/Program Files/Casebook/Casebook.exe"},
		},
		{
			name: "windows sibling is the CLI itself (case-insensitive), parent has the app",
			goos: "windows", exe: "C:/Program Files/Casebook/bin/casebook.exe",
			fs: fakeFS{
				"C:/Program Files/Casebook/bin/casebook.exe": winCLI,
				"C:/Program Files/Casebook/bin/Casebook.exe": winCLI,
				"C:/Program Files/Casebook/Casebook.exe":     file("Casebook.exe"),
			},
			wantPath: "C:/Program Files/Casebook/Casebook.exe", wantFound: true,
			wantLooked: []string{"C:/Program Files/Casebook/bin/Casebook.exe", "C:/Program Files/Casebook/Casebook.exe"},
		},
		{
			name: "windows only the CLI",
			goos: "windows", exe: "C:/tools/casebook.exe",
			fs: fakeFS{
				"C:/tools/casebook.exe": winCLI,
				"C:/tools/Casebook.exe": winCLI,
			},
			wantLooked: []string{"C:/tools/Casebook.exe", "C:/Casebook.exe"},
		},
		{
			name: "windows sibling directory does not count",
			goos: "windows", exe: "C:/tools/casebook.exe",
			fs: fakeFS{
				"C:/tools/casebook.exe": winCLI,
				"C:/tools/Casebook.exe": dir("Casebook.exe"),
			},
			wantLooked: []string{"C:/tools/Casebook.exe", "C:/Casebook.exe"},
		},
		{
			name: "darwin packaged layout: CLI in Contents/MacOS/bin (ADR-0012)",
			goos: "darwin", exe: "/Applications/Casebook.app/Contents/MacOS/bin/casebook",
			fs: fakeFS{
				"/Applications/Casebook.app/Contents/MacOS/bin/casebook": macCLI,
				"/Applications/Casebook.app/Contents/MacOS/Casebook":     file("Casebook"),
				"/Applications/Casebook.app":                             dir("Casebook.app"),
			},
			wantPath: "/Applications/Casebook.app", wantOpen: true, wantFound: true,
			wantLooked: []string{"/Applications/Casebook.app/Contents/MacOS/Casebook"},
		},
		{
			name: "darwin enclosing bundle without the Casebook executable is not opened",
			goos: "darwin", exe: "/Users/me/Apps/Other.app/Contents/Resources/bin/casebook",
			fs: fakeFS{
				"/Users/me/Apps/Other.app/Contents/Resources/bin/casebook": macCLI,
				"/Users/me/Apps/Other.app":                                 dir("Other.app"),
			},
			wantLooked: []string{
				"/Users/me/Apps/Other.app/Contents/MacOS/Casebook",
				"/Users/me/Apps/Other.app/Contents/Resources/bin/Casebook",
				"/Applications/Casebook.app",
			},
		},
		{
			name: "darwin CLI next to its own case-colliding twin is skipped",
			goos: "darwin", exe: "/Applications/Casebook.app/Contents/MacOS/casebook",
			fs: fakeFS{
				"/Applications/Casebook.app/Contents/MacOS/casebook": macCLI,
				"/Applications/Casebook.app/Contents/MacOS/Casebook": macCLI,
				"/Applications/Casebook.app":                         dir("Casebook.app"),
			},
			wantPath: "/Applications/Casebook.app", wantOpen: true, wantFound: true,
			wantLooked: []string{
				"/Applications/Casebook.app/Contents/MacOS/Casebook",
				"/Applications/Casebook.app/Contents/MacOS/Casebook",
				"/Applications/Casebook.app",
			},
		},
		{
			name: "darwin /usr/local/bin with /Applications bundle",
			goos: "darwin", exe: "/usr/local/bin/casebook", home: "/Users/me",
			fs: fakeFS{
				"/usr/local/bin/casebook":    macCLI,
				"/usr/local/bin/Casebook":    macCLI,
				"/Applications/Casebook.app": dir("Casebook.app"),
			},
			wantPath: "/Applications/Casebook.app", wantOpen: true, wantFound: true,
			wantLooked: []string{"/usr/local/bin/Casebook", "/Applications/Casebook.app"},
		},
		{
			name: "darwin sibling executable",
			goos: "darwin", exe: "/opt/casebook/casebook", home: "/Users/me",
			fs: fakeFS{
				"/opt/casebook/casebook": macCLI,
				"/opt/casebook/Casebook": file("Casebook"),
			},
			wantPath: "/opt/casebook/Casebook", wantFound: true,
			wantLooked: []string{"/opt/casebook/Casebook"},
		},
		{
			name: "darwin home Applications",
			goos: "darwin", exe: "/usr/local/bin/casebook", home: "/Users/me",
			fs: fakeFS{
				"/usr/local/bin/casebook":             macCLI,
				"/Users/me/Applications/Casebook.app": dir("Casebook.app"),
			},
			wantPath: "/Users/me/Applications/Casebook.app", wantOpen: true, wantFound: true,
			wantLooked: []string{"/usr/local/bin/Casebook", "/Applications/Casebook.app", "/Users/me/Applications/Casebook.app"},
		},
		{
			name: "darwin nothing, no home",
			goos: "darwin", exe: "/usr/local/bin/casebook",
			fs:         fakeFS{"/usr/local/bin/casebook": macCLI},
			wantLooked: []string{"/usr/local/bin/Casebook", "/Applications/Casebook.app"},
		},
		{
			name: "darwin bundle path that is a file does not count",
			goos: "darwin", exe: "/usr/local/bin/casebook", home: "/Users/me",
			fs: fakeFS{
				"/usr/local/bin/casebook":    macCLI,
				"/Applications/Casebook.app": file("Casebook.app"),
			},
			wantLooked: []string{"/usr/local/bin/Casebook", "/Applications/Casebook.app", "/Users/me/Applications/Casebook.app"},
		},
		{
			name: "linux sibling",
			goos: "linux", exe: "/opt/casebook/bin/casebook",
			fs: fakeFS{
				"/opt/casebook/bin/casebook": file("casebook"),
				"/opt/casebook/bin/Casebook": file("Casebook"),
			},
			wantPath: "/opt/casebook/bin/Casebook", wantFound: true,
			wantLooked: []string{"/opt/casebook/bin/Casebook"},
		},
		{
			name: "linux parent directory",
			goos: "linux", exe: "/opt/casebook/bin/casebook",
			fs: fakeFS{
				"/opt/casebook/bin/casebook": file("casebook"),
				"/opt/casebook/Casebook":     file("Casebook"),
			},
			wantPath: "/opt/casebook/Casebook", wantFound: true,
			wantLooked: []string{"/opt/casebook/bin/Casebook", "/opt/casebook/Casebook"},
		},
		{
			name: "linux nothing found",
			goos: "linux", exe: "/home/me/bin/casebook",
			fs:         fakeFS{"/home/me/bin/casebook": file("casebook")},
			wantLooked: []string{"/home/me/bin/Casebook", "/home/me/Casebook"},
		},
		{
			name: "no executable path",
			goos: "linux", exe: "",
			fs:         fakeFS{},
			wantLooked: nil,
		},
		{
			name: "override wins on windows",
			goos: "windows", exe: "C:/tools/casebook.exe", override: "D:/Apps/Casebook.exe",
			fs: fakeFS{
				"C:/tools/casebook.exe": winCLI,
				"C:/tools/Casebook.exe": file("Casebook.exe"),
				"D:/Apps/Casebook.exe":  file("Casebook.exe"),
			},
			wantPath: "D:/Apps/Casebook.exe", wantFound: true,
			wantLooked: []string{"D:/Apps/Casebook.exe"},
		},
		{
			name: "override bundle on darwin opens",
			goos: "darwin", exe: "/Applications/Casebook.app/Contents/MacOS/casebook", override: "/Volumes/Dev/Casebook.app",
			fs: fakeFS{
				"/Applications/Casebook.app": dir("Casebook.app"),
				"/Volumes/Dev/Casebook.app":  dir("Casebook.app"),
			},
			wantPath: "/Volumes/Dev/Casebook.app", wantOpen: true, wantFound: true,
			wantLooked: []string{"/Volumes/Dev/Casebook.app"},
		},
		{
			name: "override executable on darwin runs directly",
			goos: "darwin", exe: "/usr/local/bin/casebook", override: "/opt/dev/Casebook",
			fs:       fakeFS{"/opt/dev/Casebook": file("Casebook")},
			wantPath: "/opt/dev/Casebook", wantFound: true,
			wantLooked: []string{"/opt/dev/Casebook"},
		},
		{
			name: "override on linux",
			goos: "linux", exe: "/usr/bin/casebook", override: "/opt/dev/casebook-desktop",
			fs:       fakeFS{"/opt/dev/casebook-desktop": file("casebook-desktop")},
			wantPath: "/opt/dev/casebook-desktop", wantFound: true,
			wantLooked: []string{"/opt/dev/casebook-desktop"},
		},
		{
			name: "missing override does not fall back",
			goos: "linux", exe: "/opt/casebook/bin/casebook", override: "/nope/Casebook",
			fs: fakeFS{
				"/opt/casebook/bin/casebook": file("casebook"),
				"/opt/casebook/bin/Casebook": file("Casebook"),
			},
			wantLooked: []string{"/nope/Casebook"},
		},
		{
			name: "override directory that is not a bundle",
			goos: "darwin", exe: "/usr/local/bin/casebook", override: "/opt/dev",
			fs:         fakeFS{"/opt/dev": dir("dev")},
			wantLooked: []string{"/opt/dev"},
		},
		{
			name: "override naming the CLI itself",
			goos: "linux", exe: "/usr/bin/casebook", override: "/usr/bin/casebook",
			fs:         fakeFS{"/usr/bin/casebook": file("casebook")},
			wantLooked: []string{"/usr/bin/casebook"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, looked, found := findDesktopApp(c.goos, filepath.FromSlash(c.exe), c.fs.stat,
				filepath.FromSlash(c.override), filepath.FromSlash(c.home))
			if found != c.wantFound {
				t.Fatalf("found = %v, want %v (target %+v, looked %v)", found, c.wantFound, got, looked)
			}
			if found && (filepath.ToSlash(got.Path) != c.wantPath || got.ViaOpen != c.wantOpen) {
				t.Errorf("target = %+v, want {%s %v}", got, c.wantPath, c.wantOpen)
			}
			if !found && got != (desktopTarget{}) {
				t.Errorf("target = %+v when not found", got)
			}
			var gotLooked []string
			for _, p := range looked {
				gotLooked = append(gotLooked, filepath.ToSlash(p))
			}
			if !reflect.DeepEqual(gotLooked, c.wantLooked) {
				t.Errorf("looked = %q, want %q", gotLooked, c.wantLooked)
			}
		})
	}
}

// uiRun runs casebook with stubbed OS hooks for the ui command.
type uiRun struct {
	exe      string
	exeErr   error
	fs       fakeFS
	override string
	startErr error

	started [][]string
}

func (u *uiRun) run(t *testing.T, args ...string) cliResult {
	t.Helper()
	var out, errb bytes.Buffer
	env := NewEnv(strings.NewReader(""), &out, &errb)
	env.Getenv = func(k string) string {
		if k == "CASEBOOK_UI" {
			return u.override
		}
		return ""
	}
	env.StdinIsTerminal = func() bool { return false }
	env.Executable = func() (string, error) { return u.exe, u.exeErr }
	env.Stat = u.fs.stat
	env.UserHomeDir = func() (string, error) { return "", errors.New("no home") }
	env.StartProcess = func(name string, args ...string) error {
		u.started = append(u.started, append([]string{name}, args...))
		return u.startErr
	}
	code := run(context.Background(), env, args)
	return cliResult{Stdout: out.String(), Stderr: errb.String(), Code: code}
}

func TestUILaunch(t *testing.T) {
	db := filepath.Join(t.TempDir(), "never.db")
	exe := filepath.FromSlash("/opt/casebook/bin/casebook")
	sibling := filepath.Join(filepath.Dir(exe), desktopExeName(hostGOOS))
	u := &uiRun{exe: exe, fs: fakeFS{
		filepath.ToSlash(exe):     file("casebook"),
		filepath.ToSlash(sibling): file(filepath.Base(sibling)),
	}}
	r := u.run(t, "--db", db, "ui")
	wantCode(t, r, 0)
	if r.Stdout != "Started Casebook from "+sibling+".\n" {
		t.Errorf("stdout = %q", r.Stdout)
	}
	if !reflect.DeepEqual(u.started, [][]string{{sibling}}) {
		t.Errorf("started = %q", u.started)
	}
	if _, err := os.Stat(db); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ui touched the database file: %v", err)
	}

	u.started = nil
	r = u.run(t, "--db", db, "--json", "ui")
	wantCode(t, r, 0)
	var res map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &res); err != nil {
		t.Fatal(err)
	}
	if res["path"] != sibling || res["launched"] != true {
		t.Errorf("ui --json = %v", res)
	}

	u.started, u.startErr = nil, errors.New("exec format error")
	r = u.run(t, "--db", db, "ui")
	wantCode(t, r, 1)
	wantContains(t, r.Stderr, "could not start", sibling, "exec format error")

	wantCode(t, u.run(t, "--db", db, "ui", "extra"), 2)
}

func TestUILaunchBundleViaOpen(t *testing.T) {
	old := hostGOOS
	hostGOOS = "darwin"
	t.Cleanup(func() { hostGOOS = old })
	u := &uiRun{
		exe:      "/usr/local/bin/casebook",
		override: "/Applications/Casebook.app",
		fs:       fakeFS{"/Applications/Casebook.app": dir("Casebook.app")},
	}
	r := u.run(t, "ui")
	wantCode(t, r, 0)
	if !reflect.DeepEqual(u.started, [][]string{{"/usr/bin/open", "/Applications/Casebook.app"}}) {
		t.Errorf("started = %q", u.started)
	}
	wantContains(t, r.Stdout, "Started Casebook from /Applications/Casebook.app.")
}

func TestUINotFound(t *testing.T) {
	db := filepath.Join(t.TempDir(), "never.db")
	u := &uiRun{exe: filepath.FromSlash("/home/me/bin/casebook"), fs: fakeFS{"/home/me/bin/casebook": file("casebook")}}
	r := u.run(t, "--db", db, "ui")
	wantCode(t, r, 1)
	wantContains(t, r.Stderr, "looked in", "installed separately", "CASEBOOK_UI", "Casebook")
	if strings.Count(r.Stderr, "\n") != 1 || len(u.started) != 0 {
		t.Errorf("stderr = %q, started = %q", r.Stderr, u.started)
	}
	if _, err := os.Stat(db); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ui touched the database file: %v", err)
	}

	u = &uiRun{exeErr: errors.New("no executable"), fs: fakeFS{}}
	r = u.run(t, "--db", db, "ui")
	wantCode(t, r, 1)
	wantContains(t, r.Stderr, "installed separately", "CASEBOOK_UI")
}

// TestResolveExecutable is the WR-06 regression: a failing symlink
// resolution falls back to the unresolved path instead of losing the CLI's
// location; only a failing os.Executable is an error.
func TestResolveExecutable(t *testing.T) {
	exe := func() (string, error) { return "/opt/casebook/bin/casebook", nil }
	evalErr := func(string) (string, error) { return "", errors.New("cannot resolve") }
	evalOK := func(string) (string, error) { return "/real/bin/casebook", nil }

	if got, err := resolveExecutable(exe, evalErr); err != nil || got != "/opt/casebook/bin/casebook" {
		t.Errorf("eval fails: got %q, %v; want the unresolved path", got, err)
	}
	if got, err := resolveExecutable(exe, evalOK); err != nil || got != "/real/bin/casebook" {
		t.Errorf("eval succeeds: got %q, %v; want the resolved path", got, err)
	}
	noExe := func() (string, error) { return "", errors.New("no executable") }
	if _, err := resolveExecutable(noExe, evalOK); err == nil {
		t.Error("os.Executable failure was not reported")
	}
}

// TestResolveExecutableHomebrew: a Homebrew keg path becomes the stable opt
// link when that link points at the same keg, so AI client configurations
// survive brew upgrade; otherwise the resolved path is kept.
func TestResolveExecutableHomebrew(t *testing.T) {
	keg := filepath.FromSlash("/opt/homebrew/Cellar/casebook/1.2.3/bin/casebook")
	opt := filepath.FromSlash("/opt/homebrew/opt/casebook/bin/casebook")
	links := map[string]string{
		filepath.FromSlash("/opt/homebrew/bin/casebook"): keg,
		opt: keg,
	}
	eval := func(p string) (string, error) {
		if r, ok := links[p]; ok {
			return r, nil
		}
		return p, nil
	}
	exe := func() (string, error) { return filepath.FromSlash("/opt/homebrew/bin/casebook"), nil }
	if got, err := resolveExecutable(exe, eval); err != nil || got != opt {
		t.Errorf("keg path: got %q, %v; want %q", got, err, opt)
	}

	// The opt link points at another version: keep the keg path.
	links[opt] = filepath.FromSlash("/opt/homebrew/Cellar/casebook/1.2.4/bin/casebook")
	if got, _ := resolveExecutable(exe, eval); got != keg {
		t.Errorf("stale opt link: got %q, want %q", got, keg)
	}

	// Not a casebook keg.
	other := filepath.FromSlash("/opt/homebrew/Cellar/other/1.0/bin/casebook")
	if got := homebrewStablePath(other, eval); got != other {
		t.Errorf("other formula: got %q, want it unchanged", got)
	}
	plain := filepath.FromSlash("/home/me/.local/bin/casebook")
	if got := homebrewStablePath(plain, eval); got != plain {
		t.Errorf("plain path: got %q, want it unchanged", got)
	}
}

// TestUIRelativeOverride is the IN-01 regression: a relative CASEBOOK_UI is
// made absolute, so the launched path is the file that was found rather
// than a PATH lookup of a bare name.
func TestUIRelativeOverride(t *testing.T) {
	abs, err := filepath.Abs("Casebook")
	if err != nil {
		t.Fatal(err)
	}
	u := &uiRun{
		exe:      filepath.FromSlash("/opt/casebook/bin/casebook"),
		override: "Casebook",
		fs:       fakeFS{filepath.ToSlash(abs): file("Casebook")},
	}
	r := u.run(t, "ui")
	wantCode(t, r, 0)
	if !reflect.DeepEqual(u.started, [][]string{{abs}}) {
		t.Errorf("started = %q, want %q", u.started, abs)
	}
	if got := absOverride("/Applications/Casebook.app"); got != "/Applications/Casebook.app" {
		t.Errorf("rooted override changed to %q", got)
	}
	if got := absOverride(""); got != "" {
		t.Errorf("empty override changed to %q", got)
	}
}
