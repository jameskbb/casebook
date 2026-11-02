package app

import (
	"context"
	"database/sql"
	"strings"
	"unicode"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// Search limits.
const (
	defaultSearchLimit = 20
	maxSearchLimit     = 100
	maxSearchTerms     = 16
)

// Search runs a full-text search over investigation titles, problem
// statements and conclusions, evidence titles and summaries (never raw
// detail), claim and hypothesis statements, and questions with their
// resolutions. Hits are ranked by bm25 (title matches weigh more).
//
// query is user text, not FTS5 syntax: every term becomes a quoted literal
// (so operators such as AND, NEAR( or column filters are just words), the
// last term matches as a prefix, and the terms are ANDed. An empty query is a
// ValidationError on query; a query with no searchable terms (punctuation
// only) returns no hits. A query FTS5 still cannot run is a ValidationError
// on query, never raw FTS text.
//
// For Search, an empty InvestigationRef searches every investigation (not the
// active one). Archived investigations are skipped unless IncludeArchived is
// true or InvestigationRef names one explicitly. Limit 0 means 20; a negative
// value or one above 100 is a ValidationError on limit. The slice is never
// nil.
func (a *App) Search(ctx context.Context, query string, o SearchOptions) ([]SearchHit, error) {
	const op = "search"
	q, err := domain.SanitizeLine("query", query, domain.MaxSearchQueryBytes, true)
	if err != nil {
		return nil, a.fail(op, err)
	}
	limit := o.Limit
	if limit < 0 || limit > maxSearchLimit {
		return nil, a.fail(op, domain.NewValidationError("limit", "must be between 0 and %d", maxSearchLimit))
	}
	if limit == 0 {
		limit = defaultSearchLimit
	}
	match := buildMatchQuery(q)
	var out []SearchHit
	err = a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		f := store.SearchQuery{Match: match, IncludeArchived: o.IncludeArchived, Limit: limit}
		if strings.TrimSpace(o.InvestigationRef) != "" {
			inv, err := a.resolveInvestigation(ctx, tx, o.InvestigationRef)
			if err != nil {
				return err
			}
			f.InvestigationID = inv.ID
			f.IncludeArchived = true
		}
		if match == "" {
			out = []SearchHit{}
			return nil
		}
		hits, err := store.Search(ctx, tx, f)
		if err != nil {
			if store.IsSQLite(err) && !store.IsBusy(err) {
				a.log.Debug("search.match_failed")
				return domain.NewValidationError("query", "could not be searched")
			}
			return err
		}
		out = hits
		return nil
	})
	if err != nil {
		return nil, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", o.InvestigationRef, "hits", len(out))
	return out, nil
}

// buildMatchQuery turns user text into a safe FTS5 MATCH expression: split on
// whitespace, drop terms with no letter or digit, keep at most 16 terms, wrap
// each in double quotes (embedded quotes doubled) so it is a literal phrase,
// make the last term a prefix query, and join with spaces (implicit AND).
// It returns "" when no term remains.
func buildMatchQuery(q string) string {
	var terms []string
	for _, f := range strings.Fields(q) {
		if !strings.ContainsFunc(f, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
			continue
		}
		terms = append(terms, `"`+strings.ReplaceAll(f, `"`, `""`)+`"`)
		if len(terms) == maxSearchTerms {
			break
		}
	}
	if len(terms) == 0 {
		return ""
	}
	terms[len(terms)-1] += "*"
	return strings.Join(terms, " ")
}
