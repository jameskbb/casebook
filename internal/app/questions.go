package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// Questions move between open, resolved and deferred (D-12). Resolving needs
// a resolution and sets resolved_at; deferring takes an optional note;
// reopening clears resolution, note and resolved_at. Activity summaries name
// the ref and a truncated copy of the text.

// questionSummaryText bounds the question or resolution text copied into an
// activity summary.
const questionSummaryText = 200

func cleanQuestion(s string) (string, error) {
	return domain.SanitizeText("question", s, domain.MaxQuestionBytes, true)
}

// AddQuestion records an open question under the next Q ref of the
// investigation (empty invRef means the active one) and records question.add
// ("Added question Q-001: Is ...").
func (a *App) AddQuestion(ctx context.Context, actor Actor, invRef string, in QuestionInput) (Question, error) {
	const op = "question.add"
	actor, err := actor.Validate()
	if err != nil {
		return Question{}, a.fail(op, err)
	}
	text, err := cleanQuestion(in.Question)
	if err != nil {
		return Question{}, a.fail(op, err)
	}
	var out Question
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		q, err := a.addQuestionTx(ctx, tx, actor, invRef, text)
		out = q
		return err
	})
	if err != nil {
		return Question{}, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", out.InvestigationRef, "ref", out.Ref)
	return out, nil
}

// addQuestionTx inserts an open question with sanitized text under the next
// Q ref and records question.add.
func (a *App) addQuestionTx(ctx context.Context, tx *sql.Tx, actor domain.Actor, invRef, text string) (Question, error) {
	now := a.timestamp()
	inv, err := a.resolveInvestigation(ctx, tx, invRef)
	if err != nil {
		return Question{}, err
	}
	num, err := store.AllocateRecordNum(ctx, tx, inv.ID, domain.RecordQuestion)
	if err != nil {
		return Question{}, err
	}
	ref := domain.FormatRecordRef(domain.RecordQuestion, num)
	id, err := store.InsertQuestion(ctx, tx, store.NewQuestion{
		InvestigationID: inv.ID, RefNum: num, Question: text, CreatedAt: now, CreatedBy: actor,
	})
	if err != nil {
		return Question{}, err
	}
	if err := a.recordActivity(ctx, tx, actor, inv.ID, "question.add", ref,
		fmt.Sprintf("Added question %s: %s", ref, domain.TruncateLine(text, questionSummaryText)), "", now); err != nil {
		return Question{}, err
	}
	if err := store.TouchInvestigation(ctx, tx, inv.ID, now); err != nil {
		return Question{}, err
	}
	return store.GetQuestionByID(ctx, tx, id)
}

// getQuestion resolves and loads a question, mapping a missing row to
// ErrNotFound naming the ref.
func (a *App) getQuestion(ctx context.Context, q store.Querier, invRef, qRef string) (recordTarget, domain.Question, error) {
	t, err := a.resolveRecord(ctx, q, invRef, qRef, domain.RecordQuestion)
	if err != nil {
		return recordTarget{}, domain.Question{}, err
	}
	qq, err := store.GetQuestion(ctx, q, t.Inv.ID, t.Num)
	if errors.Is(err, sql.ErrNoRows) {
		return recordTarget{}, domain.Question{}, t.notFound()
	}
	return t, qq, err
}

// GetQuestion returns one question. qRef may be qualified (INV-0001/Q-001).
func (a *App) GetQuestion(ctx context.Context, invRef, qRef string) (Question, error) {
	var out Question
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		_, q, err := a.getQuestion(ctx, tx, invRef, qRef)
		if err != nil {
			return err
		}
		out = q
		return nil
	})
	if err != nil {
		return Question{}, a.fail("question.get", err)
	}
	return out, nil
}

// ListQuestions returns the investigation's questions ordered by ref, in
// every status. The slice is never nil.
func (a *App) ListQuestions(ctx context.Context, invRef string) ([]Question, error) {
	var out []Question
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		inv, err := a.resolveInvestigation(ctx, tx, invRef)
		if err != nil {
			return err
		}
		list, err := store.ListQuestions(ctx, tx, inv.ID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, a.fail("question.list", err)
	}
	return out, nil
}

// questionTarget is the desired state of a question after a change; nil
// fields are left alone. resolvedAt "" means NULL.
type questionTarget struct {
	question, status, resolution, deferNote, resolvedAt *string
}

// diff returns the assignments that move cur to the target.
func (qt questionTarget) diff(cur domain.Question) []store.Assignment {
	var set []store.Assignment
	for _, f := range []struct {
		column string
		want   *string
		have   string
	}{
		{"question", qt.question, cur.Question},
		{"status", qt.status, string(cur.Status)},
		{"resolution", qt.resolution, cur.Resolution},
		{"defer_note", qt.deferNote, cur.DeferNote},
		{"resolved_at", qt.resolvedAt, cur.ResolvedAt},
	} {
		if f.want == nil || *f.want == f.have {
			continue
		}
		var v any = *f.want
		if f.column == "resolved_at" && *f.want == "" {
			v = nil
		}
		set = append(set, store.Assignment{Column: f.column, Value: v})
	}
	return set
}

// questionPlan returns the target state of a question change (given the
// current question and the transaction time) and the activity summary.
type questionPlan func(cur domain.Question, now string) (questionTarget, string)

