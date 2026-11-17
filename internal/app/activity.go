package app

import (
	"context"
	"database/sql"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// Activity list limits.
const (
	defaultActivityLimit = 50
	maxActivityLimit     = 500
)

// recordActivity is the single helper every mutation uses to log itself in
// the same transaction. invID 0 records a global row. summary is collapsed to
// one line and truncated to MaxActivitySummaryBytes.
func (a *App) recordActivity(ctx context.Context, tx *sql.Tx, actor domain.Actor, invID int64, action, targetRef, summary, note, now string) error {
	_, err := store.InsertActivity(ctx, tx, store.NewActivity{
		InvestigationID: invID,
		Actor:           actor,
		Action:          action,
		TargetRef:       targetRef,
		Summary:         domain.TruncateLine(summary, domain.MaxActivitySummaryBytes),
		Note:            note,
		CreatedAt:       now,
	})
	return err
}

// ListActivity returns activity rows. For this method only, an empty
// InvestigationRef means all investigations (including global rows), not the
// active investigation; a whitespace-only ref counts as empty. Limit 0 means
// 50; a negative value or one above 500 is a ValidationError on limit (like
// every other limit option).
//
// Order depends on AfterID:
//
//   - AfterID 0: the newest Limit rows, newest first (a recent-activity view).
//   - AfterID > 0: the rows with id > AfterID in ascending id order (oldest
//     first), at most Limit of them. This is a forward cursor with no gaps:
//     set AfterID to the largest id received and call again until fewer than
//     Limit rows come back.
//
// A restore replaces the activity table with the backup's rows, so ids can
// go down or be reused: a feed that sees ChangeToken move and the newest id
// fall below its cursor must start over with AfterID 0.
func (a *App) ListActivity(ctx context.Context, f ActivityFilter) ([]Activity, error) {
	limit := f.Limit
	if limit < 0 || limit > maxActivityLimit {
		return nil, a.fail("activity.list",
			domain.NewValidationError("limit", "must be between 0 and %d", maxActivityLimit))
	}
	if limit == 0 {
		limit = defaultActivityLimit
	}
	var out []Activity
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		q := store.ActivityQuery{AfterID: f.AfterID, Limit: limit, Ascending: f.AfterID > 0}
		if strings.TrimSpace(f.InvestigationRef) != "" {
			inv, err := a.resolveInvestigation(ctx, tx, f.InvestigationRef)
			if err != nil {
				return err
			}
			q.InvestigationID = inv.ID
		}
		rows, err := store.ListActivity(ctx, tx, q)
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	if err != nil {
		return nil, a.fail("activity.list", err)
	}
	a.log.Debug("activity.list", "investigation_ref", f.InvestigationRef, "rows", len(out))
	return out, nil
}
