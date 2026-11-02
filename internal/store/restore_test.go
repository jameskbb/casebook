package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jameskbb/casebook/internal/domain"
)

var testActor = domain.Actor{Kind: domain.ActorHuman, Client: "test"}

// seedRestoreDB writes one investigation with evidence, a claim, a link, an
// activity row and the active setting, using searchable words.
func seedRestoreDB(t *testing.T, db *DB, word string) {
	t.Helper()
	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		invID, err := insertTestInvestigation(ctx, tx, "Investigation "+word)
		if err != nil {
			return err
		}
		num, err := AllocateRecordNum(ctx, tx, invID, domain.RecordEvidence)
		if err != nil {
			return err
		}
		evID, err := InsertEvidence(ctx, tx, NewEvidence{
			InvestigationID: invID, RefNum: num, Title: "Evidence " + word, Summary: "summary " + word,
			SourceType: domain.SourceTypeFile, CapturedAt: "2026-10-01T00:00:00.000Z",
			CreatedAt: "2026-10-01T00:00:00.000Z", CreatedBy: testActor, Verification: domain.VerificationHumanVerified,
		})
		if err != nil {
			return err
		}
		cnum, err := AllocateRecordNum(ctx, tx, invID, domain.RecordClaim)
		if err != nil {
			return err
		}
		claimID, err := InsertStatement(ctx, tx, LinkClaim, NewStatement{
			InvestigationID: invID, RefNum: cnum, Statement: "claim " + word, Confidence: domain.ConfidenceHigh,
			Status: "active", Verification: domain.VerificationHumanVerified,
			CreatedAt: "2026-10-01T00:00:00.000Z", CreatedBy: testActor,
		})
		if err != nil {
			return err
		}
		if err := InsertImportedLink(ctx, tx, LinkClaim, claimID, evID, domain.RelationSupports,
			"2026-10-01T00:00:00.000Z", testActor); err != nil {
			return err
		}
		if _, err := InsertActivity(ctx, tx, NewActivity{InvestigationID: invID, Actor: testActor,
			Action: "evidence.add", Summary: "added " + word, CreatedAt: "2026-10-01T00:00:00.000Z"}); err != nil {
			return err
		}
		return SetActiveInvestigationID(ctx, tx, invID, "2026-10-01T00:00:00.000Z")
	})
}

