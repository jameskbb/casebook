package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
)

const activeInvestigationKey = "active_investigation_id"

// GetActiveInvestigationID returns the active investigation id and whether one
// is set. A malformed value reads as unset.
func GetActiveInvestigationID(ctx context.Context, q Querier) (int64, bool, error) {
	var v string
	err := q.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, activeInvestigationKey).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0, false, nil
	}
	return id, true, nil
}

// SetActiveInvestigationID stores id as the active investigation.
func SetActiveInvestigationID(ctx context.Context, q Querier, id int64, now string) error {
	_, err := q.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		activeInvestigationKey, strconv.FormatInt(id, 10), now)
	return err
}

// ClearActiveInvestigation removes the active investigation setting and
// reports whether one was set.
func ClearActiveInvestigation(ctx context.Context, q Querier) (bool, error) {
	res, err := q.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, activeInvestigationKey)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// DirectoryBinding binds a directory to an investigation (ADR-0015).
type DirectoryBinding struct {
	Dir             string
	InvestigationID int64
	UpdatedAt       string
}

// NearestDirectoryBinding returns the binding of the first of dirs that has
// one (callers pass a directory and then its ancestors, nearest first) and
// whether any matched.
func NearestDirectoryBinding(ctx context.Context, q Querier, dirs []string) (DirectoryBinding, bool, error) {
	for _, d := range dirs {
		var b DirectoryBinding
		err := q.QueryRowContext(ctx, `SELECT dir, investigation_id, updated_at FROM directory_investigations WHERE dir = ?`, d).
			Scan(&b.Dir, &b.InvestigationID, &b.UpdatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return DirectoryBinding{}, false, err
		}
		return b, true, nil
	}
	return DirectoryBinding{}, false, nil
}

// SetDirectoryBinding binds dir to investigation id, replacing any binding
// dir had.
func SetDirectoryBinding(ctx context.Context, q Querier, dir string, id int64, now string) error {
	_, err := q.ExecContext(ctx, `INSERT INTO directory_investigations (dir, investigation_id, updated_at) VALUES (?, ?, ?)
ON CONFLICT (dir) DO UPDATE SET investigation_id = excluded.investigation_id, updated_at = excluded.updated_at`,
		dir, id, now)
	return err
}

// DeleteDirectoryBinding removes dir's binding and reports whether it had
// one.
func DeleteDirectoryBinding(ctx context.Context, q Querier, dir string) (bool, error) {
	res, err := q.ExecContext(ctx, `DELETE FROM directory_investigations WHERE dir = ?`, dir)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// ListDirectoryBindings returns every binding ordered by dir.
func ListDirectoryBindings(ctx context.Context, q Querier) ([]DirectoryBinding, error) {
	rows, err := q.QueryContext(ctx, `SELECT dir, investigation_id, updated_at FROM directory_investigations ORDER BY dir`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DirectoryBinding{}
	for rows.Next() {
		var b DirectoryBinding
		if err := rows.Scan(&b.Dir, &b.InvestigationID, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
