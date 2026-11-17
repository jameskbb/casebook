//go:build !unix

package app

import "os"

// openNoBlock is os.Open. Windows has no FIFOs in the file system (named
// pipes live in their own namespace, which Stat reports as not regular), so
// a plain open of a path that was a regular file cannot block on one.
func openNoBlock(path string) (*os.File, error) {
	return os.Open(path)
}
