package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// seededDB opens a database with one investigation.
func seededDB(t *testing.T) *DB {
	t.Helper()
	db := openTestDB(t)
	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		_, err := insertTestInvestigation(ctx, tx, "Snapshot me")
		return err
	})
	return db
}

// snapshotOf writes a VACUUM INTO snapshot of db into a temp dir.
func snapshotOf(t *testing.T, db *DB) string {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "snap.db")
	if err := db.VacuumInto(context.Background(), dest); err != nil {
		t.Fatalf("VacuumInto: %v", err)
	}
	return dest
}

// tamper runs statements on a file without changing its journal mode.
func tamper(t *testing.T, path string, stmts ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", fileURI(path)+"?_pragma=foreign_keys(0)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("tamper %q: %v", s, err)
		}
	}
}

// tamperOneConn runs statements on a file over a single connection, so
// connection-scoped pragmas such as writable_schema apply to the later
// statements.
func tamperOneConn(t *testing.T, path string, stmts ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", fileURI(path)+"?_pragma=foreign_keys(0)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("tamper %q: %v", s, err)
		}
	}
}

// validateFile opens path read-only and validates it.
func validateFile(t *testing.T, path string) (int, error) {
	t.Helper()
	ro, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer ro.Close()
	v, _, err := ValidateSnapshot(context.Background(), ro, LatestVersion())
	return v, err
}

func TestVacuumIntoOwnerOnlySnapshot(t *testing.T) {
	db := seededDB(t)
	dest := snapshotOf(t, db)
	if runtime.GOOS != "windows" {
		if m := fileMode(t, dest); m != 0o600 {
			t.Fatalf("snapshot mode = %o, want 600", m)
		}
	}
	ro, err := OpenReadOnly(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	v, counts, err := ValidateSnapshot(context.Background(), ro, LatestVersion())
	if err != nil || v != LatestVersion() || counts.Investigations != 1 {
		t.Fatalf("ValidateSnapshot = %d, %+v, %v", v, counts, err)
	}
	var mode string
	if err := ro.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "delete" {
		t.Fatalf("snapshot journal_mode = %q, %v; want delete", mode, err)
	}
	if _, err := ro.Exec(`DELETE FROM investigations`); err == nil {
		t.Fatal("read-only snapshot accepted a write")
	}
}

func TestVacuumIntoRefusesExisting(t *testing.T) {
	db := seededDB(t)
	dest := filepath.Join(t.TempDir(), "keep.db")
	if err := os.WriteFile(dest, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := db.VacuumInto(context.Background(), dest)
	if !errors.Is(err, ErrDestinationExists) {
		t.Fatalf("err = %v, want ErrDestinationExists", err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "keep me" {
		t.Fatalf("existing file changed: %q", b)
	}
}

func TestVacuumIntoMissingDirLeavesNothing(t *testing.T) {
	db := seededDB(t)
	dest := filepath.Join(t.TempDir(), "missing", "snap.db")
	if err := db.VacuumInto(context.Background(), dest); err == nil {
		t.Fatal("VacuumInto into a missing directory succeeded")
	}
}

func TestValidateSnapshotRejectsTampering(t *testing.T) {
	db := seededDB(t)
	good := snapshotOf(t, db)
	goodBytes, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		make   func(t *testing.T, path string)
		reason string
	}{
		{"random bytes", func(t *testing.T, p string) {
			os.WriteFile(p, bytes.Repeat([]byte("not a database "), 512), 0o600)
		}, "not a readable SQLite database"},
		{"truncated", func(t *testing.T, p string) {
			os.WriteFile(p, goodBytes[:len(goodBytes)/2], 0o600)
		}, ""},
		{"plain sqlite", func(t *testing.T, p string) {
			tamper(t, p, `CREATE TABLE notes (body TEXT)`)
		}, "no schema_migrations table"},
		{"extra view", func(t *testing.T, p string) {
			os.WriteFile(p, goodBytes, 0o600)
			tamper(t, p, `CREATE VIEW sneaky AS SELECT * FROM investigations`)
		}, "unexpected view sneaky"},
		{"extra trigger", func(t *testing.T, p string) {
			os.WriteFile(p, goodBytes, 0o600)
			tamper(t, p, `CREATE TRIGGER evil AFTER INSERT ON settings BEGIN DELETE FROM investigations; END`)
		}, "unexpected trigger evil"},
		{"trigger named like an internal object", func(t *testing.T, p string) {
			os.WriteFile(p, goodBytes, 0o600)
			tamper(t, p, `CREATE TRIGGER sqlitex_evil AFTER INSERT ON settings BEGIN DELETE FROM investigations; END`)
		}, "unexpected trigger sqlitex_evil"},
		{"view named like an internal object", func(t *testing.T, p string) {
			os.WriteFile(p, goodBytes, 0o600)
			tamper(t, p, `CREATE VIEW sqlite1_view AS SELECT * FROM investigations`)
		}, "unexpected view sqlite1_view"},
		{"expression index checked before integrity_check", func(t *testing.T, p string) {
			os.WriteFile(p, goodBytes, 0o600)
			// The index content no longer matches its definition, so
			// integrity_check would fail; the schema comparison must refuse
			// the extra index first, before any expression is evaluated.
			tamperOneConn(t, p,
				`CREATE INDEX evil_idx ON investigations (lower(title))`,
				`PRAGMA writable_schema = ON`,
				`UPDATE sqlite_schema SET sql = 'CREATE INDEX evil_idx ON investigations (upper(title))' WHERE name = 'evil_idx'`,
				`PRAGMA writable_schema = OFF`)
		}, "unexpected index evil_idx"},
		{"altered table", func(t *testing.T, p string) {
			os.WriteFile(p, goodBytes, 0o600)
			tamper(t, p, `ALTER TABLE evidence ADD COLUMN extra TEXT`)
		}, "table evidence differs"},
		{"newer version", func(t *testing.T, p string) {
			os.WriteFile(p, goodBytes, 0o600)
			tamper(t, p, `INSERT INTO schema_migrations (version, applied_at) VALUES (99, 'x')`)
		}, "schema version 99 is not supported"},
		{"oversized title", func(t *testing.T, p string) {
			os.WriteFile(p, goodBytes, 0o600)
			tamper(t, p, `UPDATE investigations SET title = '`+strings.Repeat("t", 300)+`'`)
		}, "investigations.title holds data outside Casebook limits"},
		{"NUL in summary", func(t *testing.T, p string) {
			os.WriteFile(p, goodBytes, 0o600)
			tamper(t, p, `UPDATE investigations SET problem_statement = 'a' || char(0) || 'b'`)
		}, "investigations.problem_statement holds data outside Casebook limits"},
		{"foreign key violation", func(t *testing.T, p string) {
			os.WriteFile(p, goodBytes, 0o600)
			tamper(t, p, `INSERT INTO activity (investigation_id, actor_kind, actor_client, action, summary, created_at)
VALUES (999, 'human', 'x', 'x', 'x', 'x')`)
		}, "foreign key violation in activity"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "bad.db")
			tc.make(t, p)
			_, err := validateFile(t, p)
			if !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("err = %v, want ErrInvalidSnapshot", err)
			}
			reason := SnapshotReason(err)
			if tc.reason != "" && !strings.Contains(reason, tc.reason) {
				t.Fatalf("reason = %q, want it to contain %q", reason, tc.reason)
			}
			for _, bad := range []string{"SQLITE", "malformed", "file is not a database", "sql:"} {
				if strings.Contains(err.Error(), bad) {
					t.Fatalf("error leaks SQLite text: %v", err)
				}
			}
		})
	}
}

