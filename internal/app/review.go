package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// ReviewEvidence keeps a source and its relationship visible together.
type ReviewEvidence struct {
	Relation domain.Relation `json:"relation"`
	Evidence Evidence        `json:"evidence"`
}

// ReviewFinding is one current statement with the sources a person reviews.
// Basis guards the entire displayed snapshot, including evidence raw detail.
type ReviewFinding struct {
	Kind         domain.RecordType `json:"kind"`
	Ref          string            `json:"ref"`
	Statement    string            `json:"statement"`
	Confidence   domain.Confidence `json:"confidence"`
	Rationale    string            `json:"rationale"`
	Status       string            `json:"status"`
	Verification Verification      `json:"verification"`
	Evidence     []ReviewEvidence  `json:"evidence"`
	Filters      []string          `json:"filters"`
	Warnings     []string          `json:"warnings"`
	Basis        string            `json:"basis"`
}

type ReviewStatementInput struct {
	Kind          domain.RecordType `json:"kind"`
	Ref           string            `json:"ref"`
	Basis         string            `json:"basis"`
	Verification  Verification      `json:"verification"`
	SettleDispute bool              `json:"settle_dispute"`
}

func reviewKind(kind domain.RecordType) (stmtKind, error) {
	switch kind {
	case domain.RecordClaim:
		return claimKind, nil
	case domain.RecordHypothesis:
		return hypothesisKind, nil
	default:
		return stmtKind{}, domain.NewValidationError("kind", "must be claim or hypothesis")
	}
}

func reviewCurrent(row store.StatementRow) bool {
	return row.Status != "superseded" && row.Status != "rejected" && row.Status != "disproven"
}

func reviewFinding(k stmtKind, row store.StatementRow, evidence map[string]Evidence) ReviewFinding {
	out := ReviewFinding{Kind: k.rec, Ref: row.Ref, Statement: row.Statement, Confidence: row.Confidence,
		Rationale: row.Rationale, Status: row.Status, Verification: row.Verification,
		Evidence: []ReviewEvidence{}, Filters: []string{}, Warnings: []string{}}
	switch row.Verification {
	case domain.VerificationAIGenerated:
		out.Filters = append(out.Filters, "unchecked")
	case domain.VerificationSourceVerified:
		out.Filters = append(out.Filters, "source_checked")
	case domain.VerificationDisputed:
		out.Filters = append(out.Filters, "disputed")
	}
	supports := 0
	for _, link := range row.Links {
		e, ok := evidence[link.EvidenceRef]
		if !ok {
			continue
		}
		out.Evidence = append(out.Evidence, ReviewEvidence{Relation: link.Relation, Evidence: e})
		if link.Relation == domain.RelationSupports {
			supports++
			switch e.Verification {
			case domain.VerificationDisputed, domain.VerificationSuperseded:
				out.Warnings = append(out.Warnings, fmt.Sprintf("Supporting evidence %s is %s.", e.Ref, e.Verification.Label()))
			}
		}
	}
	if supports == 0 {
		out.Warnings = append(out.Warnings, "No supporting evidence is linked.")
	}
	if len(out.Warnings) > 0 {
		out.Filters = append(out.Filters, "support_attention")
	}
	// The row includes link identity and attribution; sources include content,
	// source reference and verification. Timestamps alone are not a guard.
	data, _ := json.Marshal(struct {
		Row      store.StatementRow
		Evidence []ReviewEvidence
	}{row, out.Evidence})
	sum := sha256.Sum256(data)
	out.Basis = hex.EncodeToString(sum[:])
	return out
}

// GetReviewFindings reads a coherent current queue. Retired records remain in
// the case and are never reactivated by review.
func (a *App) GetReviewFindings(ctx context.Context, invRef string) ([]ReviewFinding, error) {
	out := []ReviewFinding{}
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		inv, err := a.resolveInvestigation(ctx, tx, invRef)
		if err != nil {
			return err
		}
		evidence, err := store.ListEvidenceRaw(ctx, tx, inv.ID, true)
		if err != nil {
			return err
		}
		byRef := map[string]Evidence{}
		for _, e := range evidence {
			byRef[e.Ref] = e
		}
		for _, k := range []stmtKind{claimKind, hypothesisKind} {
			rows, err := store.ListStatements(ctx, tx, k.target, inv.ID)
			if err != nil {
				return err
			}
			for _, row := range rows {
				if reviewCurrent(row) {
					out = append(out, reviewFinding(k, row, byRef))
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, a.fail("review.list", err)
	}
	return out, nil
}

// ReviewStatement checks the displayed snapshot and applies the existing
// statement mutation in the same write transaction. No stale view can vouch
// for an AI edit that arrived before the person clicked Verify myself.
func (a *App) ReviewStatement(ctx context.Context, actor Actor, invRef string, in ReviewStatementInput) (ReviewFinding, error) {
	const op = "review.statement"
	actor, err := actor.Validate()
	if err != nil {
		return ReviewFinding{}, a.fail(op, err)
	}
	if actor.Kind != domain.ActorHuman {
		return ReviewFinding{}, a.fail(op, domain.NewValidationError("actor", "only a person can review findings"))
	}
	k, err := reviewKind(in.Kind)
	if err != nil {
		return ReviewFinding{}, a.fail(op, err)
	}
	if in.Verification != domain.VerificationHumanVerified && in.Verification != domain.VerificationDisputed {
		return ReviewFinding{}, a.fail(op, domain.NewValidationError("verification", "must be human_verified or disputed"))
	}
	pp, err := k.preparePatch(actor, stmtPatch{Verification: &in.Verification}, "superseded_by")
	if err != nil {
		return ReviewFinding{}, a.fail(op, err)
	}
	var out ReviewFinding
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		t, row, err := a.getStatement(ctx, tx, k, invRef, in.Ref)
		if err != nil {
			return err
		}
		if !reviewCurrent(row) {
			return fmt.Errorf("%w: %s is retired; review current findings instead", domain.ErrConflict, row.Ref)
		}
		evidence, err := store.ListEvidenceRaw(ctx, tx, t.Inv.ID, true)
		if err != nil {
			return err
		}
		byRef := map[string]Evidence{}
		for _, e := range evidence {
			byRef[e.Ref] = e
		}
		current := reviewFinding(k, row, byRef)
		if in.Basis == "" || in.Basis != current.Basis {
			return fmt.Errorf("%w: this finding or its evidence changed; show the latest version before reviewing", domain.ErrConflict)
		}
		if row.Verification == domain.VerificationDisputed && in.Verification == domain.VerificationHumanVerified && !in.SettleDispute {
			return domain.NewValidationError("settle_dispute", "confirm that you want to settle this dispute")
		}
		updated, _, err := a.updateStatementTx(ctx, tx, actor, invRef, in.Ref, k, pp)
		if err != nil {
			return err
		}
		out = reviewFinding(k, updated, byRef)
		return nil
	})
	if err != nil {
		return ReviewFinding{}, a.fail(op, err)
	}
	return out, nil
}
