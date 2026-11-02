package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"
)

// Restore support (D-22): a validated snapshot copy is migrated forward in
// its own file, then its rows replace every live row inside one BEGIN
// IMMEDIATE transaction on the live database. Other connections and
// processes keep the same file open throughout and only ever see the old or
// the new contents, never a half-restored or swapped file.

// restoreTable is one table copied by ReplaceAll with its full column list.
type restoreTable struct {
	name    string
	columns []string
}

// restoreTables lists every Casebook data table in parent-first order with a
// static column list (never column names read from the source file). A test
// compares it with PRAGMA table_info so a migration that adds a column or
// table must update it. schema_migrations and the search index are not
// copied: the live schema stays, and the index is rebuilt.
var restoreTables = []restoreTable{
	{"investigations", []string{"id", "ref_num", "title", "problem_statement", "status", "pre_archive_status",
		"conclusion", "conclusion_confidence", "conclusion_rationale", "created_at", "updated_at", "archived_at",
		"created_by_kind", "created_by_client", "last_evidence_num", "last_claim_num", "last_hypothesis_num",
		"last_question_num", "conclusion_verification", "conclusion_supporting_claims", "conclusion_contradicting_claims", "conclusion_reviewed_at", "conclusion_review_basis", "conclusion_review_snapshot"}},
	{"counters", []string{"name", "value"}},
	{"settings", []string{"key", "value", "updated_at"}},
	{"evidence", []string{"id", "investigation_id", "ref_num", "title", "summary", "raw_detail", "source_type",
		"source_ref", "source_label", "captured_at", "created_at", "updated_at", "created_by_kind",
		"created_by_client", "verification", "metadata", "imported"}},
	{"claims", []string{"id", "investigation_id", "ref_num", "statement", "confidence", "confidence_rationale",
		"status", "verification", "superseded_by_id", "created_at", "updated_at", "created_by_kind",
		"created_by_client", "imported"}},
	{"hypotheses", []string{"id", "investigation_id", "ref_num", "statement", "confidence", "rationale", "status",
		"verification", "superseded_by_id", "created_at", "updated_at", "created_by_kind", "created_by_client",
		"imported"}},
	{"questions", []string{"id", "investigation_id", "ref_num", "question", "status", "resolution", "defer_note",
		"resolved_at", "created_at", "updated_at", "created_by_kind", "created_by_client", "imported"}},
	{"claim_evidence", []string{"claim_id", "evidence_id", "relation", "created_at", "created_by_kind",
		"created_by_client"}},
	{"hypothesis_evidence", []string{"hypothesis_id", "evidence_id", "relation", "created_at", "created_by_kind",
		"created_by_client"}},
	{"activity", []string{"id", "investigation_id", "actor_kind", "actor_client", "action", "target_ref",
		"summary", "note", "created_at", "imported"}},
	{"directory_investigations", []string{"dir", "investigation_id", "updated_at"}},
	// evernote_links (migration 0006) is dormant: nothing reads or writes
	// it since the Evernote integration was removed, but it stays in the
	// schema, so backups and restores keep copying it and an existing
	// database restores without losing rows. The stable export never
	// included it.
	{"evernote_links", []string{"investigation_id", "note_guid", "note_title", "block_hash", "note_usn",
		"posted_at"}},
}

// replaceHook, when set by a test, runs at each ReplaceAll stage ("deleted",
// then "inserted:<table>" after each table) inside the transaction; an error
// aborts and rolls back the restore.
var replaceHook func(stage string) error

