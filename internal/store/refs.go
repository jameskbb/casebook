package store

import (
	"context"
	"fmt"

	"github.com/jameskbb/casebook/internal/domain"
)

// AllocateInvestigationNum returns the next global investigation ref number.
// Numbers are never reused. Call it inside a write transaction. A number above
// domain.MaxRefNum is refused with an error wrapping domain.ErrConflict (the
// caller's transaction then rolls back), so no unparseable ref is ever
// created.
func AllocateInvestigationNum(ctx context.Context, q Querier) (int64, error) {
	var n int64
	err := q.QueryRowContext(ctx, `INSERT INTO counters (name, value) VALUES ('investigation', 1)
ON CONFLICT (name) DO UPDATE SET value = value + 1
RETURNING value`).Scan(&n)
	if err != nil {
		return 0, err
	}
	if n > domain.MaxRefNum {
		return 0, fmt.Errorf("%w: investigation refs are exhausted (the highest is %s)",
			domain.ErrConflict, domain.FormatInvestigationRef(domain.MaxRefNum))
	}
	return n, nil
}

// AllocateRecordNum returns the next per-investigation ref number for a record
// type. Numbers are never reused. Call it inside a write transaction. A number
// above domain.MaxRefNum is refused with an error wrapping domain.ErrConflict.
func AllocateRecordNum(ctx context.Context, q Querier, invID int64, t domain.RecordType) (int64, error) {
	var query string
	// The column comes from this fixed switch, never from input.
	switch t {
	case domain.RecordEvidence:
		query = `UPDATE investigations SET last_evidence_num = last_evidence_num + 1 WHERE id = ? RETURNING last_evidence_num`
	case domain.RecordClaim:
		query = `UPDATE investigations SET last_claim_num = last_claim_num + 1 WHERE id = ? RETURNING last_claim_num`
	case domain.RecordHypothesis:
		query = `UPDATE investigations SET last_hypothesis_num = last_hypothesis_num + 1 WHERE id = ? RETURNING last_hypothesis_num`
	case domain.RecordQuestion:
		query = `UPDATE investigations SET last_question_num = last_question_num + 1 WHERE id = ? RETURNING last_question_num`
	default:
		return 0, fmt.Errorf("allocate record ref: unsupported record type %q", t)
	}
	var n int64
	if err := q.QueryRowContext(ctx, query, invID).Scan(&n); err != nil {
		return 0, err
	}
	if n > domain.MaxRefNum {
		return 0, fmt.Errorf("%w: %s refs are exhausted in this investigation (the highest is %s)",
			domain.ErrConflict, t, domain.FormatRecordRef(t, domain.MaxRefNum))
	}
	return n, nil
}
