package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
)

// runTerminal runs casebook like runCLI but lets the test say whether stdin
// is an interactive terminal.
func runTerminal(t *testing.T, terminal bool, stdin string, args ...string) cliResult {
	t.Helper()
	var out, errb bytes.Buffer
	env := NewEnv(strings.NewReader(stdin), &out, &errb)
	env.Getenv = func(string) string { return "" }
	env.StdinIsTerminal = func() bool { return terminal }
	code := run(context.Background(), env, args)
	return cliResult{Stdout: out.String(), Stderr: errb.String(), Code: code}
}

// investigationCount reads the number of investigations through the CLI.
func investigationCount(t *testing.T, db string) int {
	t.Helper()
	r := runCLI(t, "", "--db", db, "--json", "status")
	wantCode(t, r, 0)
	var info app.StatusInfo
	decodeJSON(t, r.Stdout, &info)
	return info.Counts.Investigations
}

func TestBackup(t *testing.T) {
	_, db, _ := seededApp(t)
	dest := filepath.Join(t.TempDir(), "b.db")
	r := runCLI(t, "", "--db", db, "backup", dest)
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "Backup written to "+dest, "bytes", "schema version", "1 cases")
	if _, err := os.Stat(dest); err != nil {
		t.Fatal(err)
	}

	r = runCLI(t, "", "--db", db, "backup", dest)
	wantCode(t, r, 1)
	if !strings.HasPrefix(r.Stderr, "casebook: ") {
		t.Errorf("stderr = %q", r.Stderr)
	}

	dest2 := filepath.Join(t.TempDir(), "b2.db")
	r = runCLI(t, "", "--db", db, "--json", "backup", dest2)
	wantCode(t, r, 0)
	var info app.BackupInfo
	decodeJSON(t, r.Stdout, &info)
	if info.Path != dest2 || info.SizeBytes <= 0 || info.Counts.Investigations != 1 {
		t.Errorf("backup info = %+v", info)
	}
	wantCode(t, runCLI(t, "", "--db", db, "backup"), 2)
}

func TestRestore(t *testing.T) {
	_, db, _ := seededApp(t)
	backup := filepath.Join(t.TempDir(), "b.db")
	wantCode(t, runCLI(t, "", "--db", db, "backup", backup), 0)
	wantCode(t, runCLI(t, "", "--db", db, "investigation", "create", "--title", "After the backup"), 0)
	if n := investigationCount(t, db); n != 2 {
		t.Fatalf("investigations before restore = %d", n)
	}

	t.Run("non-terminal without --yes", func(t *testing.T) {
		r := runTerminal(t, false, "yes\n", "--db", db, "restore", backup)
		wantCode(t, r, 2)
		wantContains(t, r.Stderr, "--yes")
		if n := investigationCount(t, db); n != 2 {
			t.Errorf("restore changed data without --yes: %d investigations", n)
		}
	})
	t.Run("terminal answers no", func(t *testing.T) {
		r := runTerminal(t, true, "no\n", "--db", db, "restore", backup)
		wantCode(t, r, 1)
		wantContains(t, r.Stderr, `Type "yes" to continue`, "restore cancelled")
		if n := investigationCount(t, db); n != 2 {
			t.Errorf("cancelled restore changed data: %d investigations", n)
		}
	})
	t.Run("terminal at end of input", func(t *testing.T) {
		r := runTerminal(t, true, "", "--db", db, "restore", backup)
		wantCode(t, r, 1)
		wantContains(t, r.Stderr, "restore cancelled")
	})
	t.Run("not a backup", func(t *testing.T) {
		bad := filepath.Join(t.TempDir(), "not-a-backup.db")
		if err := os.WriteFile(bad, []byte("definitely not sqlite"), 0o600); err != nil {
			t.Fatal(err)
		}
		r := runCLI(t, "", "--db", db, "restore", bad, "--yes")
		wantCode(t, r, 1)
		wantContains(t, r.Stderr, "not a valid Casebook backup")
		// A bad file fails before the confirmation, even interactively.
		r = runTerminal(t, true, "yes\n", "--db", db, "restore", bad)
		wantCode(t, r, 1)
		if strings.Contains(r.Stderr, "Type \"yes\"") {
			t.Errorf("prompted before validating the backup: %q", r.Stderr)
		}
		if n := investigationCount(t, db); n != 2 {
			t.Errorf("bad restore changed data: %d investigations", n)
		}
	})
	t.Run("missing source", func(t *testing.T) {
		r := runCLI(t, "", "--db", db, "restore", filepath.Join(t.TempDir(), "missing.db"), "--yes")
		wantCode(t, r, 3)
	})
	t.Run("--yes", func(t *testing.T) {
		r := runCLI(t, "", "--db", db, "restore", backup, "--yes")
		wantCode(t, r, 0)
		wantContains(t, r.Stdout, "Restored from "+backup, "Safety copy of the previous data: ")
		safety := strings.TrimSpace(r.Stdout[strings.Index(r.Stdout, "previous data: ")+len("previous data: "):])
		if _, err := os.Stat(safety); err != nil {
			t.Errorf("safety copy %q: %v", safety, err)
		}
		if n := investigationCount(t, db); n != 1 {
			t.Errorf("investigations after restore = %d, want 1", n)
		}
		wantCode(t, runCLI(t, "", "--db", db, "investigation", "show", "INV-0002"), 3)
	})
	t.Run("terminal answers yes with --json", func(t *testing.T) {
		wantCode(t, runCLI(t, "", "--db", db, "investigation", "create", "--title", "Again"), 0)
		r := runTerminal(t, true, " YES \n", "--db", db, "--json", "restore", backup)
		wantCode(t, r, 0)
		var res app.RestoreResult
		decodeJSON(t, r.Stdout, &res)
		if res.SourcePath != backup || res.SafetyCopyPath == "" || res.Counts.Investigations != 1 {
			t.Errorf("restore result = %+v", res)
		}
		if n := investigationCount(t, db); n != 1 {
			t.Errorf("investigations after restore = %d, want 1", n)
		}
	})
}