func countsOf(t *testing.T, db *DB) domain.Counts {
	t.Helper()
	c, err := CountAll(context.Background(), db.reader)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRestoreColumnsCoverSchema(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	mapped := map[string]bool{}
	for _, rt := range restoreTables {
		mapped[rt.name] = true
		rows, err := db.reader.QueryContext(ctx, `SELECT name FROM pragma_table_info(?) ORDER BY cid`, rt.name)
		if err != nil {
			t.Fatal(err)
		}
		var cols []string
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				t.Fatal(err)
			}
			cols = append(cols, c)
		}
		rows.Close()
		if !reflect.DeepEqual(cols, rt.columns) {
			t.Errorf("table_info(%s) = %v\nrestore map     = %v", rt.name, cols, rt.columns)
		}
	}
	// Every data table must be in the map: a migration adding a table has
	// to update restoreTables.
	rows, err := db.reader.QueryContext(ctx, `SELECT name FROM sqlite_schema WHERE type = 'table'
AND substr(name, 1, 7) != 'sqlite_' AND name != 'schema_migrations'
AND name != 'search_index' AND substr(name, 1, 13) != 'search_index_'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		if !mapped[name] {
			t.Errorf("table %s is not in restoreTables", name)
		}
	}
}

func TestReplaceAllReplacesEveryRow(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	seedRestoreDB(t, db, "alpha")
	snap := snapshotOf(t, db)
	want := countsOf(t, db)

	seedRestoreDB(t, db, "omega")
	seedRestoreDB(t, db, "omega")
	if countsOf(t, db) == want {
		t.Fatal("mutation did not change counts")
	}

	var afterCalls int
	err := db.ReplaceAll(ctx, snap, func(tx *sql.Tx) error {
		afterCalls++
		_, err := InsertActivity(ctx, tx, NewActivity{Actor: testActor, Action: "restore", Summary: "restored",
			CreatedAt: "2026-10-02T00:00:00.000Z"})
		return err
	})
	if err != nil {
		t.Fatalf("ReplaceAll: %v", err)
	}
	if afterCalls != 1 {
		t.Fatalf("after ran %d times", afterCalls)
	}
	got := countsOf(t, db)
	want.Activity++
	if got != want {
		t.Fatalf("counts = %+v, want %+v", got, want)
	}
	if n := ftsCount(t, db, "alpha"); n == 0 {
		t.Fatal("search index lost restored text")
	}
	if n := ftsCount(t, db, "omega"); n != 0 {
		t.Fatalf("search index still has %d rows of replaced text", n)
	}
	var indexed, records int
	if err := db.reader.QueryRow(`SELECT count(*) FROM search_index`).Scan(&indexed); err != nil {
		t.Fatal(err)
	}
	records = got.Investigations + got.Evidence + got.Claims + got.Hypotheses + got.Questions
	if indexed != records {
		t.Fatalf("search_index rows = %d, records = %d", indexed, records)
	}
	if id, ok, err := GetActiveInvestigationID(ctx, db.reader); err != nil || !ok || id != 1 {
		t.Fatalf("active = %d %v %v; want the restored setting", id, ok, err)
	}
	problems, err := IntegrityProblems(ctx, db.reader)
	if err != nil || len(problems) != 0 {
		t.Fatalf("integrity after restore: %v %v", problems, err)
	}
	assertDetached(t, db)
	// The database still accepts normal writes on the writer connection.
	seedRestoreDB(t, db, "later")
}

// assertDetached fails when restore_src is still attached to the writer.
func assertDetached(t *testing.T, db *DB) {
	t.Helper()
	rows, err := db.writer.Query(`PRAGMA database_list`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			seq        int
			name, file string
		)
		if err := rows.Scan(&seq, &name, &file); err != nil {
			t.Fatal(err)
		}
		if name == "restore_src" {
			t.Fatal("restore_src is still attached")
		}
	}
}

func TestReplaceAllRollsBackMidway(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	seedRestoreDB(t, db, "alpha")
	snap := snapshotOf(t, db)
	seedRestoreDB(t, db, "omega")
	before := countsOf(t, db)
	beforeFTS := ftsCount(t, db, "omega")

	boom := errors.New("forced failure")
	var stages []string
	replaceHook = func(stage string) error {
		stages = append(stages, stage)
		if stage == "inserted:evidence" {
			return boom
		}
		return nil
	}
	t.Cleanup(func() { replaceHook = nil })
	afterRan := false
	err := db.ReplaceAll(ctx, snap, func(*sql.Tx) error { afterRan = true; return nil })
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the forced failure", err)
	}
	if afterRan {
		t.Fatal("after ran despite the failure")
	}
	if strings.Join(stages, ",") != "deleted,inserted:investigations,inserted:counters,inserted:settings,inserted:evidence" {
		t.Fatalf("stages = %v", stages)
	}
	if got := countsOf(t, db); got != before {
		t.Fatalf("counts after rollback = %+v, want %+v", got, before)
	}
	if n := ftsCount(t, db, "omega"); n != beforeFTS {
		t.Fatalf("search index after rollback = %d rows, want %d", n, beforeFTS)
	}
	assertDetached(t, db)

	// An error from after rolls back too.
	replaceHook = nil
	err = db.ReplaceAll(ctx, snap, func(*sql.Tx) error { return boom })
	if !errors.Is(err, boom) || countsOf(t, db) != before {
		t.Fatalf("after failure: err = %v, counts = %+v", err, countsOf(t, db))
	}
}

func TestRebuildSearchIndexMatchesTriggers(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	seedRestoreDB(t, db, "alpha")
	snapshot := func() [][]any {
		rows, err := db.reader.QueryContext(ctx, `SELECT rowid, title, body, record_type, investigation_id, record_id
FROM search_index ORDER BY rowid`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out [][]any
		for rows.Next() {
			var (
				rowid, inv, rec int64
				title, body, rt string
			)
			if err := rows.Scan(&rowid, &title, &body, &rt, &inv, &rec); err != nil {
				t.Fatal(err)
			}
			out = append(out, []any{rowid, title, body, rt, inv, rec})
		}
		return out
	}
	byTriggers := snapshot()
	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error { return RebuildSearchIndex(ctx, tx) })
	if rebuilt := snapshot(); !reflect.DeepEqual(rebuilt, byTriggers) {
		t.Fatalf("rebuilt index differs from trigger-maintained index:\n%v\n%v", rebuilt, byTriggers)
	}
}

func TestMigrateFileUpgradesToSyntheticVersion(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	seedRestoreDB(t, db, "alpha")
	snap := snapshotOf(t, db)

	sub, err := fs.Sub(Migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	testFS := fstest.MapFS{}
	entries, err := fs.ReadDir(sub, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := fs.ReadFile(sub, e.Name())
		if err != nil {
			t.Fatal(err)
		}
		testFS[e.Name()] = &fstest.MapFile{Data: b}
	}
	next := LatestVersion() + 1
	name := fmt.Sprintf("%04d_synthetic.sql", next)
	testFS[name] = &fstest.MapFile{Data: []byte(
		`CREATE TABLE synthetic_notes (id INTEGER PRIMARY KEY, body TEXT NOT NULL) STRICT;
CREATE INDEX evidence_source_type ON evidence (source_type);`)}

	from, to, err := MigrateFile(ctx, snap, testFS)
	if err != nil {
		t.Fatalf("MigrateFile: %v", err)
	}
	if from != LatestVersion() || to != next {
		t.Fatalf("MigrateFile = %d -> %d, want %d -> %d", from, to, LatestVersion(), next)
	}
	ro, err := OpenReadOnly(snap)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	got, err := SchemaFingerprint(ctx, ro)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ReferenceFingerprint(ctx, testFS, next)
	if err != nil {
		t.Fatal(err)
	}
	if d := fingerprintDiff(got, want); d != "" {
		t.Fatalf("migrated file differs from the version %d reference: %s", next, d)
	}
	var mode string
	if err := ro.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "delete" {
		t.Fatalf("journal_mode = %q, %v", mode, err)
	}
	var title string
	if err := ro.QueryRow(`SELECT title FROM investigations`).Scan(&title); err != nil || title != "Investigation alpha" {
		t.Fatalf("seeded row = %q, %v", title, err)
	}
	// Against the embedded migrations the upgraded file is too new.
	if _, err := validateFile(t, snap); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("embedded validation of a version %d file = %v", next, err)
	}
	// The same file at the current version needs no migration.
	snap2 := snapshotOf(t, db)
	if from, to, err := MigrateFile(ctx, snap2, nil); err != nil || from != LatestVersion() || to != LatestVersion() {
		t.Fatalf("MigrateFile(current) = %d, %d, %v", from, to, err)
	}
}