// changeQuestion runs one question mutation in a write transaction (see
// changeQuestionTx).
func (a *App) changeQuestion(ctx context.Context, actor Actor, invRef, qRef, op string, plan questionPlan) (Question, error) {
	actor, err := actor.Validate()
	if err != nil {
		return Question{}, a.fail(op, err)
	}
	var out Question
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		q, _, err := a.changeQuestionTx(ctx, tx, actor, invRef, qRef, op, plan)
		out = q
		return err
	})
	if err != nil {
		return Question{}, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", out.InvestigationRef, "ref", out.Ref)
	return out, nil
}

// changeQuestionTx applies plan inside tx and reports whether anything
// changed. When nothing changes it returns the question with no activity.
func (a *App) changeQuestionTx(ctx context.Context, tx *sql.Tx, actor domain.Actor, invRef, qRef, op string,
	plan questionPlan) (Question, bool, error) {
	now := a.timestamp()
	t, cur, err := a.getQuestion(ctx, tx, invRef, qRef)
	if err != nil {
		return Question{}, false, err
	}
	target, summary := plan(cur, now)
	set := target.diff(cur)
	if len(set) == 0 {
		return cur, false, nil
	}
	if err := store.UpdateQuestion(ctx, tx, cur.ID, set, now); err != nil {
		return Question{}, false, err
	}
	if err := a.recordActivity(ctx, tx, actor, t.Inv.ID, op, t.Ref, summary, "", now); err != nil {
		return Question{}, false, err
	}
	if err := store.TouchInvestigation(ctx, tx, t.Inv.ID, now); err != nil {
		return Question{}, false, err
	}
	q, err := store.GetQuestionByID(ctx, tx, cur.ID)
	return q, true, err
}

func strp(s string) *string { return &s }

// UpdateQuestion replaces the question text and records question.update. The
// status is unchanged. Unchanged text is a no-op with no activity.
func (a *App) UpdateQuestion(ctx context.Context, actor Actor, invRef, qRef, text string) (Question, error) {
	const op = "question.update"
	clean, err := cleanQuestion(text)
	if err != nil {
		return Question{}, a.fail(op, err)
	}
	return a.changeQuestion(ctx, actor, invRef, qRef, op, func(cur domain.Question, _ string) (questionTarget, string) {
		return questionTarget{question: &clean},
			fmt.Sprintf("Updated question %s: %s", cur.Ref, domain.TruncateLine(clean, questionSummaryText))
	})
}

// ResolveQuestion resolves the question with a required resolution (status
// resolved, resolved_at now, defer note cleared) and records question.resolve.
// Resolving an already resolved question with new text replaces the
// resolution and resolved_at; the same text is a no-op.
func (a *App) ResolveQuestion(ctx context.Context, actor Actor, invRef, qRef, resolution string) (Question, error) {
	const op = "question.resolve"
	res, err := domain.SanitizeText("resolution", resolution, domain.MaxResolutionBytes, true)
	if err != nil {
		return Question{}, a.fail(op, err)
	}
	return a.changeQuestion(ctx, actor, invRef, qRef, op, resolvePlan(res))
}

// resolvePlan resolves with the sanitized resolution res.
func resolvePlan(res string) questionPlan {
	return func(cur domain.Question, now string) (questionTarget, string) {
		summary := fmt.Sprintf("Resolved question %s: %s", cur.Ref, domain.TruncateLine(res, questionSummaryText))
		if cur.Status == domain.QuestionStatusResolved && cur.Resolution == res {
			return questionTarget{}, summary
		}
		return questionTarget{
			status: strp(string(domain.QuestionStatusResolved)), resolution: &res,
			deferNote: strp(""), resolvedAt: &now,
		}, summary
	}
}

// DeferQuestion defers the question with an optional note (status deferred,
// resolution and resolved_at cleared) and records question.defer. Deferring
// again with the same note is a no-op.
func (a *App) DeferQuestion(ctx context.Context, actor Actor, invRef, qRef, note string) (Question, error) {
	const op = "question.defer"
	clean, err := domain.SanitizeText("note", note, domain.MaxDeferNoteBytes, false)
	if err != nil {
		return Question{}, a.fail(op, err)
	}
	return a.changeQuestion(ctx, actor, invRef, qRef, op, deferPlan(clean))
}

// deferPlan defers with the sanitized note clean.
func deferPlan(clean string) questionPlan {
	return func(cur domain.Question, _ string) (questionTarget, string) {
		summary := "Deferred question " + cur.Ref
		if clean != "" {
			summary += ": " + domain.TruncateLine(clean, questionSummaryText)
		}
		return questionTarget{
			status: strp(string(domain.QuestionStatusDeferred)), deferNote: &clean,
			resolution: strp(""), resolvedAt: strp(""),
		}, summary
	}
}

// ReopenQuestion sets the question back to open, clearing resolution, defer
// note and resolved_at, and records question.reopen. Reopening an open
// question is a no-op with no activity.
func (a *App) ReopenQuestion(ctx context.Context, actor Actor, invRef, qRef string) (Question, error) {
	const op = "question.reopen"
	return a.changeQuestion(ctx, actor, invRef, qRef, op, reopenPlan)
}

// reopenPlan reopens a question.
func reopenPlan(cur domain.Question, _ string) (questionTarget, string) {
	return questionTarget{
		status: strp(string(domain.QuestionStatusOpen)), resolution: strp(""),
		deferNote: strp(""), resolvedAt: strp(""),
	}, "Reopened question " + cur.Ref
}
