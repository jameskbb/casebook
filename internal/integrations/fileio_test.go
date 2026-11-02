package integrations

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fixedTime is the clock every backup test uses.
var fixedTime = time.Date(2026, 10, 2, 13, 4, 5, 0, time.UTC)

// fixedClock returns fixedTime.
func fixedClock() time.Time { return fixedTime }

// leftovers lists files in dir that a Casebook write left behind: backups
// and temp files.
func leftovers(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		if strings.Contains(e.Name(), ".casebook-") {
			out = append(out, e.Name())
		}
	}
	return out
}

// wantMode checks a file's permission bits (skipped on Windows, which has
// no Unix modes).
func wantMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != want {
		t.Errorf("mode of %s = %o, want %o", path, got, want)
	}
}

func TestCommitMissingFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a", "b", "config.json")
	f, err := readConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Exists || f.Data != nil || f.Mode != 0o600 || f.Path != path {
		t.Fatalf("missing file read as %+v", f)
	}
	backup, changed, err := f.commit([]byte("{}\n"), fixedTime)
	if err != nil || !changed || backup != "" {
		t.Fatalf("commit = %q, %v, %v; want a write with no backup", backup, changed, err)
	}
	if b, _ := os.ReadFile(path); string(b) != "{}\n" {
		t.Errorf("content = %q", b)
	}
	wantMode(t, path, 0o600)
	wantMode(t, filepath.Join(root, "a"), 0o700)
	wantMode(t, filepath.Join(root, "a", "b"), 0o700)
	if l := leftovers(t, filepath.Dir(path)); len(l) != 0 {
		t.Errorf("leftovers = %v", l)
	}
}

func TestCommitKeepsModeAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f, err := readConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	backup, changed, err := f.commit([]byte("new\n"), fixedTime)
	if err != nil || !changed {
		t.Fatalf("commit: %v, %v", changed, err)
	}
	if want := path + ".casebook-backup-20261002T130405Z"; backup != want {
		t.Errorf("backup = %s, want %s", backup, want)
	}
	if b, _ := os.ReadFile(backup); string(b) != "old\n" {
		t.Errorf("backup content = %q", b)
	}
	if b, _ := os.ReadFile(path); string(b) != "new\n" {
		t.Errorf("content = %q", b)
	}
	wantMode(t, path, 0o644)
	wantMode(t, backup, 0o600)
}

func TestCommitIdenticalWritesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("same\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	f, err := readConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	backup, changed, err := f.commit([]byte("same\n"), fixedTime)
	if err != nil || changed || backup != "" {
		t.Fatalf("commit = %q, %v, %v; want no write", backup, changed, err)
	}
	fi, _ := os.Stat(path)
	if !fi.ModTime().Equal(old) {
		t.Errorf("mtime changed to %v", fi.ModTime())
	}
	if l := leftovers(t, dir); len(l) != 0 {
		t.Errorf("leftovers = %v", l)
	}
}

func TestBackupCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, next := range []string{"v2\n", "v3\n"} {
		f, err := readConfigFile(path)
		if err != nil {
			t.Fatal(err)
		}
		backup, _, err := f.commit([]byte(next), fixedTime)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, backup)
	}
	base := path + ".casebook-backup-20261002T130405Z"
	if got[0] != base || got[1] != base+"-2" {
		t.Fatalf("backups = %v, want %s and %s-2", got, base, base)
	}
	if b, _ := os.ReadFile(got[1]); string(b) != "v2\n" {
		t.Errorf("second backup = %q, want v2", b)
	}
	if p, err := writeBackup(path, []byte("v3\n"), fixedTime); err != nil || p != base+"-3" {
		t.Errorf("writeBackup = %s, %v; want %s-3", p, err, base)
	}
}

