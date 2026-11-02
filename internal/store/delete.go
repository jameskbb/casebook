package store

import (
	"context"
	"fmt"

	"github.com/jameskbb/casebook/internal/domain"
)

// Deletes (AR-10). The app checks the guards first (EvidenceDependents,
// SupersededByRefs); these functions only remove rows. Foreign keys cascade
// a record's own links and, for an investigation, every row that belongs to
// it (records, links and its activity); the FTS triggers remove the search
// rows. Ref counters never go down, so a deleted ref is never reused.

// EvidenceDependents counts the claim and hypothesis links to evidence id.
func EvidenceDependents(ctx context.Context, q Querier, id int64) (claims, hypotheses int, err error) {
	err = q.QueryRowContext(ctx, `SELECT
    (SELECT count(*) FROM claim_evidence WHERE evidence_id = ?),
    (SELECT count(*) FROM hypothesis_evidence WHERE evidence_id = ?)`, id, id).Scan(&claims, &hypotheses)
	return claims, hypotheses, err
}

// SupersededByRefs returns the ref numbers of the records of kind t that
// name record id as their replacement, in ref order.
func SupersededByRefs(ctx context.Context, q Querier, t LinkTarget, id int64) ([]int64, error) {
	s, err := t.spec()
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, `SELECT ref_num FROM `+s.table+` WHERE superseded_by_id = ? ORDER BY ref_num`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var n int64
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// DeleteEvidence deletes evidence id.
func DeleteEvidence(ctx context.Context, q Querier, id int64) error {
	return deleteByID(ctx, q, "evidence", id)
}

// DeleteStatement deletes claim or hypothesis id and, by cascade, its links.
func DeleteStatement(ctx context.Context, q Querier, t LinkTarget, id int64) error {
	s, err := t.spec()
	if err != nil {
		return err
	}
	return deleteByID(ctx, q, s.table, id)
}

// DeleteQuestion deletes question id.
func DeleteQuestion(ctx context.Context, q Querier, id int64) error {
	return deleteByID(ctx, q, "questions", id)
}

// DeleteInvestigation deletes investigation id and, by cascade, everything
// that belongs to it.
func DeleteInvestigation(ctx context.Context, q Querier, id int64) error {
	return deleteByID(ctx, q, "investigations", id)
}

// deleteByID deletes one row of table (a name from this file only).
func deleteByID(ctx context.Context, q Querier, table string, id int64) error {
	res, err := q.ExecContext(ctx, `DELETE FROM `+table+` WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete from %s: %w", table, err)
	}
	return expectOne(res)
}

// InvestigationCounts counts the evidence, claims, hypotheses and questions
// of investigation id (the other Counts fields stay 0).
func InvestigationCounts(ctx context.Context, q Querier, id int64) (domain.Counts, error) {
	var c domain.Counts
	err := q.QueryRowContext(ctx, `SELECT
    (SELECT count(*) FROM evidence WHERE investigation_id = ?),
    (SELECT count(*) FROM claims WHERE investigation_id = ?),
    (SELECT count(*) FROM hypotheses WHERE investigation_id = ?),
    (SELECT count(*) FROM questions WHERE investigation_id = ?)`, id, id, id, id).Scan(
		&c.Evidence, &c.Claims, &c.Hypotheses, &c.Questions)
	return c, err
}
