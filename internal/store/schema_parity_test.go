package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/domain"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var errRollback = errors.New("rollback test transaction")

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "casebook.db"), Config{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func strs[T ~string](vs []T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

func isConstraintErr(err error) bool {
	var se *sqlite.Error
	return errors.As(err, &se) && se.Code()&0xff == sqlite3.SQLITE_CONSTRAINT
}

// parityColumn is one CHECKed enum column and the Go values it must accept.
type parityColumn struct {
	table, column string
	values        []string
}

func parityColumns() []parityColumn {
	actors := strs(domain.AllActorKinds())
	var preArchive []string
	for _, s := range domain.AllInvestigationStatuses() {
		if s != domain.InvestigationStatusArchived {
			preArchive = append(preArchive, string(s))
		}
	}
	return []parityColumn{
		{"investigations", "status", strs(domain.AllInvestigationStatuses())},
		{"investigations", "pre_archive_status", preArchive},
		{"investigations", "conclusion_confidence", strs(domain.AllConfidences())},
		{"investigations", "created_by_kind", actors},
		{"evidence", "source_type", strs(domain.AllSourceTypes())},
		{"evidence", "verification", strs(domain.AllVerifications())},
		{"evidence", "created_by_kind", actors},
		{"claims", "confidence", strs(domain.AllConfidences())},
		{"claims", "status", strs(domain.AllClaimStatuses())},
		{"claims", "verification", strs(domain.AllVerifications())},
		{"claims", "created_by_kind", actors},
		{"hypotheses", "confidence", strs(domain.AllConfidences())},
		{"hypotheses", "status", strs(domain.AllHypothesisStatuses())},
		{"hypotheses", "verification", strs(domain.AllVerifications())},
		{"hypotheses", "created_by_kind", actors},
		{"questions", "status", strs(domain.AllQuestionStatuses())},
		{"questions", "created_by_kind", actors},
		{"claim_evidence", "relation", strs(domain.AllRelations())},
		{"claim_evidence", "created_by_kind", actors},
		{"hypothesis_evidence", "relation", strs(domain.AllRelations())},
		{"hypothesis_evidence", "created_by_kind", actors},
		{"activity", "actor_kind", actors},
	}
}

// baseRow returns valid column values for one row of table; seq keeps unique
// keys unique. Parent ids refer to rows created by seedParents.
func baseRow(table string, seq int) map[string]any {
	const ts = "2026-10-01T00:00:00.000Z"
	switch table {
	case "investigations":
		return map[string]any{"ref_num": 1000 + seq, "title": "t", "status": "new", "created_at": ts, "updated_at": ts,
			"created_by_kind": "human", "created_by_client": "test"}
	case "evidence":
		return map[string]any{"investigation_id": 1, "ref_num": 100 + seq, "title": "t", "summary": "s",
			"source_type": "code", "captured_at": ts, "created_at": ts, "updated_at": ts,
			"created_by_kind": "human", "created_by_client": "test", "verification": "human_verified"}
	case "claims":
		return map[string]any{"investigation_id": 1, "ref_num": 100 + seq, "statement": "s", "verification": "human_verified",
			"created_at": ts, "updated_at": ts, "created_by_kind": "human", "created_by_client": "test"}
	case "hypotheses":
		return map[string]any{"investigation_id": 1, "ref_num": 100 + seq, "statement": "s", "verification": "human_verified",
			"created_at": ts, "updated_at": ts, "created_by_kind": "human", "created_by_client": "test"}
	case "questions":
		return map[string]any{"investigation_id": 1, "ref_num": 100 + seq, "question": "q",
			"created_at": ts, "updated_at": ts, "created_by_kind": "human", "created_by_client": "test"}
	case "claim_evidence":
		return map[string]any{"claim_id": 1, "evidence_id": 1, "relation": "supports", "created_at": ts,
			"created_by_kind": "human", "created_by_client": "test"}
	case "hypothesis_evidence":
		return map[string]any{"hypothesis_id": 1, "evidence_id": 1, "relation": "supports", "created_at": ts,
			"created_by_kind": "human", "created_by_client": "test"}
	case "activity":
		return map[string]any{"investigation_id": 1, "actor_kind": "human", "actor_client": "test", "action": "x",
			"summary": "s", "created_at": ts}
	}
	panic("unknown table " + table)
}

func insertRow(ctx context.Context, q Querier, table string, row map[string]any) error {
	cols := make([]string, 0, len(row))
	for c := range row {
		cols = append(cols, c)
	}
	sort.Strings(cols)
	args := make([]any, len(cols))
	for i, c := range cols {
		args[i] = row[c]
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "),
		strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", "))
	_, err := q.ExecContext(ctx, query, args...)
	return err
}

func seedParents(ctx context.Context, tx *sql.Tx) error {
	for _, table := range []string{"investigations", "evidence", "claims", "hypotheses"} {
		row := baseRow(table, 0)
		row["id"] = 1
		if err := insertRow(ctx, tx, table, row); err != nil {
			return fmt.Errorf("seed %s: %w", table, err)
		}
	}
	return nil
}

// schemaCheckValues extracts the IN (...) list of column's CHECK constraint
// from table's CREATE statement.
func schemaCheckValues(t *testing.T, db *DB, table, column string) []string {
	t.Helper()
	var ddl string
	if err := db.reader.QueryRow(`SELECT sql FROM sqlite_schema WHERE type = 'table' AND name = ?`, table).Scan(&ddl); err != nil {
		t.Fatalf("read %s DDL: %v", table, err)
	}
	re := regexp.MustCompile(`[(\s]` + regexp.QuoteMeta(column) + `\s+IN\s*\(([^)]*)\)`)
	m := re.FindStringSubmatch(ddl)
	if m == nil {
		t.Fatalf("no CHECK (%s IN (...)) in %s DDL", column, table)
	}
	var out []string
	for _, part := range strings.Split(m[1], ",") {
		out = append(out, strings.Trim(strings.TrimSpace(part), "'"))
	}
	return out
}

// TestEnumSchemaParity keeps the D-09 Go enums and the 0001 CHECK lists in
// lockstep: every Go value inserts, an invalid value is a constraint error, and
// the CHECK list equals the Go list exactly.
func TestEnumSchemaParity(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	for _, pc := range parityColumns() {
		t.Run(pc.table+"."+pc.column, func(t *testing.T) {
			if len(pc.values) == 0 {
				t.Fatal("Go enum has no values")
			}
			want := append([]string(nil), pc.values...)
			got := schemaCheckValues(t, db, pc.table, pc.column)
			sort.Strings(want)
			sort.Strings(got)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("schema CHECK values %v != Go enum values %v", got, want)
			}

			err := db.WithWriteTx(ctx, func(tx *sql.Tx) error {
				if err := seedParents(ctx, tx); err != nil {
					return err
				}
				isLink := pc.table == "claim_evidence" || pc.table == "hypothesis_evidence"
				for i, v := range pc.values {
					row := baseRow(pc.table, i+1)
					row[pc.column] = v
					pairStatusVerification(pc.table, pc.column, v, row)
					if err := insertRow(ctx, tx, pc.table, row); err != nil {
						return fmt.Errorf("insert %s=%q: %w", pc.column, v, err)
					}
					if isLink {
						if _, err := tx.ExecContext(ctx, "DELETE FROM "+pc.table); err != nil {
							return err
						}
					}
				}
				row := baseRow(pc.table, 999)
				row[pc.column] = "bogus"
				err := insertRow(ctx, tx, pc.table, row)
				if err == nil {
					return fmt.Errorf("insert %s=bogus succeeded", pc.column)
				}
				if !isConstraintErr(err) {
					return fmt.Errorf("insert %s=bogus: want constraint error, got %v", pc.column, err)
				}
				return errRollback
			})
			if !errors.Is(err, errRollback) {
				t.Fatal(err)
			}
		})
	}

	// The rolled-back transactions left nothing behind.
	var n int
	if err := db.reader.QueryRow(`SELECT count(*) FROM investigations`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("investigations after rollback = %d, %v", n, err)
	}
}

// pairStatusVerification sets the other half of a claim's or hypothesis's
// status and verification so the row satisfies the migration 0002 triggers
// (ADR-0013) while the column under test takes value v.
func pairStatusVerification(table, column, v string, row map[string]any) {
	if table != "claims" && table != "hypotheses" {
		return
	}
	paired := v == "superseded" || (table == "claims" && v == "disputed")
	switch column {
	case "status":
		if paired {
			row["verification"] = v
		}
	case "verification":
		if paired {
			row["status"] = v
		}
	}
}
