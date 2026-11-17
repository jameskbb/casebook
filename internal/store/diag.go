package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// OpenTokenConn takes one connection from the reader pool for the caller to
// keep for its lifetime (the change token reads data_version on it). The
// caller must Close it.
func (db *DB) OpenTokenConn(ctx context.Context) (*sql.Conn, error) {
	return db.reader.Conn(ctx)
}

// DataVersion runs PRAGMA data_version on c. Call it outside any transaction:
// the value then changes whenever another connection (in this or any other
// process) has committed since the previous call on c.
func DataVersion(ctx context.Context, c *sql.Conn) (int64, error) {
	var v int64
	err := c.QueryRowContext(ctx, `PRAGMA data_version`).Scan(&v)
	return v, err
}

// MaxActivityID returns the largest activity id, or 0 when there is none.
func MaxActivityID(ctx context.Context, q Querier) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), 0) FROM activity`).Scan(&id)
	return id, err
}

// SchemaVersion returns the schema version recorded in schema_migrations (0
// when the table does not exist).
func SchemaVersion(ctx context.Context, q Querier) (int, error) {
	return currentVersion(ctx, q)
}

// CountAll counts the rows of every record and link table and the activity
// log.
func CountAll(ctx context.Context, q Querier) (domain.Counts, error) {
	var c domain.Counts
	err := q.QueryRowContext(ctx, `SELECT
    (SELECT count(*) FROM investigations),
    (SELECT count(*) FROM evidence),
    (SELECT count(*) FROM claims),
    (SELECT count(*) FROM hypotheses),
    (SELECT count(*) FROM questions),
    (SELECT count(*) FROM claim_evidence),
    (SELECT count(*) FROM hypothesis_evidence),
    (SELECT count(*) FROM activity)`).Scan(
		&c.Investigations, &c.Evidence, &c.Claims, &c.Hypotheses, &c.Questions,
		&c.ClaimLinks, &c.HypothesisLinks, &c.Activity)
	return c, err
}

// IntegrityProblems runs PRAGMA integrity_check and PRAGMA foreign_key_check
// and returns one sanitized line per problem (nil when healthy). Lines never
// carry raw SQLite text or column values: an integrity_check line becomes
// "integrity_check: problem in table claims" (or index, naming the first
// schema object the raw line mentions) or "integrity_check: damaged database
// pages" when it names none; a foreign key line names tables and the row id
// only, e.g. "foreign key violation in claim_evidence (rowid 3) referencing
// claims".
func IntegrityProblems(ctx context.Context, q Querier) ([]string, error) {
	objects, err := schemaObjectTypes(ctx, q)
	if err != nil {
		return nil, err
	}
	var problems []string
	// Read the FTS5 index once first: a pooled connection that ran
	// integrity_check before another connection wrote keeps a stale FTS5
	// structure for the pragma and reports a false "checksum mismatch";
	// an ordinary query reloads it. Best effort: on a damaged index the
	// pragma below reports the problem.
	var refresh int
	_ = q.QueryRowContext(ctx, `SELECT count(*) FROM search_index WHERE search_index MATCH 'casebook'`).Scan(&refresh)
	rows, err := q.QueryContext(ctx, `PRAGMA integrity_check`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			rows.Close()
			return nil, err
		}
		if line != "ok" {
			problems = append(problems, sanitizeIntegrityLine(line, objects))
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = q.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			table, parent string
			rowid         sql.NullInt64
			fkid          int64
		)
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return nil, err
		}
		where := "(no rowid)"
		if rowid.Valid {
			where = fmt.Sprintf("(rowid %d)", rowid.Int64)
		}
		problems = append(problems, fmt.Sprintf("foreign key violation in %s %s referencing %s",
			safeName(table), where, safeName(parent)))
	}
	return problems, rows.Err()
}

// schemaObjectTypes maps the name of every table and index in the schema to
// its type.
func schemaObjectTypes(ctx context.Context, q Querier) (map[string]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT name, type FROM sqlite_schema WHERE type IN ('table', 'index')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return nil, err
		}
		if safeIdent.MatchString(name) {
			out[name] = typ
		}
	}
	return out, rows.Err()
}

// sanitizeIntegrityLine reduces a raw integrity_check line to the first
// table or index it names (identifier tokens checked against the schema),
// so no raw SQLite text or value ever leaves the store.
func sanitizeIntegrityLine(line string, objects map[string]string) string {
	isIdent := func(r rune) bool {
		return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
	}
	for _, tok := range strings.FieldsFunc(line, func(r rune) bool { return !isIdent(r) }) {
		if typ, ok := objects[tok]; ok {
			return "integrity_check: problem in " + typ + " " + tok
		}
	}
	return "integrity_check: damaged database pages"
}

// safeName returns name when it is a plain identifier, else "a table".
func safeName(name string) string {
	if safeIdent.MatchString(name) {
		return name
	}
	return "a table"
}
