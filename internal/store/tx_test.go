package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/domain"
)

// holdWriteLock opens the file with the sqlite driver directly, takes one
// connection and runs BEGIN IMMEDIATE on it. The returned func rolls back.
func holdWriteLock(t *testing.T, path string) (release func()) {
	t.Helper()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { raw.Close() })
	conn, err := raw.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatalf("BEGIN IMMEDIATE: %v", err)
	}
	done := false
	release = func() {
		if done {
			return
		}
		done = true
		if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
			t.Errorf("ROLLBACK: %v", err)
		}
		conn.Close()
	}
	t.Cleanup(release)
	return release
}

func insertTestInvestigation(ctx context.Context, tx *sql.Tx, title string) (int64, error) {
	num, err := AllocateInvestigationNum(ctx, tx)
	if err != nil {
		return 0, err
	}
	return InsertInvestigation(ctx, tx, NewInvestigation{
		RefNum: num, Title: title, Status: domain.InvestigationStatusNew,
		CreatedAt: "2026-10-01T00:00:00.000Z", CreatedBy: domain.Actor{Kind: domain.ActorHuman, Client: "test"},
	})
}

func TestWithWriteTxBusyReturnsErrBusy(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "casebook.db")
	db, err := Open(ctx, path, Config{BusyTimeout: 20 * time.Millisecond, RetryBudget: 300 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	release := holdWriteLock(t, path)
	write := func() error {
		return db.WithWriteTx(ctx, func(tx *sql.Tx) error {
			_, err := insertTestInvestigation(ctx, tx, "busy")
			return err
		})
	}
	start := time.Now()
	err = write()
	elapsed := time.Since(start)
	if !errors.Is(err, domain.ErrBusy) {
		t.Fatalf("WithWriteTx under lock = %v, want ErrBusy", err)
	}
	if msg := err.Error(); strings.Contains(strings.ToLower(msg), "locked") || strings.Contains(msg, "SQLITE") {
		t.Fatalf("busy error leaks SQLite text: %q", msg)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("busy error took %v, want under 2s", elapsed)
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("busy error after %v: the retry budget (300ms) was not used", elapsed)
	}

	release()
	if err := write(); err != nil {
		t.Fatalf("WithWriteTx after release = %v", err)
	}
}

func TestWithWriteTxBusyRespectsContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "casebook.db")
	db, err := Open(context.Background(), path, Config{BusyTimeout: 20 * time.Millisecond, RetryBudget: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	holdWriteLock(t, path)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = db.WithWriteTx(ctx, func(tx *sql.Tx) error { return nil })
	if err == nil {
		t.Fatal("WithWriteTx under lock with a short context succeeded")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, domain.ErrBusy) {
		t.Fatalf("err = %v, want context deadline or ErrBusy", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("WithWriteTx ignored the context deadline (%v)", time.Since(start))
	}
}

func TestWithWriteTxRollbackLeavesNoRows(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	sentinel := errors.New("fail mid-transaction")
	err := db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		id, err := insertTestInvestigation(ctx, tx, "rollback canary")
		if err != nil {
			return err
		}
		if _, err := InsertActivity(ctx, tx, NewActivity{
			InvestigationID: id, Actor: domain.Actor{Kind: domain.ActorHuman, Client: "test"},
			Action: "investigation.create", TargetRef: "INV-0001", Summary: "s", CreatedAt: "2026-10-01T00:00:00.000Z",
		}); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithWriteTx = %v, want the closure's error", err)
	}
	for _, table := range []string{"investigations", "activity", "search_index", "counters"} {
		var n int
		if err := db.reader.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s has %d rows after rollback, want 0", table, n)
		}
	}
	// The connection is clean: the next write succeeds and gets INV 1.
	var num int64
	if err := db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		var err error
		num, err = AllocateInvestigationNum(ctx, tx)
		return err
	}); err != nil || num != 1 {
		t.Fatalf("next write = %d, %v; want 1", num, err)
	}
}

func TestWithWriteTxNonBusyErrorNotRetried(t *testing.T) {
	db := openTestDB(t)
	calls := 0
	sentinel := errors.New("boom")
	err := db.WithWriteTx(context.Background(), func(tx *sql.Tx) error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) || calls != 1 {
		t.Fatalf("err = %v, calls = %d; want sentinel once", err, calls)
	}
}

func TestWithWriteTxIsBusyAndConstraintClassify(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	err := db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO counters (name, value) VALUES ('x', NULL)`)
		return err
	})
	if !IsConstraint(err) || IsBusy(err) {
		t.Fatalf("NOT NULL violation: IsConstraint=%v IsBusy=%v (%v)", IsConstraint(err), IsBusy(err), err)
	}
	if IsBusy(errors.New("database is locked")) || IsConstraint(errors.New("constraint failed")) {
		t.Fatal("classification must use SQLite codes, not error text")
	}
	if IsBusy(nil) || IsConstraint(nil) {
		t.Fatal("nil classified as an error")
	}
}
