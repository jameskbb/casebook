package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// NewActivity is the data needed to insert an activity row. InvestigationID 0
// means a global row (stored as NULL).
type NewActivity struct {
	InvestigationID int64
	Actor           domain.Actor
	Action          string
	TargetRef       string
	Summary         string
	Note            string
	CreatedAt       string
	Imported        bool // copied from an imported file
}

// InsertActivity inserts an activity row and returns its id.
func InsertActivity(ctx context.Context, q Querier, a NewActivity) (int64, error) {
	var invID sql.NullInt64
	if a.InvestigationID != 0 {
		invID = sql.NullInt64{Int64: a.InvestigationID, Valid: true}
	}
	var id int64
	err := q.QueryRowContext(ctx, `INSERT INTO activity
    (investigation_id, actor_kind, actor_client, action, target_ref, summary, note, created_at, imported)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id`,
		invID, string(a.Actor.Kind), a.Actor.Client, a.Action, a.TargetRef, a.Summary, a.Note, a.CreatedAt,
		a.Imported).Scan(&id)
	return id, err
}

// ActivityQuery selects activity rows. InvestigationID 0 means every
// investigation plus global rows. AfterID > 0 keeps rows with a larger id.
// Ascending returns the oldest matching rows first (for paging forward from
// a cursor); otherwise the newest come first.
type ActivityQuery struct {
	InvestigationID int64
	AfterID         int64
	Limit           int
	Ascending       bool
}

// ListActivity returns up to Limit activity rows, newest first, or oldest
// first when Ascending.
func ListActivity(ctx context.Context, q Querier, f ActivityQuery) ([]domain.Activity, error) {
	var (
		where []string
		args  []any
	)
	if f.InvestigationID != 0 {
		where = append(where, "a.investigation_id = ?")
		args = append(args, f.InvestigationID)
	}
	if f.AfterID > 0 {
		where = append(where, "a.id > ?")
		args = append(args, f.AfterID)
	}
	query := `SELECT a.id, i.ref_num, a.actor_kind, a.actor_client, a.action, a.target_ref, a.summary, a.note, a.created_at, a.imported
FROM activity a LEFT JOIN investigations i ON i.id = a.investigation_id`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	if f.Ascending {
		query += " ORDER BY a.id ASC LIMIT ?"
	} else {
		query += " ORDER BY a.id DESC LIMIT ?"
	}
	args = append(args, f.Limit)

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Activity{}
	for rows.Next() {
		var (
			a      domain.Activity
			refNum sql.NullInt64
			kind   string
		)
		if err := rows.Scan(&a.ID, &refNum, &kind, &a.Actor.Client, &a.Action, &a.TargetRef, &a.Summary, &a.Note, &a.CreatedAt, &a.Imported); err != nil {
			return nil, err
		}
		if refNum.Valid {
			a.InvestigationRef = domain.FormatInvestigationRef(refNum.Int64)
		}
		a.Actor.Kind = domain.ActorKind(kind)
		out = append(out, a)
	}
	return out, rows.Err()
}

// LatestActivity returns the id and created_at of the newest activity row of
// investigation invID with the given action, or 0 and "" when there is none.
func LatestActivity(ctx context.Context, q Querier, invID int64, action string) (int64, string, error) {
	var (
		id int64
		at string
	)
	err := q.QueryRowContext(ctx, `SELECT id, created_at FROM activity
WHERE investigation_id = ? AND action = ? ORDER BY id DESC LIMIT 1`, invID, action).Scan(&id, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", err
	}
	return id, at, nil
}