func TestCommitChangedUnderneath(t *testing.T) {
	for _, existed := range []bool{true, false} {
		name := "missing file created concurrently"
		if existed {
			name = "existing file rewritten concurrently"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.json")
			if existed {
				if err := os.WriteFile(path, []byte("mine\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			f, err := readConfigFile(path)
			if err != nil {
				t.Fatal(err)
			}
			beforeRename = func(p string) {
				if err := os.WriteFile(p, []byte("theirs\n"), 0o600); err != nil {
					t.Error(err)
				}
			}
			t.Cleanup(func() { beforeRename = nil })
			backup, changed, err := f.commit([]byte("casebook\n"), fixedTime)
			if err == nil || changed || backup != "" {
				t.Fatalf("commit = %q, %v, %v; want an abort", backup, changed, err)
			}
			want := path + " changed while Casebook was updating it; nothing was written; rerun the command"
			if err.Error() != want {
				t.Errorf("error = %q, want %q", err, want)
			}
			if b, _ := os.ReadFile(path); string(b) != "theirs\n" {
				t.Errorf("content = %q, want the concurrent bytes", b)
			}
			if l := leftovers(t, dir); len(l) != 0 {
				t.Errorf("leftovers = %v", l)
			}
		})
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	for _, content := range []string{"one\n", "two\n"} {
		if err := writeFileAtomic(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if b, _ := os.ReadFile(path); string(b) != content {
			t.Errorf("content = %q, want %q", b, content)
		}
	}
	wantMode(t, path, 0o600)
	if l := leftovers(t, dir); len(l) != 0 {
		t.Errorf("leftovers = %v", l)
	}
}

func TestReadConfigFileRefusesDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := readConfigFile(dir)
	if _, ok := err.(*RefusedError); !ok {
		t.Fatalf("readConfigFile(dir) = %v, want a RefusedError", err)
	}
}

// commitN writes content to path through readConfigFile and commit at now
// and returns the backup it made.
func commitN(t *testing.T, path, content string, now time.Time) string {
	t.Helper()
	f, err := readConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	backup, changed, err := f.commit([]byte(content), now)
	if err != nil || !changed {
		t.Fatalf("commit(%q) = %v, %v", content, changed, err)
	}
	return backup
}

// regularBackups lists, sorted, the regular files in dir whose names are
// Casebook backups of base.
func regularBackups(t *testing.T, dir, base string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		if e.Type().IsRegular() && strings.HasPrefix(e.Name(), base) && backupStamp.MatchString(e.Name()[len(base):]) {
			out = append(out, e.Name())
		}
	}
	return out
}

func TestCommitKeepsNewestBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("v0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Names that look similar but are not Casebook backups of config.json:
	// pruning must never touch them.
	foreign := []string{
		"config.json.casebook-backup-20200101T000000Z.bak",
		"config.json.casebook-backup-20200101T000000Z-1",
		"config.json.casebook-backup-20200101T000000Z-01",
		"config.json.casebook-backup-2020",
		"other.json.casebook-backup-20200101T000000Z",
		"xconfig.json.casebook-backup-20200101T000000Z",
		"config.json.backup-20200101T000000Z",
		".config.json.casebook-123",
	}
	for _, name := range foreign {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// A directory and a symbolic link with backup names are not regular
	// files: never removed, and the link target is never touched.
	subdir := filepath.Join(dir, "config.json.casebook-backup-20000101T000000Z")
	if err := os.Mkdir(subdir, 0o700); err != nil {
		t.Fatal(err)
	}
	linkTarget := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(linkTarget, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config.json.casebook-backup-20000101T000001Z")
	haveLink := os.Symlink(linkTarget, link) == nil

	// Six changes a second apart, then two within one second (-2 suffix).
	var made []string
	for i := 1; i <= 6; i++ {
		made = append(made, commitN(t, path, "v"+strconv.Itoa(i)+"\n", fixedTime.Add(time.Duration(i)*time.Second)))
	}
	same := fixedTime.Add(10 * time.Second)
	made = append(made, commitN(t, path, "v7\n", same), commitN(t, path, "v8\n", same))
	if !strings.HasSuffix(made[7], "-2") {
		t.Fatalf("last backup %s has no -2 suffix", made[7])
	}

	var want []string
	for _, m := range made[3:] {
		want = append(want, filepath.Base(m))
	}
	if got := regularBackups(t, dir, "config.json"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("kept backups = %v, want %v", got, want)
	}
	if b, _ := os.ReadFile(made[7]); string(b) != "v7\n" {
		t.Errorf("newest backup = %q, want v7", b)
	}
	for _, name := range foreign {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(b) != "keep" {
			t.Errorf("foreign file %s was touched: %q, %v", name, b, err)
		}
	}
	if fi, err := os.Stat(subdir); err != nil || !fi.IsDir() {
		t.Errorf("directory with a backup name was touched: %v", err)
	}
	if haveLink {
		if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("symbolic link with a backup name was touched: %v", err)
		}
		if b, _ := os.ReadFile(linkTarget); string(b) != "target" {
			t.Errorf("link target changed: %q", b)
		}
	}
}

// A backup made with a clock earlier than the existing backups is still
// kept: it holds the bytes this write just replaced.
func TestPruneKeepsCurrentBackupWhenClockGoesBack(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("v0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 6; i++ {
		commitN(t, path, "v"+strconv.Itoa(i)+"\n", fixedTime.Add(time.Duration(i)*time.Hour))
	}
	early := commitN(t, path, "v7\n", fixedTime)
	if b, err := os.ReadFile(early); err != nil || string(b) != "v6\n" {
		t.Fatalf("current backup = %q, %v; want v6", b, err)
	}
	if n := len(regularBackups(t, dir, "settings.json")); n != keepBackups {
		t.Errorf("%d backups kept, want %d", n, keepBackups)
	}
}

// Pruning is best effort: a backup that cannot be removed does not fail the
// write.
func TestPruneFailureDoesNotFailWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("v0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	orig := removeBackup
	defer func() { removeBackup = orig }()
	tried := 0
	removeBackup = func(string) error { tried++; return os.ErrPermission }
	for i := 1; i <= 7; i++ {
		commitN(t, path, "v"+strconv.Itoa(i)+"\n", fixedTime.Add(time.Duration(i)*time.Second))
	}
	if tried == 0 {
		t.Fatal("pruning never tried to remove a backup")
	}
	if b, _ := os.ReadFile(path); string(b) != "v7\n" {
		t.Errorf("file = %q, want v7", b)
	}
	if n := len(regularBackups(t, dir, "config.json")); n != 7 {
		t.Errorf("%d backups, want all 7 after failed removals", n)
	}
}
