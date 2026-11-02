//go:build unix

package cli

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestImportRefusesFIFO is S7: a FIFO with no writer would block an open or
// read forever, so import refuses it before opening it.
func TestImportRefusesFIFO(t *testing.T) {
	_, db, _ := seededApp(t)
	fifo := filepath.Join(t.TempDir(), "pipe.json")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	done := make(chan cliResult, 1)
	go func() { done <- runCLI(t, "", "--db", db, "import", fifo) }()
	select {
	case r := <-done:
		wantCode(t, r, 2)
		if r.Stderr != "casebook: invalid data: must be a regular file\n" {
			t.Errorf("stderr = %q", r.Stderr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("import of a FIFO blocked")
	}
}
