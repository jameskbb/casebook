//go:build unix

package doctor

import (
	"os"
	"syscall"
)

// accessWriteOK is access(2)'s W_OK, the same value on every unix.
const accessWriteOK = 0x2

// probeWritable reports whether this process may write the file at path,
// using access(2) so no descriptor to the database is ever opened. Closing
// any descriptor to a file releases every POSIX lock the process holds on
// it, including the locks of SQLite connections that are still open in this
// process (SQLite's "How To Corrupt" section 2.2), so the probe must not
// open the file.
func probeWritable(path string) error {
	if err := syscall.Access(path, accessWriteOK); err != nil {
		return &os.PathError{Op: "access", Path: path, Err: err}
	}
	return nil
}
