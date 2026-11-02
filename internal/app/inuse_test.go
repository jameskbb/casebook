package app_test

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

func TestCheckNotInUse(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db := filepath.Join(dir, "casebook.db")

	// A missing file is not in use, and the probe does not create it.
	if err := app.CheckNotInUse(ctx, db); err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if _, err := os.Stat(db); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("probe created the file: %v", err)
	}

	// An open engine holds the file, even when idle.
	a, err := app.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.CheckNotInUse(ctx, db); !errors.Is(err, app.ErrBusy) {
		t.Fatalf("open engine: %v, want ErrBusy", err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := app.CheckNotInUse(ctx, db); err != nil {
		t.Fatalf("closed engine: %v", err)
	}
	// The probe left a usable database behind.
	b, err := app.Open(db)
	if err != nil {
		t.Fatalf("reopen after probe: %v", err)
	}
	_ = b.Close()

	// A file that is not a database is not in use.
	junk := filepath.Join(dir, "junk.db")
	if err := os.WriteFile(junk, []byte("this is not a database, it is a broken file of sufficient length"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.CheckNotInUse(ctx, junk); err != nil {
		t.Fatalf("junk file: %v", err)
	}
}

// TestCheckNotInUseOtherProcess holds the database open in a child process
// (this test binary re-run as a helper) and probes it from here.
func TestCheckNotInUseOtherProcess(t *testing.T) {
	if path := os.Getenv("CASEBOOK_TEST_HOLD_DB"); path != "" {
		holdDatabase(path)
		return
	}
	db := filepath.Join(t.TempDir(), "casebook.db")
	cmd := exec.Command(os.Args[0], "-test.run=^TestCheckNotInUseOtherProcess$")
	cmd.Env = append(os.Environ(), "CASEBOOK_TEST_HOLD_DB="+db)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "holding\n" {
		_ = cmd.Process.Kill()
		t.Fatalf("helper said %q, %v", line, err)
	}
	busy := app.CheckNotInUse(context.Background(), db)
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper: %v", err)
	}
	if !errors.Is(busy, app.ErrBusy) {
		t.Fatalf("held by another process: %v, want ErrBusy", busy)
	}
	if err := app.CheckNotInUse(context.Background(), db); err != nil {
		t.Fatalf("after the helper exited: %v", err)
	}
}

// holdDatabase opens the engine at path, says "holding" and keeps it open
// until stdin closes.
func holdDatabase(path string) {
	a, err := app.Open(path)
	if err != nil {
		os.Stdout.WriteString("open failed: " + err.Error() + "\n")
		os.Exit(1)
	}
	if _, err := a.ListInvestigations(context.Background(), app.ListOptions{}); err != nil {
		os.Exit(1)
	}
	os.Stdout.WriteString("holding\n")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	_ = a.Close()
}
