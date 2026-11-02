package integrations

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jameskbb/casebook/internal/paths"
)

// configFile is a client config file as read before a change (D-04).
type configFile struct {
	// Path is the file actually written: symlinks are resolved so a
	// symlinked config stays a symlink and its target is updated.
	Path string
	// Data is the file's bytes; nil when the file is missing.
	Data []byte
	// Exists reports whether the file existed.
	Exists bool
	// Mode is the original permission bits; 0600 when the file is missing.
	Mode fs.FileMode
}

// backupLayout is the UTC timestamp in backup file names.
const backupLayout = "20060102T150405Z"

// maxBackupSuffix bounds the -N suffixes tried when backup names collide.
const maxBackupSuffix = 1000

// beforeRename, when set by a test, runs after the new content is written to
// the temp file and before the changed-underneath check and rename.
var beforeRename func(path string)

// readConfigFile reads the config file at path. A missing file is not an
// error: it reads as Exists false with mode 0600.
func readConfigFile(path string) (configFile, error) {
	f := configFile{Path: path, Mode: 0o600}
	resolved, err := filepath.EvalSymlinks(path)
	switch {
	case err == nil:
		f.Path = resolved
	case errors.Is(err, fs.ErrNotExist):
		if fi, lerr := os.Lstat(path); lerr == nil && fi.Mode()&fs.ModeSymlink != 0 {
			return f, &RefusedError{File: path, Reason: "it is a symbolic link whose target does not exist"}
		}
	default:
		return f, fmt.Errorf("read %s: %w", path, err)
	}
	fi, err := os.Stat(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, fmt.Errorf("read %s: %w", f.Path, err)
	}
	if !fi.Mode().IsRegular() {
		return f, &RefusedError{File: f.Path, Reason: "it is not a regular file"}
	}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return f, fmt.Errorf("read %s: %w", f.Path, err)
	}
	f.Data = data
	f.Exists = true
	f.Mode = fi.Mode().Perm()
	return f, nil
}

// commit replaces the file with newData (D-04). Identical bytes write
// nothing and report changed false. Otherwise: create the parent directory
// (0700) when missing, back up the existing bytes, write a temp file in the
// same directory with the original mode, check that the file still holds
// the bytes that were read, and rename the temp file over it. Any failure
// removes the temp file and the backup.
func (f configFile) commit(newData []byte, now time.Time) (backup string, changed bool, err error) {
	if f.Exists && bytes.Equal(f.Data, newData) {
		return "", false, nil
	}
	dir := filepath.Dir(f.Path)
	if err := paths.EnsureDir(dir); err != nil {
		return "", false, fmt.Errorf("create %s: %w", dir, err)
	}
	if f.Exists {
		backup, err = writeBackup(f.Path, f.Data, now)
		if err != nil {
			return "", false, err
		}
	}
	tmp, err := writeTemp(f.Path, newData, f.Mode)
	if err != nil {
		removeIf(backup)
		return "", false, err
	}
	fail := func(err error) (string, bool, error) {
		removeIf(tmp)
		removeIf(backup)
		return "", false, err
	}
	if beforeRename != nil {
		beforeRename(f.Path)
	}
	cur, err := os.ReadFile(f.Path)
	switch {
	case err == nil:
		if !f.Exists || !bytes.Equal(cur, f.Data) {
			return fail(changedUnderneath(f.Path))
		}
	case errors.Is(err, fs.ErrNotExist):
		if f.Exists {
			return fail(changedUnderneath(f.Path))
		}
	default:
		return fail(fmt.Errorf("read %s: %w", f.Path, err))
	}
	if err := os.Rename(tmp, f.Path); err != nil {
		return fail(fmt.Errorf("replace %s: %w", f.Path, err))
	}
	syncDir(dir)
	if backup != "" {
		pruneBackups(f.Path, backup, keepBackups)
	}
	return backup, true, nil
}

// changedUnderneath is the error for a file another program changed while
// Casebook was updating it.
func changedUnderneath(path string) error {
	return fmt.Errorf("%s changed while Casebook was updating it; nothing was written; rerun the command", path)
}

// writeFileAtomic replaces path with data through a temp file in the same
// directory (write, fsync, rename), without a backup or comparison. The
// parent directory must exist.
func writeFileAtomic(path string, data []byte, mode fs.FileMode) error {
	tmp, err := writeTemp(path, data, mode)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		removeIf(tmp)
		return fmt.Errorf("replace %s: %w", path, err)
	}
	syncDir(filepath.Dir(path))
	return nil
}

