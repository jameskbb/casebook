package app

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameskbb/casebook/internal/store"
)

// CheckNotInUse reports whether another connection holds the database file
// at dbPath right now: it returns ErrBusy when one does and nil otherwise.
// Every open Casebook engine counts, in this process or another one (the
// CLI, an MCP server, a second desktop window), even when it is idle.
//
// The probe opens the file in SQLite's exclusive locking mode with no busy
// timeout and asks for an exclusive transaction, which fails at once while
// any other connection has the file open. It never creates the file and
// writes nothing; closing it may checkpoint a leftover write-ahead log into
// the file, which keeps the same data. A missing file, or one SQLite cannot
// read as a database at all, is not in use as far as this check can tell, so
// it returns nil: the caller is about to replace exactly such a file. The
// answer is only good for the moment it was taken.
func CheckNotInUse(ctx context.Context, dbPath string) error {
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	p := filepath.ToSlash(abs)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // Windows drive path C:/x becomes /C:/x in a file URI.
	}
	u := url.URL{Scheme: "file", Path: p}
	db, err := sql.Open("sqlite", u.String()+"?mode=rw&_pragma=busy_timeout(0)&_pragma=locking_mode(EXCLUSIVE)")
	if err != nil {
		return nil
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return probeResult(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN EXCLUSIVE"); err != nil {
		return probeResult(err)
	}
	// Reading the schema takes the file lock even where BEGIN alone defers it.
	var n int
	err = conn.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master").Scan(&n)
	_, _ = conn.ExecContext(ctx, "ROLLBACK")
	return probeResult(err)
}

// probeResult keeps only the answer CheckNotInUse gives: lock contention is
// ErrBusy, a cancelled context passes through, anything else means the file
// is not held by another connection.
func probeResult(err error) error {
	switch {
	case err == nil:
		return nil
	case store.IsBusy(err):
		return ErrBusy
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	}
	return nil
}
