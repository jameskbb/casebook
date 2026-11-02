package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// NewQuestion is the data needed to insert a question row (status open).
type NewQuestion struct {
	InvestigationID int64
	RefNum          int64
	Question        string
	CreatedAt       string
	CreatedBy       domain.Actor
}

// InsertQuestion inserts an open question and returns its id.
func InsertQuestion(ctx context.Context, q Querier, n NewQuestion) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `INSERT INTO questions
    (investigation_id, ref_num, question, status, created_at, updated_at, created_by_kind, created_by_client)
VALUES (?, ?, ?, 'open', ?, ?, ?, ?)
RETURNING id`,
		n.InvestigationID, n.RefNum, n.Question, n.CreatedAt, n.CreatedAt, string(n.CreatedBy.Kind),
		n.CreatedBy.Client).Scan(&id)
	return id, err
}

const questionSelect = `SELECT q.id, q.ref_num, i.ref_num, q.question, q.status, q.resolution, q.defer_note,
    COALESCE(q.resolved_at, ''), q.created_at, q.updated_at, q.created_by_kind, q.created_by_client, q.imported
FROM questions q JOIN investigations i ON i.id = q.investigation_id`

func scanQuestion(row interface{ Scan(...any) error }) (domain.Question, error) {
	var (
		out            domain.Question
		refNum, invNum int64
		status, kind   string
	)
	if err := row.Scan(&out.ID, &refNum, &invNum, &out.Question, &status, &out.Resolution, &out.DeferNote,
		&out.ResolvedAt, &out.CreatedAt, &out.UpdatedAt, &kind, &out.CreatedBy.Client, &out.Imported); err != nil {
		return domain.Question{}, err
	}
	out.Ref = domain.FormatRecordRef(domain.RecordQuestion, refNum)
	out.InvestigationRef = domain.FormatInvestigationRef(invNum)
	out.Status = domain.QuestionStatus(status)
	out.CreatedBy.Kind = domain.ActorKind(kind)
	return out, nil
}

// GetQuestion returns question refNum of investigation invID, or an error
// matching sql.ErrNoRows.
func GetQuestion(ctx context.Context, q Querier, invID, refNum int64) (domain.Question, error) {
	return scanQuestion(q.QueryRowContext(ctx, questionSelect+` WHERE q.investigation_id = ? AND q.ref_num = ?`,
		invID, refNum))
}

// GetQuestionByID returns the question with primary key id, or an error
// matching sql.ErrNoRows.
func GetQuestionByID(ctx context.Context, q Querier, id int64) (domain.Question, error) {
	return scanQuestion(q.QueryRowContext(ctx, questionSelect+` WHERE q.id = ?`, id))
}

// ListQuestions returns every question of investigation invID ordered by ref
// number. The slice is never nil.
func ListQuestions(ctx context.Context, q Querier, invID int64) ([]domain.Question, error) {
	rows, err := q.QueryContext(ctx, questionSelect+` WHERE q.investigation_id = ? ORDER BY q.ref_num`, invID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Question{}
	for rows.Next() {
		qq, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, qq)
	}
	return out, rows.Err()
}

// questionUpdatable whitelists the columns UpdateQuestion may set.
// resolved_at takes a string or nil (NULL).
var questionUpdatable = map[string]bool{
	"question": true, "status": true, "resolution": true, "defer_note": true, "resolved_at": true,
}

// UpdateQuestion sets the given columns and updated_at on question id. Only
// whitelisted columns are accepted, so the search index is rewritten only
// when question or resolution is among them.
func UpdateQuestion(ctx context.Context, q Querier, id int64, set []Assignment, now string) error {
	parts := make([]string, 0, len(set)+1)
	args := make([]any, 0, len(set)+2)
	for _, a := range set {
		if !questionUpdatable[a.Column] {
			return fmt.Errorf("update questions: column %q is not updatable", a.Column)
		}
		parts = append(parts, a.Column+" = ?")
		args = append(args, a.Value)
	}
	parts = append(parts, "updated_at = ?")
	args = append(args, now, id)
	res, err := q.ExecContext(ctx, `UPDATE questions SET `+strings.Join(parts, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return err
	}
	return expectOne(res)
}