// writeTemp writes data to a new temp file next to path with the given
// mode, syncs and closes it, and returns its name.
func writeTemp(path string, data []byte, mode fs.FileMode) (string, error) {
	dir := filepath.Dir(path)
	t, err := os.CreateTemp(dir, "."+filepath.Base(path)+".casebook-*")
	if err != nil {
		return "", fmt.Errorf("create a temp file in %s: %w", dir, err)
	}
	name := t.Name()
	fail := func(err error) (string, error) {
		t.Close()
		os.Remove(name)
		return "", fmt.Errorf("write %s: %w", name, err)
	}
	if runtime.GOOS != "windows" {
		if err := t.Chmod(mode); err != nil {
			return fail(err)
		}
	}
	if _, err := t.Write(data); err != nil {
		return fail(err)
	}
	if err := t.Sync(); err != nil {
		return fail(err)
	}
	if err := t.Close(); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("write %s: %w", name, err)
	}
	return name, nil
}

// backupName is the n-th candidate backup name for path at now (n starts at
// 1, which has no suffix).
func backupName(path string, now time.Time, n int) string {
	name := path + ".casebook-backup-" + now.UTC().Format(backupLayout)
	if n > 1 {
		name += "-" + strconv.Itoa(n)
	}
	return name
}

// writeBackup writes data to a new backup file next to path (mode 0600,
// created exclusively, so an existing file is never overwritten) and returns
// its name.
func writeBackup(path string, data []byte, now time.Time) (string, error) {
	for n := 1; n <= maxBackupSuffix; n++ {
		name := backupName(path, now, n)
		bf, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("write backup %s: %w", name, err)
		}
		_, werr := bf.Write(data)
		if werr == nil {
			werr = bf.Sync()
		}
		if cerr := bf.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			os.Remove(name)
			return "", fmt.Errorf("write backup %s: %w", name, werr)
		}
		return name, nil
	}
	return "", fmt.Errorf("too many backups of %s at %s", path, now.UTC().Format(backupLayout))
}

// keepBackups is how many Casebook backups of one config file are kept; a
// successful write removes older ones (the files hold the whole config,
// for ~/.claude.json account details and per-project state).
const keepBackups = 5

// backupStamp matches the part of a backup name after "<file>": the UTC
// timestamp and an optional -N suffix (N >= 2), as backupName writes it.
var backupStamp = regexp.MustCompile(`^\.casebook-backup-([0-9]{8}T[0-9]{6}Z)(?:-([2-9]|[1-9][0-9]+))?$`)

// removeBackup removes one old backup; tests replace it to make removal
// fail.
var removeBackup = os.Remove

// backupFile is one existing backup of a config file.
type backupFile struct {
	name  string
	stamp string
	n     int
}

// pruneBackups removes the oldest Casebook backups of path so that at most
// keep remain, always keeping current (the backup this write just made).
// Only regular files in path's directory whose names are exactly
// <base of path>.casebook-backup-<timestamp>[-N] are considered; symbolic
// links, directories and every other name are never touched. It is best
// effort: the write already succeeded, so errors are ignored.
func pruneBackups(path, current string, keep int) {
	dir, base := filepath.Dir(path), filepath.Base(path)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var olds []backupFile
	for _, e := range ents {
		name := e.Name()
		if name == filepath.Base(current) || !strings.HasPrefix(name, base) || !e.Type().IsRegular() {
			continue
		}
		m := backupStamp.FindStringSubmatch(name[len(base):])
		if m == nil {
			continue
		}
		n := 1
		if m[2] != "" {
			v, err := strconv.Atoi(m[2])
			if err != nil {
				continue
			}
			n = v
		}
		olds = append(olds, backupFile{name: name, stamp: m[1], n: n})
	}
	if len(olds) < keep {
		return
	}
	// Newest first: by timestamp, then by suffix within one second.
	sort.Slice(olds, func(i, j int) bool {
		if olds[i].stamp != olds[j].stamp {
			return olds[i].stamp > olds[j].stamp
		}
		return olds[i].n > olds[j].n
	})
	for _, b := range olds[keep-1:] {
		full := filepath.Join(dir, b.name)
		// Re-check right before removing: still a regular file, not a link.
		if fi, err := os.Lstat(full); err == nil && fi.Mode().IsRegular() {
			_ = removeBackup(full)
		}
	}
}

// removeIf removes name when it is not empty, ignoring errors.
func removeIf(name string) {
	if name != "" {
		os.Remove(name)
	}
}

// syncDir makes a rename in dir durable where the OS supports it. Errors are
// ignored: the rename itself already succeeded.
func syncDir(dir string) {
	if runtime.GOOS == "windows" {
		return
	}
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
