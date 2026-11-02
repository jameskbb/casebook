//go:build unix

package app

import (
	"os"
	"syscall"
)

// openNoBlock opens path read-only with O_NONBLOCK, so opening a FIFO that
// has no writer returns at once instead of waiting for one (OpenImportFile
// then refuses it as not a regular file). O_NONBLOCK has no effect on reads
// of a regular file, the only kind OpenImportFile returns, so it stays set.
func openNoBlock(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
