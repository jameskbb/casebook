package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/jameskbb/casebook/internal/domain"
)

// NewInvestigation is the data needed to insert an investigation row.
type NewInvestigation struct {
	RefNum           int64
	Title            string
	ProblemStatement string
	Status           domain.InvestigationStatus
	CreatedAt        string
	CreatedBy        domain.Actor
}

// InsertInvestigation inserts a row and returns its id.
func InsertInvestigation(ctx context.Context, q Querier, in NewInvestigation) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `INSERT INTO investigations
    (ref_num, title, problem_statement, status, created_at, updated_at, created_by_kind, created_by_client)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id`,
		in.RefNum, in.Title, in.ProblemStatement, string(in.Status), in.CreatedAt, in.CreatedAt,
		string(in.CreatedBy.Kind), in.CreatedBy.Client).Scan(&id)
	return id, err
}

// investigationColumns selects an investigation row; is_active is computed
// from the active_investigation_id setting.
const investigationColumns = `i.id, i.ref_num, i.title, i.problem_statement, i.status, i.conclusion,
    i.conclusion_confidence, i.conclusion_rationale, i.conclusion_verification, i.conclusion_supporting_claims, i.conclusion_contradicting_claims, i.conclusion_reviewed_at, i.conclusion_review_basis, i.created_at, i.updated_at,
    COALESCE(i.archived_at, ''), i.created_by_kind, i.created_by_client,
    COALESCE((SELECT s.value = CAST(i.id AS TEXT) FROM settings s WHERE s.key = 'active_investigation_id'), 0)`

func scanInvestigation(row interface{ Scan(...any) error }) (domain.Investigation, error) {
	var (
		inv                       domain.Investigation
		refNum                    int64
		status                    string
		conf                      string
		kind                      string
		isActive                  bool
		supporting, contradicting string
	)
	if err := row.Scan(&inv.ID, &refNum, &inv.Title, &inv.ProblemStatement, &status, &inv.Conclusion,
		&conf, &inv.ConclusionRationale, &inv.ConclusionVerification, &supporting, &contradicting, &inv.ConclusionReviewedAt, &inv.ConclusionReviewBasis, &inv.CreatedAt, &inv.UpdatedAt,
		&inv.ArchivedAt, &kind, &inv.CreatedBy.Client, &isActive); err != nil {
		return domain.Investigation{}, err
	}
	if err := json.Unmarshal([]byte(supporting), &inv.ConclusionSupportingClaims); err != nil {
		return inv, err
	}
	if err := json.Unmarshal([]byte(contradicting), &inv.ConclusionContradictingClaims); err != nil {
		return inv, err
	}
	inv.Ref = domain.FormatInvestigationRef(refNum)
	inv.Status = domain.InvestigationStatus(status)
	inv.ConclusionConfidence = domain.Confidence(conf)
	inv.CreatedBy.Kind = domain.ActorKind(kind)
	inv.Archived = inv.ArchivedAt != ""
	inv.IsActive = isActive
	return inv, nil
}

// GetInvestigationByNum returns the investigation with ref number n, or an
// error matching sql.ErrNoRows.
func GetInvestigationByNum(ctx context.Context, q Querier, n int64) (domain.Investigation, error) {
	return scanInvestigation(q.QueryRowContext(ctx,
		`SELECT `+investigationColumns+` FROM investigations i WHERE i.ref_num = ?`, n))
}

// GetInvestigationByID returns the investigation with primary key id, or an
// error matching sql.ErrNoRows.
func GetInvestigationByID(ctx context.Context, q Querier, id int64) (domain.Investigation, error) {
	return scanInvestigation(q.QueryRowContext(ctx,
		`SELECT `+investigationColumns+` FROM investigations i WHERE i.id = ?`, id))
}

// TouchInvestigation sets updated_at. It does not rewrite the search index.
func TouchInvestigation(ctx context.Context, q Querier, id int64, now string) error {
	res, err := q.ExecContext(ctx, `UPDATE investigations SET updated_at = ? WHERE id = ?`, now, id)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// expectOne returns sql.ErrNoRows when res affected no row.
func expectOne(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// nullString maps "" to SQL NULL.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// GetPreArchiveStatus returns the status an archived investigation had before
// it was archived, or "" when none is recorded.
func GetPreArchiveStatus(ctx context.Context, q Querier, id int64) (domain.InvestigationStatus, error) {
	var s sql.NullString
	err := q.QueryRowContext(ctx, `SELECT pre_archive_status FROM investigations WHERE id = ?`, id).Scan(&s)
	if err != nil {
		return "", err
	}
	return domain.InvestigationStatus(s.String), nil
}

// UpdateInvestigationText sets title and problem_statement and updated_at.
func UpdateInvestigationText(ctx context.Context, q Querier, id int64, title, problem, now string) error {
	res, err := q.ExecContext(ctx,
		`UPDATE investigations SET title = ?, problem_statement = ?, updated_at = ? WHERE id = ?`,
		title, problem, now, id)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// InvestigationStatusUpdate is a full status write. PreArchiveStatus and
// ArchivedAt are stored as NULL when "".
type InvestigationStatusUpdate struct {
	Status           domain.InvestigationStatus
	PreArchiveStatus domain.InvestigationStatus
	ArchivedAt       string
	UpdatedAt        string
}

// SetInvestigationStatus writes status, pre_archive_status, archived_at and
// updated_at. It does not rewrite the search index.
func SetInvestigationStatus(ctx context.Context, q Querier, id int64, u InvestigationStatusUpdate) error {
	res, err := q.ExecContext(ctx, `UPDATE investigations
SET status = ?, pre_archive_status = ?, archived_at = ?, updated_at = ?
WHERE id = ?`,
		string(u.Status), nullString(string(u.PreArchiveStatus)), nullString(u.ArchivedAt), u.UpdatedAt, id)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// UpdateInvestigationConclusion replaces the conclusion, its confidence and
// rationale, and sets updated_at.
func UpdateInvestigationConclusion(ctx context.Context, q Querier, id int64, conclusion string,
	confidence domain.Confidence, rationale, now string) error {
	res, err := q.ExecContext(ctx, `UPDATE investigations
SET conclusion = ?, conclusion_confidence = ?, conclusion_rationale = ?, updated_at = ?
WHERE id = ?`,
		conclusion, string(confidence), rationale, now, id)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// ConclusionReviewSnapshot returns the private material-basis fingerprint.
func ConclusionReviewSnapshot(ctx context.Context, q Querier, id int64) (string, error) {
	var snapshot string
	err := q.QueryRowContext(ctx, `SELECT conclusion_review_snapshot FROM investigations WHERE id = ?`, id).Scan(&snapshot)
	return snapshot, err
}

// SetConclusionAccountability persists the statement label and selected review basis.
func SetConclusionAccountability(ctx context.Context, q Querier, inv domain.Investigation, snapshot string) error {
	supporting, err := json.Marshal(inv.ConclusionSupportingClaims)
	if err != nil {
		return err
	}
	contradicting, err := json.Marshal(inv.ConclusionContradictingClaims)
	if err != nil {
		return err
	}
	res, err := q.ExecContext(ctx, `UPDATE investigations SET conclusion_verification = ?, conclusion_supporting_claims = ?, conclusion_contradicting_claims = ?, conclusion_reviewed_at = ?, conclusion_review_basis = ?, conclusion_review_snapshot = ? WHERE id = ?`, string(inv.ConclusionVerification), string(supporting), string(contradicting), inv.ConclusionReviewedAt, inv.ConclusionReviewBasis, snapshot, inv.ID)
	if err != nil {
		return err
	}
	return expectOne(res)
}
