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
	query := `SELECT i.id, i.ref_num, i.title, i.status, i.conclusion_confidence, i.created_at, i.updated_at,
    COALESCE(i.archived_at, ''),
    COALESCE((SELECT s.value = CAST(i.id AS TEXT) FROM settings s WHERE s.key = 'active_investigation_id'), 0),
    (SELECT count(*) FROM evidence e WHERE e.investigation_id = i.id),
    (SELECT count(*) FROM claims c WHERE c.investigation_id = i.id),
    (SELECT count(*) FROM hypotheses h WHERE h.investigation_id = i.id),
    (SELECT count(*) FROM questions qq WHERE qq.investigation_id = i.id),
    (SELECT count(*) FROM questions qq WHERE qq.investigation_id = i.id AND qq.status = 'open')
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
		if err := rows.Scan(&s.ID, &refNum, &s.Title, &status, &conf, &s.CreatedAt, &s.UpdatedAt, &s.ArchivedAt,
			&s.IsActive, &s.Counts.Evidence, &s.Counts.Claims, &s.Counts.Hypotheses, &s.Counts.Questions,
			&s.Counts.OpenQuestions); err != nil {
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
