package app

import (
	"context"
	"database/sql"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// Read-model limits.
const (
	defaultDetailActivity = 20
	maxDetailActivity     = 200
	defaultListLimit      = 100
	maxListLimit          = 1000
)

// GetInvestigationDetail returns the investigation (empty invRef means the
// active one) with its evidence, claims and hypotheses (each with links),
// questions and the newest ActivityLimit activity rows, all read from one
// snapshot. Evidence raw_detail is returned only with IncludeRawDetail;
// source content is read internally to guard a person's later verification.
// ActivityLimit 0 means 20; a negative value or one above 200 is a
// ValidationError on activity_limit. Every slice is non-nil.
func (a *App) GetInvestigationDetail(ctx context.Context, invRef string, o DetailOptions) (InvestigationDetail, error) {
	const op = "investigation.detail"
	limit := o.ActivityLimit
	if limit < 0 || limit > maxDetailActivity {
		return InvestigationDetail{}, a.fail(op,
			domain.NewValidationError("activity_limit", "must be between 0 and %d", maxDetailActivity))
	}
	if limit == 0 {
		limit = defaultDetailActivity
	}
	var out InvestigationDetail
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		inv, err := a.resolveInvestigation(ctx, tx, invRef)
		if err != nil {
			return err
		}
		d := InvestigationDetail{Investigation: inv, ReviewBases: []domain.RecordReviewBasis{}}
		if d.Evidence, err = store.ListEvidenceRaw(ctx, tx, inv.ID, true); err != nil {
			return err
		}
		claims, err := store.ListStatements(ctx, tx, store.LinkClaim, inv.ID)
		if err != nil {
			return err
		}
		d.Claims = store.Claims(claims)
		hyps, err := store.ListStatements(ctx, tx, store.LinkHypothesis, inv.ID)
		if err != nil {
			return err
		}
		d.Hypotheses = store.Hypotheses(hyps)
		byRef := map[string]Evidence{}
		for _, e := range d.Evidence {
			byRef[e.Ref] = e
		}
		for _, group := range []struct {
			kind stmtKind
			rows []store.StatementRow
		}{{claimKind, claims}, {hypothesisKind, hyps}} {
			for _, row := range group.rows {
				if reviewCurrent(row) {
					d.ReviewBases = append(d.ReviewBases, domain.RecordReviewBasis{Ref: row.Ref, Basis: reviewFinding(group.kind, row, byRef).Basis})
				}
			}
		}
		if !o.IncludeRawDetail {
			for i := range d.Evidence {
				d.Evidence[i].RawDetail = ""
			}
		}
		if d.Questions, err = store.ListQuestions(ctx, tx, inv.ID); err != nil {
			return err
		}
		if d.RecentActivity, err = store.ListActivity(ctx, tx, store.ActivityQuery{InvestigationID: inv.ID, Limit: limit}); err != nil {
			return err
		}
		out = d
		return nil
	})
	if err != nil {
		return InvestigationDetail{}, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", out.Investigation.Ref)
	return out, nil
}

// ListInvestigations returns investigation summaries (ref, title, status,
// conclusion confidence, timestamps, archived and active flags, record
// counts) newest updated first. Archived investigations are hidden unless
// IncludeArchived is true or Status is archived. Status "" means any status;
// an unknown status is a ValidationError on status. Limit 0 means 100; a
// negative value or one above 1000 is a ValidationError on limit. The slice
// is never nil.
func (a *App) ListInvestigations(ctx context.Context, o ListOptions) ([]InvestigationSummary, error) {
	const op = "investigation.list"
	q := store.SummaryQuery{IncludeArchived: o.IncludeArchived, Limit: o.Limit}
	if o.Status != "" {
		st := domain.InvestigationStatus(o.Status)
		if err := st.Validate("status"); err != nil {
			return nil, a.fail(op, err)
		}
		q.Status = st
		if st == domain.InvestigationStatusArchived {
			q.IncludeArchived = true
		}
	}
	if q.Limit < 0 || q.Limit > maxListLimit {
		return nil, a.fail(op, domain.NewValidationError("limit", "must be between 0 and %d", maxListLimit))
	}
	if q.Limit == 0 {
		q.Limit = defaultListLimit
	}
	var out []InvestigationSummary
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		list, err := store.ListInvestigationSummaries(ctx, tx, q)
		if err != nil {
			return err
		}
		if len(a.workDirs) > 0 {
			id, ok, _, err := a.activeID(ctx, tx)
			if err != nil {
				return err
			}
			for i := range list {
				list[i].IsActive = ok && list[i].ID == id
			}
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, a.fail(op, err)
	}
	a.log.Debug(op, "rows", len(out))
	return out, nil
}