func TestValidateSnapshotLeavesFileUntouched(t *testing.T) {
	db := seededDB(t)
	snap := snapshotOf(t, db)
	before, err := os.ReadFile(snap)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := validateFile(t, snap); err != nil || v != LatestVersion() {
		t.Fatalf("validate = %d, %v", v, err)
	}
	after, err := os.ReadFile(snap)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("validation modified the snapshot")
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(snap + suffix); err == nil {
			t.Fatalf("validation left %s", filepath.Base(snap+suffix))
		}
	}
}

func TestSchemaFingerprintMatchesReference(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	got, err := SchemaFingerprint(ctx, db.reader)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ReferenceFingerprint(ctx, migrationsFS(), LatestVersion())
	if err != nil {
		t.Fatal(err)
	}
	if d := fingerprintDiff(got, want); d != "" {
		t.Fatalf("live schema differs from the reference: %s", d)
	}
	if len(got) < 10 {
		t.Fatalf("fingerprint has only %d objects", len(got))
	}
	for _, o := range got {
		if strings.Contains(o.SQL, "\n") || strings.Contains(o.SQL, "  ") {
			t.Fatalf("%s %s SQL whitespace not collapsed", o.Type, o.Name)
		}
		if strings.HasPrefix(o.Name, "sqlite_") {
			t.Fatalf("fingerprint includes internal object %s", o.Name)
		}
	}
	if _, err := ReferenceFingerprint(ctx, migrationsFS(), LatestVersion()+1); err == nil {
		t.Fatal("reference for a version beyond the migrations succeeded")
	}
}
