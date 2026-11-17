package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Migration 0006 widens the CHECK list of evidence.source_type by one value
// ('evernote_note'). SQLite cannot alter a CHECK constraint in place, and a
// table rebuild would cascade through the link tables and the search
// triggers, so this follows SQLite's documented procedure for loosening a
// constraint (https://www.sqlite.org/lang_altertable.html, "Making Other
// Kinds Of Table Schema Changes"): rewrite the table's CREATE statement in
// sqlite_schema under writable_schema, bump schema_version so every
// connection reloads the schema, and check integrity before the
// transaction commits. Every existing row already satisfies the wider list.

// sourceTypeCheckOld and sourceTypeCheckNew are the tail of the
// evidence.source_type CHECK list in 0001_init.sql and after 0006.
const (
	sourceTypeCheckOld = `'casebook_artifact'))`
	sourceTypeCheckNew = `'casebook_artifact', 'evernote_note'))`
)

func widenSourceTypeCheck(ctx context.Context, tx *sql.Tx) error {
	var ddl string
	if err := tx.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_schema WHERE type = 'table' AND name = 'evidence'`).Scan(&ddl); err != nil {
		return fmt.Errorf("read evidence table: %w", err)
	}
	if strings.Count(ddl, sourceTypeCheckOld) != 1 {
		return fmt.Errorf("widen evidence.source_type: unexpected evidence table definition")
	}
	newDDL := strings.Replace(ddl, sourceTypeCheckOld, sourceTypeCheckNew, 1)

	var version int64
	if err := tx.QueryRowContext(ctx, `PRAGMA schema_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `PRAGMA writable_schema = ON`); err != nil {
		return fmt.Errorf("widen evidence.source_type: %w", err)
	}
	_, uerr := tx.ExecContext(ctx,
		`UPDATE sqlite_schema SET sql = ? WHERE type = 'table' AND name = 'evidence'`, newDDL)
	if uerr == nil {
		_, uerr = tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA schema_version = %d`, version+1))
	}
	// Always turn writable_schema off again, even after a failure.
	if _, err := tx.ExecContext(ctx, `PRAGMA writable_schema = OFF`); err != nil && uerr == nil {
		uerr = err
	}
	if uerr != nil {
		return fmt.Errorf("widen evidence.source_type: %w", uerr)
	}
	var result string
	if err := tx.QueryRowContext(ctx, `PRAGMA integrity_check(1)`).Scan(&result); err != nil {
		return fmt.Errorf("integrity check after widening evidence.source_type: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check after widening evidence.source_type failed")
	}
	return nil
}