// MigrateFile migrates the database file at path (a private copy of a
// validated snapshot) forward with the migrations in fsys (root holds
// NNNN_name.sql files; nil means the embedded migrations) and validates the
// result against the reference schema at the resulting version. The file
// stays a single file in rollback (delete) journal mode. It returns the
// version before and after.
func MigrateFile(ctx context.Context, path string, fsys fs.FS) (int, int, error) {
	db, err := sql.Open("sqlite", fileURI(path)+
		"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(DELETE)&_pragma=trusted_schema(0)")
	if err != nil {
		return 0, 0, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	from, err := currentVersion(ctx, db)
	if err != nil {
		return 0, 0, readFailure(ctx, err)
	}
	mfs := fsys
	if mfs == nil {
		mfs = migrationsFS()
	}
	to, err := Migrate(ctx, db, mfs, time.Now)
	if err != nil {
		return from, 0, err
	}
	if _, _, err := validateSnapshot(ctx, db, fsys, to); err != nil {
		return from, to, err
	}
	if err := db.Close(); err != nil {
		return from, to, err
	}
	return from, to, nil
}

// ReplaceAll replaces every row of every Casebook table with the rows of the
// snapshot at srcPath, which must already be migrated to this database's
// schema version and validated (MigrateFile). On one dedicated writer
// connection it ATTACHes the snapshot read-only, then in one BEGIN IMMEDIATE
// transaction (retried on lock contention like WithWriteTx) deletes every
// live row children first, copies every table parent first with INSERT ...
// SELECT over the static column lists, rebuilds the search index, runs after
// (for the restore activity row) and commits. Any failure rolls everything
// back. The snapshot is DETACHed afterwards. after must not do file or
// network I/O and may run more than once.
func (db *DB) ReplaceAll(ctx context.Context, srcPath string, after func(*sql.Tx) error) error {
	conn, err := db.writer.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS restore_src`, fileURI(srcPath)+"?mode=ro"); err != nil {
		return err
	}
	// ATTACH survives ROLLBACK, so always detach explicitly.
	defer conn.ExecContext(context.Background(), `DETACH DATABASE restore_src`)

	return db.retryBusy(ctx, "restore", func() error {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := replaceRows(ctx, tx); err != nil {
			return errors.Join(err, rollback(tx))
		}
		if after != nil {
			if err := after(tx); err != nil {
				return errors.Join(err, rollback(tx))
			}
		}
		if err := tx.Commit(); err != nil {
			return errors.Join(err, rollback(tx))
		}
		return nil
	})
}

// replaceRows deletes every live row and copies the attached snapshot's rows.
func replaceRows(ctx context.Context, tx *sql.Tx) error {
	for i := len(restoreTables) - 1; i >= 0; i-- {
		// Table names come from the static restoreTables list.
		if _, err := tx.ExecContext(ctx, `DELETE FROM main.`+restoreTables[i].name); err != nil {
			return fmt.Errorf("clear %s: %w", restoreTables[i].name, err)
		}
	}
	if err := runReplaceHook("deleted"); err != nil {
		return err
	}
	for _, t := range restoreTables {
		cols := strings.Join(t.columns, ", ")
		if _, err := tx.ExecContext(ctx, `INSERT INTO main.`+t.name+` (`+cols+`) SELECT `+cols+
			` FROM restore_src.`+t.name); err != nil {
			return fmt.Errorf("copy %s: %w", t.name, err)
		}
		if err := runReplaceHook("inserted:" + t.name); err != nil {
			return err
		}
	}
	if err := repairCounters(ctx, tx); err != nil {
		return err
	}
	return RebuildSearchIndex(ctx, tx)
}

// repairCounters raises the global investigation counter and every
// per-investigation record counter to at least the highest ref number in
// use, so a restored backup whose counters lag behind its refs (crafted or
// damaged) can never make the next allocation collide with an existing ref.
// Counters only ever go up; validation already refused values above
// domain.MaxRefNum.
func repairCounters(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`INSERT INTO main.counters (name, value)
SELECT 'investigation', COALESCE(MAX(ref_num), 0) FROM main.investigations WHERE true
ON CONFLICT (name) DO UPDATE SET value = max(value, excluded.value)`,
		`UPDATE main.investigations SET
  last_evidence_num = max(last_evidence_num,
    (SELECT COALESCE(MAX(ref_num), 0) FROM main.evidence r WHERE r.investigation_id = investigations.id)),
  last_claim_num = max(last_claim_num,
    (SELECT COALESCE(MAX(ref_num), 0) FROM main.claims r WHERE r.investigation_id = investigations.id)),
  last_hypothesis_num = max(last_hypothesis_num,
    (SELECT COALESCE(MAX(ref_num), 0) FROM main.hypotheses r WHERE r.investigation_id = investigations.id)),
  last_question_num = max(last_question_num,
    (SELECT COALESCE(MAX(ref_num), 0) FROM main.questions r WHERE r.investigation_id = investigations.id))`,
	}
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("repair counters: %w", err)
		}
	}
	return nil
}

func runReplaceHook(stage string) error {
	if replaceHook == nil {
		return nil
	}
	return replaceHook(stage)
}

// RebuildSearchIndex empties the full-text index and repopulates it from the
// five record tables, composing title and body and the rowid scheme exactly
// like the 0001 triggers (rowid = id * 8 + type code). Evidence raw_detail is
// never indexed.
func RebuildSearchIndex(ctx context.Context, q Querier) error {
	stmts := []string{
		`DELETE FROM search_index`,
		`INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
SELECT id * 8 + 1, title, problem_statement || char(10) || conclusion, 'investigation', id, id FROM investigations`,
		`INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
SELECT id * 8 + 2, title, summary, 'evidence', investigation_id, id FROM evidence`,
		`INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
SELECT id * 8 + 3, '', statement, 'claim', investigation_id, id FROM claims`,
		`INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
SELECT id * 8 + 4, '', statement, 'hypothesis', investigation_id, id FROM hypotheses`,
		`INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
SELECT id * 8 + 5, '', question || char(10) || resolution, 'question', investigation_id, id FROM questions`,
	}
	for _, s := range stmts {
		if _, err := q.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("rebuild search index: %w", err)
		}
	}
	return nil
}