// syncBuffer is a bytes.Buffer safe for one writer and a polling reader.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestInterruptedPrompts is the WR-05 regression: Ctrl-C (a cancelled
// context) ends the restore confirmation and an import from stdin at once,
// without waiting for input, and changes nothing.
func TestInterruptedPrompts(t *testing.T) {
	_, db, _ := seededApp(t)
	backup := filepath.Join(t.TempDir(), "b.db")
	wantCode(t, runCLI(t, "", "--db", db, "backup", backup), 0)
	wantCode(t, runCLI(t, "", "--db", db, "investigation", "create", "--title", "After the backup"), 0)

	// interrupted runs args and cancels once stderr shows waitFor (the
	// command is then blocked on input), or after 100ms when waitFor is
	// empty. Waiting for the prompt keeps a slow run (the race detector)
	// from cancelling before the command ever reaches it.
	interrupted := func(t *testing.T, terminal bool, waitFor string, args ...string) cliResult {
		t.Helper()
		stdin, w := io.Pipe() // never written: a read blocks until cancel
		t.Cleanup(func() { _ = w.Close() })
		var out, errb syncBuffer
		env := NewEnv(stdin, &out, &errb)
		env.Getenv = func(string) string { return "" }
		env.StdinIsTerminal = func() bool { return terminal }
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if waitFor == "" {
			time.AfterFunc(100*time.Millisecond, cancel)
		} else {
			go func() {
				for !strings.Contains(errb.String(), waitFor) {
					if ctx.Err() != nil {
						return
					}
					time.Sleep(10 * time.Millisecond)
				}
				cancel()
			}()
		}
		done := make(chan int, 1)
		go func() { done <- run(ctx, env, args) }()
		select {
		case code := <-done:
			return cliResult{Stdout: out.String(), Stderr: errb.String(), Code: code}
		case <-time.After(10 * time.Second):
			t.Fatal("command still waiting for input after the interrupt")
			return cliResult{}
		}
	}

	t.Run("restore prompt", func(t *testing.T) {
		r := interrupted(t, true, `Type "yes" to continue`, "--db", db, "restore", backup)
		wantCode(t, r, 1)
		wantContains(t, r.Stderr, `Type "yes" to continue`, "restore cancelled; nothing was changed")
		if n := investigationCount(t, db); n != 2 {
			t.Errorf("interrupted restore changed data: %d investigations", n)
		}
	})
	t.Run("import from stdin", func(t *testing.T) {
		r := interrupted(t, false, "", "--db", db, "import", "-")
		wantCode(t, r, 1)
		wantContains(t, r.Stderr, "import cancelled; nothing was changed")
		if n := investigationCount(t, db); n != 2 {
			t.Errorf("interrupted import changed data: %d investigations", n)
		}
	})
}

// TestRestoreNullStdin is the IN-03 regression: stdin redirected from the
// null device (cron, systemd, CI) is not a terminal, so restore without
// --yes is a usage error (exit 2) rather than a prompt nobody can answer.
func TestRestoreNullStdin(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = null.Close() })
	var out, errb bytes.Buffer
	env := NewEnv(null, &out, &errb)
	if env.StdinIsTerminal() {
		t.Fatalf("%s reported as a terminal", os.DevNull)
	}
	_, db, _ := seededApp(t)
	backup := filepath.Join(t.TempDir(), "b.db")
	wantCode(t, runCLI(t, "", "--db", db, "backup", backup), 0)
	env.Getenv = func(string) string { return "" }
	code := run(context.Background(), env, []string{"--db", db, "restore", backup})
	r := cliResult{Stdout: out.String(), Stderr: errb.String(), Code: code}
	wantCode(t, r, 2)
	wantContains(t, r.Stderr, "--yes")
}
