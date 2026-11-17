package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Characters migration 0004 must drop (they reached the database before
// stripControls covered them) and ones it must keep.
const (
	zwsp      = "\u200b"     // zero-width space (Cf)
	rlo       = "\u202e"     // right-to-left override (Cf)
	lri       = "\u2066"     // left-to-right isolate (Cf)
	bom       = "\ufeff"     // byte order mark (Cf)
	nel       = "\u0085"     // next line (C1)
	nonchar   = "\uffff"     // noncharacter
	tagA      = "\U000E0041" // tag latin capital letter A, outside a flag
	zwj       = "\u200d"     // zero width joiner: kept
	lrm       = "\u200e"     // left-to-right mark: kept
	englandFl = "\U0001F3F4\U000E0067\U000E0062\U000E0065\U000E006E\U000E0067\U000E007F"
)

// dumpTable renders every row of table, all columns, in rowid order.
func dumpTable(t *testing.T, q Querier, table string) []string {
	t.Helper()
	rows, err := q.QueryContext(context.Background(), "SELECT rowid, * FROM "+table+" ORDER BY rowid")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for i, c := range cols {
			fmt.Fprintf(&b, "%s=%q ", c, fmt.Sprint(vals[i]))
		}
		out = append(out, b.String())
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func dumpAll(t *testing.T, q Querier) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, tb := range resanitizeTables {
		out[tb.table] = dumpTable(t, q, tb.table)
	}
	out["search_index"] = dumpTable(t, q, "search_index")
	return out
}

// TestMigration0004ResanitizesText seeds rows under schema 3 that hold
// characters today's sanitizing drops, migrates to 4 and checks that every
// free-text column is cleaned with its write-path rule, that clean rows are
// byte for byte unchanged, that updated_at, status, verification and
// imported are kept, that no activity is written, that the search index
// follows, and that running the step again changes nothing.
func TestMigration0004ResanitizesText(t *testing.T) {
	ctx := context.Background()
	w := rawDB(t, filepath.Join(t.TempDir(), "casebook.db"))
	if v, err := Migrate(ctx, w, migrationsUpTo(t, 3), nil); err != nil || v != 3 {
		t.Fatalf("migrate to 3 = %d, %v", v, err)
	}
	const updated = "2026-01-02T03:04:05.000Z"
	tx, err := w.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := seedParents(ctx, tx); err != nil {
		t.Fatal(err)
	}
	// Clean rows whose kept format characters (ZWJ, LRM, a subdivision
	// flag) and layout (LF and tab in multi-line text) must survive.
	cleanInv := baseRow("investigations", 9)
	cleanInv["id"] = 9
	cleanInv["title"] = "Family " + "\U0001F468" + zwj + "\U0001F469" + " in " + englandFl
	cleanInv["problem_statement"] = "line one\n\tindented " + lrm + "\u05e9\u05dc\u05d5\u05dd"
	cleanEv := baseRow("evidence", 9)
	cleanEv["id"], cleanEv["investigation_id"] = 9, 9
	cleanEv["raw_detail"] = "col1\tcol2\nv1\tv2"
	inv := baseRow("investigations", 2)
	inv["id"] = 2
	inv["title"] = "Pay" + zwsp + "roll " + rlo + "outage" + bom
	inv["problem_statement"] = "first" + nel + "second\r\nthird " + lri + "x"
	inv["conclusion"] = bom + "concluded" + nonchar
	inv["conclusion_rationale"] = "because" + tagA
	inv["created_by_client"] = "cli" + zwsp + "ent"
	inv["status"], inv["updated_at"] = "blocked", updated
	ev := baseRow("evidence", 2)
	ev["id"], ev["investigation_id"] = 2, 2
	ev["title"] = "Log" + zwsp + "line"
	ev["summary"] = "  summary" + rlo + " text  "
	ev["raw_detail"] = "raw\r\nline" + zwsp + "\n"
	ev["source_ref"] = "src" + bom + "/ref"
	ev["source_label"] = "lab" + nel + "el"
	ev["verification"], ev["updated_at"], ev["imported"] = "source_verified", updated, 1
	evEmpty := baseRow("evidence", 3)
	evEmpty["id"], evEmpty["investigation_id"] = 3, 2
	evEmpty["title"] = zwsp + rlo
	evEmpty["summary"] = bom
	evEmpty["created_by_client"] = zwsp
	claim := baseRow("claims", 2)
	claim["id"], claim["investigation_id"] = 2, 2
	claim["statement"] = "the " + rlo + "cache" + zwsp + " expired"
	claim["confidence_rationale"] = "seen" + nonchar
	claim["status"], claim["verification"], claim["updated_at"] = "disputed", "disputed", updated
	hyp := baseRow("hypotheses", 2)
	hyp["id"], hyp["investigation_id"] = 2, 2
	hyp["statement"] = "hyp" + lri + "othesis"
	hyp["rationale"] = "why" + bom
	hyp["status"], hyp["verification"], hyp["updated_at"] = "superseded", "superseded", updated
	q := baseRow("questions", 2)
	q["id"], q["investigation_id"] = 2, 2
	q["question"] = "who" + zwsp + "?"
	q["resolution"] = "nobody" + nel
	q["defer_note"] = "later" + tagA
	q["status"], q["updated_at"] = "resolved", updated
	ce := baseRow("claim_evidence", 0)
	ce["claim_id"], ce["evidence_id"], ce["created_by_client"] = 2, 2, "link"+zwsp+"er"
	he := baseRow("hypothesis_evidence", 0)
	he["hypothesis_id"], he["evidence_id"], he["created_by_client"] = 2, 2, "link"+bom+"er"
	act := baseRow("activity", 0)
	act["investigation_id"] = 2
	act["actor_client"] = "act" + zwsp + "or"
	act["target_ref"] = "C-" + rlo + "002"
	act["summary"] = "Added claim" + zwsp + " C-002"
	act["note"] = "note" + nel + "text"
	act["imported"] = 1
	inserts := []struct {
		table string
		row   map[string]any
	}{
		{"investigations", cleanInv}, {"evidence", cleanEv},
		{"investigations", inv}, {"evidence", ev}, {"evidence", evEmpty}, {"claims", claim},
		{"hypotheses", hyp}, {"questions", q}, {"claim_evidence", ce}, {"hypothesis_evidence", he},
		{"activity", act},
	}
	for _, in := range inserts {
		if err := insertRow(ctx, tx, in.table, in.row); err != nil {
			t.Fatalf("insert %s: %v", in.table, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	before := dumpAll(t, w)
	var activityBefore int
	if err := w.QueryRow(`SELECT count(*) FROM activity`).Scan(&activityBefore); err != nil {
		t.Fatal(err)
	}
	if n := searchCount(t, w, "payroll"); n != 0 {
		t.Fatalf("payroll hits before = %d, want 0 (the zero-width space splits the word)", n)
	}

	if v, err := Migrate(ctx, w, migrationsUpTo(t, 4), nil); err != nil || v != 4 {
		t.Fatalf("migrate to 4 = %d, %v", v, err)
	}

	str := func(query string, args ...any) string {
		t.Helper()
		var s string
		if err := w.QueryRow(query, args...).Scan(&s); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return s
	}
	want := []struct{ query, want string }{
		{`SELECT title FROM investigations WHERE id = 2`, "Payroll outage"},
		{`SELECT problem_statement FROM investigations WHERE id = 2`, "firstsecond\nthird x"},
		{`SELECT conclusion FROM investigations WHERE id = 2`, "concluded"},
		{`SELECT conclusion_rationale FROM investigations WHERE id = 2`, "because"},
		{`SELECT created_by_client FROM investigations WHERE id = 2`, "client"},
		{`SELECT status || ' ' || updated_at FROM investigations WHERE id = 2`, "blocked " + updated},
		{`SELECT title FROM evidence WHERE id = 2`, "Logline"},
		{`SELECT summary FROM evidence WHERE id = 2`, "summary text"},
		{`SELECT raw_detail FROM evidence WHERE id = 2`, "raw\nline"},
		{`SELECT source_ref FROM evidence WHERE id = 2`, "src/ref"},
		{`SELECT source_label FROM evidence WHERE id = 2`, "label"},
		{`SELECT verification || ' ' || updated_at || ' ' || imported FROM evidence WHERE id = 2`, "source_verified " + updated + " 1"},
		{`SELECT title FROM evidence WHERE id = 3`, resanitizedTextPlaceholder},
		{`SELECT summary FROM evidence WHERE id = 3`, resanitizedTextPlaceholder},
		{`SELECT created_by_client FROM evidence WHERE id = 3`, resanitizedClientPlaceholder},
		{`SELECT statement FROM claims WHERE id = 2`, "the cache expired"},
		{`SELECT confidence_rationale FROM claims WHERE id = 2`, "seen"},
		{`SELECT status || ' ' || verification || ' ' || updated_at FROM claims WHERE id = 2`, "disputed disputed " + updated},
		{`SELECT statement FROM hypotheses WHERE id = 2`, "hypothesis"},
		{`SELECT rationale FROM hypotheses WHERE id = 2`, "why"},
		{`SELECT status || ' ' || verification || ' ' || updated_at FROM hypotheses WHERE id = 2`, "superseded superseded " + updated},
		{`SELECT question FROM questions WHERE id = 2`, "who?"},
		{`SELECT resolution FROM questions WHERE id = 2`, "nobody"},
		{`SELECT defer_note FROM questions WHERE id = 2`, "later"},
		{`SELECT status || ' ' || updated_at FROM questions WHERE id = 2`, "resolved " + updated},
		{`SELECT created_by_client FROM claim_evidence WHERE claim_id = 2`, "linker"},
		{`SELECT created_by_client FROM hypothesis_evidence WHERE hypothesis_id = 2`, "linker"},
		{`SELECT actor_client FROM activity WHERE investigation_id = 2`, "actor"},
		{`SELECT target_ref FROM activity WHERE investigation_id = 2`, "C-002"},
		{`SELECT summary FROM activity WHERE investigation_id = 2`, "Added claim C-002"},
		{`SELECT note FROM activity WHERE investigation_id = 2`, "notetext"},
		{`SELECT imported FROM activity WHERE investigation_id = 2`, "1"},
		// The search index follows the cleaned text.
		{`SELECT title FROM search_index WHERE rowid = 2 * 8 + 1`, "Payroll outage"},
		{`SELECT body FROM search_index WHERE rowid = 2 * 8 + 3`, "the cache expired"},
	}
	for _, c := range want {
		if got := str(c.query); got != c.want {
			t.Errorf("%s = %q, want %q", c.query, got, c.want)
		}
	}
	if n := searchCount(t, w, "payroll"); n != 1 {
		t.Errorf("payroll hits after = %d, want 1", n)
	}

	// Clean rows are untouched, and no activity is written.
	after := dumpAll(t, w)
	for _, table := range []string{"investigations", "evidence"} {
		for i, row := range before[table] {
			if !strings.HasPrefix(row, `rowid="1"`) && !strings.HasPrefix(row, `rowid="9"`) {
				continue
			}
			if after[table][i] != row {
				t.Errorf("clean %s row changed:\n before %s\n after  %s", table, row, after[table][i])
			}
		}
	}
	for _, table := range []string{"claims", "hypotheses"} {
		if after[table][0] != before[table][0] {
			t.Errorf("clean %s row changed:\n before %s\n after  %s", table, before[table][0], after[table][0])
		}
	}
	var activityAfter int
	if err := w.QueryRow(`SELECT count(*) FROM activity`).Scan(&activityAfter); err != nil {
		t.Fatal(err)
	}
	if activityAfter != activityBefore {
		t.Errorf("activity rows %d -> %d, want no new rows", activityBefore, activityAfter)
	}

	// Idempotent: a second run issues no UPDATE at all.
	conn, err := w.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	changes := func() int {
		var n int
		if err := conn.QueryRowContext(ctx, `SELECT total_changes()`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	n0 := changes()
	rtx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := resanitizeText(ctx, rtx); err != nil {
		t.Fatal(err)
	}
	if err := rtx.Commit(); err != nil {
		t.Fatal(err)
	}
	if n1 := changes(); n1 != n0 {
		t.Errorf("second run changed %d rows, want 0", n1-n0)
	}
	conn.Close()
	again := dumpAll(t, w)
	for table, rows := range after {
		if strings.Join(rows, "\n") != strings.Join(again[table], "\n") {
			t.Errorf("%s changed on the second run", table)
		}
	}
	if v, err := Migrate(ctx, w, migrationsUpTo(t, 4), nil); err != nil || v != 4 || migrationRows(t, w) != 4 {
		t.Fatalf("re-run migrate = %d, %v", v, err)
	}
}

// TestResanitizeColumnsCoverSchema fails when a TEXT column is added to a
// table migration 0004 cleans without deciding whether it is free text.
func TestResanitizeColumnsCoverSchema(t *testing.T) {
	db := openTestDB(t)
	// Columns that hold no free text: enums, timestamps, JSON metadata.
	notFree := map[string]bool{
		"status": true, "pre_archive_status": true, "conclusion_confidence": true, "created_at": true,
		"updated_at": true, "archived_at": true, "created_by_kind": true, "source_type": true,
		"captured_at": true, "verification": true, "metadata": true, "confidence": true, "resolved_at": true,
		"relation": true, "actor_kind": true, "action": true,
		"conclusion_verification": true, "conclusion_supporting_claims": true, "conclusion_contradicting_claims": true,
		"conclusion_reviewed_at": true, "conclusion_review_snapshot": true,
	}
	for _, tb := range resanitizeTables {
		listed := map[string]bool{}
		for _, c := range tb.columns {
			listed[c.name] = true
		}
		rows, err := db.reader.Query(`SELECT name, type FROM pragma_table_info(?)`, tb.table)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var name, typ string
			if err := rows.Scan(&name, &typ); err != nil {
				t.Fatal(err)
			}
			// Schema 8 adds review_basis after migration 0004 has run. It is
			// sanitized on all application writes and never trusted on import.
			laterSanitized := tb.table == "investigations" && name == "conclusion_review_basis"
			if typ == "TEXT" && !listed[name] && !notFree[name] && !laterSanitized {
				t.Errorf("%s.%s is TEXT but neither re-sanitized nor listed as not free text", tb.table, name)
			}
		}
		rows.Close()
	}
}

func searchCount(t *testing.T, q *sql.DB, term string) int {
	t.Helper()
	var n int
	if err := q.QueryRow(`SELECT count(*) FROM search_index WHERE search_index MATCH ?`, term).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
