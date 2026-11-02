//go:build windows

package doctor

import (
	"errors"
	"os"
	"path/filepath"
)

// errReadOnly is the reason for a database file with the read-only
// attribute.
var errReadOnly = errors.New("the file has the read-only attribute")

// probeWritable reports whether the database at path can be written without
// opening it: the file must not have the read-only attribute (os.Stat
// reports it as missing write bits), and a temp file must be creatable in
// its directory, where SQLite keeps its -wal and -shm files. The temp file
// is removed at once.
func probeWritable(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fi.Mode().Perm()&0o200 == 0 {
		return &os.PathError{Op: "stat", Path: path, Err: errReadOnly}
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".casebook-doctor-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return nil
}
