package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// Placeholders resanitizeText stores when a value that must not be empty
// held nothing but characters sanitizing drops.
const (
	// resanitizedTextPlaceholder replaces a required text field (a title,
	// summary, statement or question).
	resanitizedTextPlaceholder = "(invisible text removed)"
	// resanitizedClientPlaceholder replaces an author or actor client name.
	resanitizedClientPlaceholder = "unknown"
)

// sanitizeKind says which write-path rule a stored column follows.
type sanitizeKind int

const (
	kindText   sanitizeKind = iota // domain.SanitizeText
	kindLine                       // domain.SanitizeLine
	kindClient                     // domain.SanitizeLine, never empty
)

// textColumn is one stored free-text column and the rule its writes use.
type textColumn struct {
	name     string
	kind     sanitizeKind
	required bool
}

// resanitizeTables lists every free-text column Casebook stores, with the
// sanitizer internal/app applies to it on write. Enum, timestamp, ref and
// metadata columns are not free text (metadata is JSON that
// domain.NormalizeMetadata validates without stripping) and are left alone.
var resanitizeTables = []struct {
	table   string
	columns []textColumn
}{
	{"investigations", []textColumn{
		{"title", kindLine, true},
		{"problem_statement", kindText, false},
		{"conclusion", kindText, false},
		{"conclusion_rationale", kindText, false},
		{"created_by_client", kindClient, true},
	}},
	{"evidence", []textColumn{
		{"title", kindLine, true},
		{"summary", kindText, true},
		{"raw_detail", kindText, false},
		{"source_ref", kindLine, false},
		{"source_label", kindLine, false},
		{"created_by_client", kindClient, true},
	}},
	{"claims", []textColumn{
		{"statement", kindText, true},
		{"confidence_rationale", kindText, false},
		{"created_by_client", kindClient, true},
	}},
	{"hypotheses", []textColumn{
		{"statement", kindText, true},
		{"rationale", kindText, false},
		{"created_by_client", kindClient, true},
	}},
	{"questions", []textColumn{
		{"question", kindText, true},
		{"resolution", kindText, false},
		{"defer_note", kindText, false},
		{"created_by_client", kindClient, true},
	}},
	{"claim_evidence", []textColumn{
		{"created_by_client", kindClient, true},
	}},
	{"hypothesis_evidence", []textColumn{
		{"created_by_client", kindClient, true},
	}},
	{"activity", []textColumn{
		{"actor_client", kindClient, true},
		{"target_ref", kindLine, false},
		{"summary", kindLine, false},
		{"note", kindText, false},
	}},
}

// resanitize cleans one stored value with the same domain function its
// writes use. It never fails: invalid UTF-8 (which no write path stores) is
// dropped first, no byte limit applies (sanitizing only shrinks a value),
// and a required value left empty gets a placeholder instead of an error.
func (c textColumn) resanitize(s string) string {
	v := strings.ToValidUTF8(s, "")
	var err error
	switch c.kind {
	case kindText:
		v, err = domain.SanitizeText(c.name, v, 0, false)
	default:
		v, err = domain.SanitizeLine(c.name, v, 0, false)
	}
	if err != nil {
		// Unreachable for valid UTF-8 with no limit and required false;
		// keep the stored value rather than lose it.
		return s
	}
	if v == "" && c.required {
		if c.kind == kindClient {
			return resanitizedClientPlaceholder
		}
		return resanitizedTextPlaceholder
	}
	return v
}

// resanitizeText is the Go step of migration 0004. It re-applies today's
// write-path sanitizing to every stored free-text column, so rows written
// before stripControls dropped C1 controls, invisible format characters and
// noncharacters lose them too. Only values that change are updated; nothing
// else in the row is touched (updated_at, status, verification and imported
// keep their values) and no activity row is written, because the record says
// what it said before minus characters nobody could see. The search index
// follows through the existing AFTER UPDATE triggers. Running it again finds
// nothing to change.
func resanitizeText(ctx context.Context, tx *sql.Tx) error {
	for _, t := range resanitizeTables {
		if err := resanitizeTable(ctx, tx, t.table, t.columns); err != nil {
			return fmt.Errorf("re-sanitize %s: %w", t.table, err)
		}
	}
	return nil
}

// resanitizeTable updates the rows of table whose columns change when
// re-sanitized. Changes are collected before any update so no statement
// runs while the scan is open.
func resanitizeTable(ctx context.Context, tx *sql.Tx, table string, cols []textColumn) error {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.name
	}
	rows, err := tx.QueryContext(ctx, "SELECT rowid, "+strings.Join(names, ", ")+" FROM "+table+" ORDER BY rowid")
	if err != nil {
		return err
	}
	type change struct {
		rowid int64
		set   []string
		args  []any
	}
	var changes []change
	vals := make([]string, len(cols))
	dest := make([]any, len(cols)+1)
	for rows.Next() {
		var rowid int64
		dest[0] = &rowid
		for i := range vals {
			dest[i+1] = &vals[i]
		}
		if err := rows.Scan(dest...); err != nil {
			rows.Close()
			return err
		}
		var ch change
		for i, c := range cols {
			if v := c.resanitize(vals[i]); v != vals[i] {
				ch.set = append(ch.set, c.name+" = ?")
				ch.args = append(ch.args, v)
			}
		}
		if len(ch.set) > 0 {
			ch.rowid = rowid
			changes = append(changes, ch)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, ch := range changes {
		q := "UPDATE " + table + " SET " + strings.Join(ch.set, ", ") + " WHERE rowid = ?"
		if _, err := tx.ExecContext(ctx, q, append(ch.args, ch.rowid)...); err != nil {
			return err
		}
	}
	return nil
}
