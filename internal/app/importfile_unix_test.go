//go:build unix

package app

import (
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// openImportWithin runs OpenImportFile(path) and fails the test when it
// blocks for 10 seconds.
func openImportWithin(t *testing.T, path string) (*os.File, error) {
	t.Helper()
	type result struct {
		f   *os.File
		err error
	}
	done := make(chan result, 1)
	go func() {
		f, err := OpenImportFile(path)
		done <- result{f, err}
	}()
	select {
	case r := <-done:
		return r.f, r.err
	case <-time.After(10 * time.Second):
		t.Fatal("OpenImportFile blocked")
		return nil, nil
	}
}

func wantNotRegular(t *testing.T, f *os.File, err error) {
	t.Helper()
	if f != nil {
		f.Close()
	}
	ve, ok := IsValidation(err)
	if !ok || ve.Field != "data" || ve.Message != "must be a regular file" {
		t.Fatalf("err = %v, want validation on data", err)
	}
}

// TestOpenImportFileRefusesFIFO: a FIFO with no writer is refused without
// blocking.
func TestOpenImportFileRefusesFIFO(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "pipe.json")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	f, err := openImportWithin(t, fifo)
	wantNotRegular(t, f, err)
}

// TestOpenImportFileFIFOSwappedIn replaces a regular file with a FIFO
// between the path check and the open. The non-blocking open returns at once
// and the check on the opened file refuses it.
func TestOpenImportFileFIFOSwappedIn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inv.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	beforeImportOpen = func() {
		if err := os.Remove(path); err != nil {
			t.Error(err)
		}
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Errorf("mkfifo: %v", err)
		}
	}
	t.Cleanup(func() { beforeImportOpen = nil })
	f, err := openImportWithin(t, path)
	wantNotRegular(t, f, err)
}

// TestOpenImportFileRegularReadsAfterNonblockingOpen: a regular file opened
// with O_NONBLOCK reads in full.
func TestOpenImportFileRegularReadsAfterNonblockingOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inv.json")
	want := make([]byte, 1<<20)
	for i := range want {
		want[i] = byte('a' + i%26)
	}
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := openImportWithin(t, path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := io.ReadAll(f)
	if err != nil || string(got) != string(want) {
		t.Fatalf("read %d bytes, %v; want %d", len(got), err, len(want))
	}
}
