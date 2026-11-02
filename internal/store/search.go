package store

import (
	"context"
	"fmt"

	"github.com/jameskbb/casebook/internal/domain"
)

// SearchQuery is a full-text search. Match is an FTS5 MATCH expression the
// caller has already made safe (internal/app quotes every term); it is bound
// as a parameter, never spliced into SQL. InvestigationID 0 means every
// investigation. Archived investigations are skipped unless IncludeArchived.
type SearchQuery struct {
	Match           string
	InvestigationID int64
	IncludeArchived bool
	Limit           int
}

// Search runs q against search_index and returns hits ranked by bm25 (title
// weighted 4, body 1; lower is better), then by investigation ref number,
// then by index rowid, so equal scores order deterministically. record_id is
// mapped to the display ref of its record type. The slice is never nil.
func Search(ctx context.Context, q Querier, f SearchQuery) ([]domain.SearchHit, error) {
	query := `SELECT s.record_type, i.ref_num, i.title,
    CASE s.record_type
        WHEN 'investigation' THEN i.ref_num
        WHEN 'evidence' THEN (SELECT e.ref_num FROM evidence e WHERE e.id = s.record_id)
        WHEN 'claim' THEN (SELECT c.ref_num FROM claims c WHERE c.id = s.record_id)
        WHEN 'hypothesis' THEN (SELECT h.ref_num FROM hypotheses h WHERE h.id = s.record_id)
        WHEN 'question' THEN (SELECT qq.ref_num FROM questions qq WHERE qq.id = s.record_id)
    END,
    snippet(search_index, -1, char(2), char(3), '...', 12),
    bm25(search_index, 4.0, 1.0) AS score
FROM search_index s
JOIN investigations i ON i.id = s.investigation_id
WHERE search_index MATCH ?`
	args := []any{f.Match}
	if f.InvestigationID != 0 {
		query += " AND s.investigation_id = ?"
		args = append(args, f.InvestigationID)
	}
	if !f.IncludeArchived {
		query += " AND i.archived_at IS NULL"
	}
	query += " ORDER BY score, i.ref_num, s.rowid LIMIT ?"
	args = append(args, f.Limit)

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SearchHit{}
	for rows.Next() {
		var (
			h              domain.SearchHit
			recType        string
			invNum, recNum int64
		)
		var raw string
		if err := rows.Scan(&recType, &invNum, &h.InvestigationTitle, &recNum, &raw, &h.Rank); err != nil {
			return nil, err
		}
		h.Snippet, h.Segments = domain.ParseSnippet(raw)
		h.RecordType = domain.RecordType(recType)
		if !h.RecordType.Valid() || recNum <= 0 {
			return nil, fmt.Errorf("search: unexpected index row type %q ref %d", recType, recNum)
		}
		h.InvestigationRef = domain.FormatInvestigationRef(invNum)
		h.RecordRef = domain.FormatRecordRef(h.RecordType, recNum)
		out = append(out, h)
	}
	return out, rows.Err()
}
