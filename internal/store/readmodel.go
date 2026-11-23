package store

import (
	"context"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// SummaryQuery selects investigations for ListInvestigationSummaries.
// Archived rows are skipped unless IncludeArchived. Status "" means any.
type SummaryQuery struct {
	IncludeArchived bool
	Status          domain.InvestigationStatus
	Limit           int
}

// ListInvestigationSummaries returns investigation summaries with record
// counts, newest updated_at first (ties by ref number, newest first). The
// slice is never nil.
func ListInvestigationSummaries(ctx context.Context, q Querier, f SummaryQuery) ([]domain.InvestigationSummary, error) {
	var (
		where []string
		args  []any
	)
	if !f.IncludeArchived {
		where = append(where, "i.archived_at IS NULL")
	}
	if f.Status != "" {
		where = append(where, "i.status = ?")
		args = append(args, string(f.Status))
	}
	query := `SELECT i.id, i.ref_num, i.title, i.status, i.conclusion, i.conclusion_confidence, i.created_at, i.updated_at,
    COALESCE(i.archived_at, ''),
    COALESCE((SELECT s.value = CAST(i.id AS TEXT) FROM settings s WHERE s.key = 'active_investigation_id'), 0),
    (SELECT count(*) FROM evidence e WHERE e.investigation_id = i.id),
    (SELECT count(*) FROM claims c WHERE c.investigation_id = i.id),
    (SELECT count(*) FROM hypotheses h WHERE h.investigation_id = i.id),
    (SELECT count(*) FROM questions qq WHERE qq.investigation_id = i.id),
    (SELECT count(*) FROM questions qq WHERE qq.investigation_id = i.id AND qq.status = 'open'),
    (SELECT count(*) FROM evidence e WHERE e.investigation_id = i.id AND e.verification != 'superseded'),
    (SELECT count(*) FROM evidence e WHERE e.investigation_id = i.id AND e.verification = 'human_verified'),
    (SELECT count(*) FROM claims c WHERE c.investigation_id = i.id AND c.status NOT IN ('superseded', 'rejected')
        AND (c.verification = 'disputed' OR (c.verification != 'human_verified' AND EXISTS (
            SELECT 1 FROM claim_evidence l JOIN evidence e ON e.id = l.evidence_id
            WHERE l.claim_id = c.id AND l.relation = 'contradicts' AND e.verification != 'superseded'))))
     + (SELECT count(*) FROM hypotheses h WHERE h.investigation_id = i.id AND h.status NOT IN ('superseded', 'disproven')
        AND (h.verification = 'disputed' OR (h.verification != 'human_verified' AND EXISTS (
            SELECT 1 FROM hypothesis_evidence l JOIN evidence e ON e.id = l.evidence_id
            WHERE l.hypothesis_id = h.id AND l.relation = 'contradicts' AND e.verification != 'superseded')))),
    ` + findingCount(backingClaim, backingHypothesis) + `,
    ` + findingCount(decidedClaim, decidedHypothesis) + `
FROM investigations i`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY i.updated_at DESC, i.ref_num DESC LIMIT ?"
	args = append(args, f.Limit)

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.InvestigationSummary{}
	for rows.Next() {
		var (
			s            domain.InvestigationSummary
			refNum       int64
			status, conf string
		)
		if err := rows.Scan(&s.ID, &refNum, &s.Title, &status, &s.Conclusion, &conf, &s.CreatedAt, &s.UpdatedAt, &s.ArchivedAt,
			&s.IsActive, &s.Counts.Evidence, &s.Counts.Claims, &s.Counts.Hypotheses, &s.Counts.Questions,
			&s.Counts.OpenQuestions, &s.SourcesTotal, &s.SourcesCheckedByYou, &s.FindingsDisagree, &s.FindingsBacking, &s.FindingsDecided); err != nil {
			return nil, err
		}
		s.Ref = domain.FormatInvestigationRef(refNum)
		s.Status = domain.InvestigationStatus(status)
		s.ConclusionConfidence = domain.Confidence(conf)
		s.Archived = s.ArchivedAt != ""
		out = append(out, s)
	}
	return out, rows.Err()
}

// against is true when a source that has not been replaced goes against the
// finding (alias f) through the link table, as the disagree count reads it.
func against(table, fk string) string {
	return `EXISTS (SELECT 1 FROM ` + table + ` l JOIN evidence e ON e.id = l.evidence_id
            WHERE l.` + fk + ` = f.id AND l.relation = 'contradicts' AND e.verification != 'superseded')`
}

// The rules for the list's finding counts, the same as the desktop's
// lib/records groupFindings: a finding backs the answer when it is current,
// tested (a hypothesis is supported) and sources do not disagree about it; a
// person decided one when they settled it while a source still goes against it.
var (
	claimDisagree      = `(f.verification = 'disputed' OR f.status = 'disputed' OR (f.verification != 'human_verified' AND ` + against("claim_evidence", "claim_id") + `))`
	hypothesisDisagree = `(f.verification = 'disputed' OR (f.verification != 'human_verified' AND ` + against("hypothesis_evidence", "hypothesis_id") + `))`
	backingClaim       = `SELECT count(*) FROM claims f WHERE f.investigation_id = i.id AND f.status NOT IN ('superseded', 'rejected') AND NOT ` + claimDisagree
	backingHypothesis  = `SELECT count(*) FROM hypotheses f WHERE f.investigation_id = i.id AND f.status = 'supported' AND NOT ` + hypothesisDisagree
	decidedClaim       = `SELECT count(*) FROM claims f WHERE f.investigation_id = i.id AND f.status NOT IN ('superseded', 'rejected', 'disputed') AND f.verification = 'human_verified' AND ` + against("claim_evidence", "claim_id")
	decidedHypothesis  = `SELECT count(*) FROM hypotheses f WHERE f.investigation_id = i.id AND f.status NOT IN ('superseded', 'disproven') AND f.verification = 'human_verified' AND ` + against("hypothesis_evidence", "hypothesis_id")
)

// findingCount adds a claims count and a hypotheses count.
func findingCount(claims, hypotheses string) string {
	return "(" + claims + ") + (" + hypotheses + ")"
}
