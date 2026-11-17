package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func fileMode(t *testing.T, p string) os.FileMode {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestOpenPermissionsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions only")
	}
	dir := filepath.Join(t.TempDir(), "missing", "data")
	path := filepath.Join(dir, "casebook.db")
	db, err := Open(context.Background(), path, Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if m := fileMode(t, dir); m != 0o700 {
		t.Errorf("data dir mode = %o, want 700", m)
	}
	if m := fileMode(t, filepath.Dir(dir)); m != 0o700 {
		t.Errorf("created parent mode = %o, want 700", m)
	}
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if m := fileMode(t, p); m != 0o600 {
			t.Errorf("%s mode = %o, want 600", filepath.Base(p), m)
		}
	}
}

func TestOpenExistingDirUntouched(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions only")
	}
	dir := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := Open(context.Background(), filepath.Join(dir, "casebook.db"), Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if m := fileMode(t, dir); m != 0o755 {
		t.Fatalf("pre-existing dir mode = %o, want 755", m)
	}
	if m := fileMode(t, filepath.Join(dir, "casebook.db")); m != 0o600 {
		t.Fatalf("db mode = %o, want 600", m)
	}
}

func TestOpenOddPathCharacters(t *testing.T) {
	ctx := context.Background()
	name := "a b#c?d&_pragma=foreign_keys(0)"
	if runtime.GOOS == "windows" {
		name = "a b#c&d" // '?' is not a legal Windows file name character
	}
	dir := filepath.Join(t.TempDir(), name)
	path := filepath.Join(dir, "case book.db")
	db, err := Open(ctx, path, Config{})
	if err != nil {
		t.Fatalf("Open(%q): %v", path, err)
	}
	if err := db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		_, err := insertTestInvestigation(ctx, tx, "odd path")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var fk int
	if err := db.reader.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys = %d, %v: the path injected a parameter", fk, err)
	}
	db.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database not at the requested path: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(dir))
	if len(entries) != 1 {
		t.Fatalf("unexpected files next to the data dir: %v", entries)
	}

	db, err = Open(ctx, path, Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	inv, err := GetInvestigationByNum(ctx, db.reader, 1)
	if err != nil || inv.Title != "odd path" {
		t.Fatalf("round trip = %+v, %v", inv, err)
	}
}

func TestOpenPragmas(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "casebook.db"), Config{BusyTimeout: 1234 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for name, pool := range map[string]*sql.DB{"writer": db.writer, "reader": db.reader} {
		var (
			mode           string
			fk, busy, sync int
		)
		if err := pool.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&mode); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&busy); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRowContext(ctx, `PRAGMA synchronous`).Scan(&sync); err != nil {
			t.Fatal(err)
		}
		if mode != "wal" || fk != 1 || busy != 1234 || sync != 1 {
			t.Errorf("%s pragmas: journal_mode=%s foreign_keys=%d busy_timeout=%d synchronous=%d", name, mode, fk, busy, sync)
		}
	}
}
