package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// Conclusion attention is a direct comparison of selected claims and their
// evidence, on the caller's transaction. It never follows other conclusions.
func (a *App) markConclusionAttention(ctx context.Context, q store.Querier, inv *domain.Investigation) error {
	if inv.Conclusion == "" {
		return nil
	}
	current, err := conclusionBasisSnapshot(ctx, q, *inv)
	if err != nil {
		return err
	}
	inv.ConclusionReviewToken = conclusionReviewToken(*inv, current)
	inv.ConclusionNeedsReview = inv.ConclusionReviewedAt == ""
	if inv.ConclusionReviewedAt == "" {
		return nil
	}
	old, err := store.ConclusionReviewSnapshot(ctx, q, inv.ID)
	if err != nil {
		return err
	}
	inv.ConclusionNeedsReview = inv.ConclusionVerification == domain.VerificationDisputed || old == "" || old != current
	return nil
}

func canonicalConclusionRefs(inv domain.Investigation, field string, refs []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	invNum, _ := domain.ParseInvestigationRef(inv.Ref)
	if len(refs) > 100 {
		return nil, domain.NewValidationError(field, "at most 100 selected claims are allowed")
	}
	for i, ref := range refs {
		rr, err := domain.ParseRecordRef(ref, domain.RecordClaim)
		if err != nil {
			return nil, refieldValidation(err, fmt.Sprintf("%s[%d]", field, i))
		}
		if rr.InvestigationNum != 0 && rr.InvestigationNum != invNum {
			return nil, domain.NewValidationError(field, "%s belongs to another investigation", ref)
		}
		canonical := domain.FormatRecordRef(domain.RecordClaim, rr.Num)
		if !seen[canonical] {
			out = append(out, canonical)
			seen[canonical] = true
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := domain.ParseRecordRef(out[i], domain.RecordClaim)
		b, _ := domain.ParseRecordRef(out[j], domain.RecordClaim)
		return a.Num < b.Num
	})
	return out, nil
}

func (a *App) prepareConclusionAccountability(ctx context.Context, q store.Querier, actor domain.Actor, inv domain.Investigation, c preparedConclusion, now string) (domain.Investigation, string, bool, error) {
	next := inv
	var err error
	if c.supporting != nil {
		next.ConclusionSupportingClaims, err = canonicalConclusionRefs(inv, "supporting_claims", c.supporting)
		if err != nil {
			return next, "", false, err
		}
	}
	if c.contradicting != nil {
		next.ConclusionContradictingClaims, err = canonicalConclusionRefs(inv, "contradicting_claims", c.contradicting)
		if err != nil {
			return next, "", false, err
		}
	}
	for _, s := range next.ConclusionSupportingClaims {
		for _, c := range next.ConclusionContradictingClaims {
			if s == c {
				return next, "", false, domain.NewValidationError("contradicting_claims", "%s cannot both support and contradict the conclusion", s)
			}
		}
	}
	// Existing refs remain inspectable when a selected claim is later deleted.
	// A new selection must point to records in this case.
	if c.supporting != nil || c.contradicting != nil {
		for _, ref := range append(append([]string{}, next.ConclusionSupportingClaims...), next.ConclusionContradictingClaims...) {
			rr, _ := domain.ParseRecordRef(ref, domain.RecordClaim)
			if _, err := store.GetStatement(ctx, q, store.LinkClaim, inv.ID, rr.Num); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return next, "", false, domain.NewValidationError("supporting_claims", "claim %s does not exist in %s", ref, inv.Ref)
				}
				return next, "", false, err
			}
		}
	}
	statementChanged := inv.Conclusion != c.conclusion
	substanceChanged := statementChanged || inv.ConclusionConfidence != c.confidence || inv.ConclusionRationale != c.rationale || !reflect.DeepEqual(inv.ConclusionSupportingClaims, next.ConclusionSupportingClaims) || !reflect.DeepEqual(inv.ConclusionContradictingClaims, next.ConclusionContradictingClaims)
	if c.verification != "" {
		next.ConclusionVerification = c.verification
	} else if inv.Conclusion == "" && c.conclusion != "" {
		next.ConclusionVerification = domain.DefaultVerification(actor.Kind)
	} else if statementChanged && actor.Kind != domain.ActorHuman && (inv.ConclusionVerification == domain.VerificationHumanVerified || inv.ConclusionVerification == domain.VerificationSourceVerified) {
		next.ConclusionVerification = domain.VerificationAIGenerated
	}
	snapshot, err := store.ConclusionReviewSnapshot(ctx, q, inv.ID)
	if err != nil {
		return next, "", false, err
	}
	if substanceChanged {
		next.ConclusionReviewedAt, next.ConclusionReviewBasis, snapshot = "", "", ""
	}
	if c.conclusion == "" {
		next.ConclusionVerification = domain.VerificationAIGenerated
		next.ConclusionSupportingClaims = []string{}
		next.ConclusionContradictingClaims = []string{}
		next.ConclusionReviewedAt, next.ConclusionReviewBasis, snapshot = "", "", ""
	}
	if c.review {
		if c.expectedReviewToken == "" || c.expectedReviewToken != inv.ConclusionReviewToken {
			return next, "", false, fmt.Errorf("%w: conclusion or selected claims and evidence changed; reload and review the current basis", domain.ErrConflict)
		}
		if !reflect.DeepEqual(inv.ConclusionSupportingClaims, next.ConclusionSupportingClaims) || !reflect.DeepEqual(inv.ConclusionContradictingClaims, next.ConclusionContradictingClaims) {
			return next, "", false, domain.NewValidationError("review", "save selected claims before recording their review")
		}
		next.ConclusionReviewedAt, next.ConclusionReviewBasis = now, c.reviewBasis
		snapshot, err = conclusionBasisSnapshot(ctx, q, next)
		if err != nil {
			return next, "", false, err
		}
	}
	changed := substanceChanged || inv.ConclusionVerification != next.ConclusionVerification || inv.ConclusionReviewedAt != next.ConclusionReviewedAt || inv.ConclusionReviewBasis != next.ConclusionReviewBasis || (c.review && inv.ConclusionNeedsReview)
	return next, snapshot, changed, nil
}

func conclusionBasisSnapshot(ctx context.Context, q store.Querier, inv domain.Investigation) (string, error) {
	// Strip ids, administrative timestamps and authors: a touch or activity row is not a
	// material change. Keep statement, confidence, status, verification,
	// replacement, link relation and the linked observation/source fields.
	type evidenceBasis struct {
		Ref                                string
		Relation                           domain.Relation
		Title, Summary, RawDetail          string
		SourceType                         domain.SourceType
		SourceRef, SourceLabel, CapturedAt string
		Verification                       domain.Verification
		Metadata                           string
	}
	type claimBasis struct {
		Ref, Role, Statement, Rationale, Status, SupersededBy string
		Confidence                                            domain.Confidence
		Verification                                          domain.Verification
		Missing                                               bool
		Evidence                                              []evidenceBasis
	}
	basis := []claimBasis{}
	for _, group := range []struct {
		role string
		refs []string
	}{{"supports", inv.ConclusionSupportingClaims}, {"contradicts", inv.ConclusionContradictingClaims}} {
		for _, ref := range group.refs {
			rr, err := domain.ParseRecordRef(ref, domain.RecordClaim)
			if err != nil {
				return "", err
			}
			row, err := store.GetStatement(ctx, q, store.LinkClaim, inv.ID, rr.Num)
			c := claimBasis{Ref: ref, Role: group.role}
			if errors.Is(err, sql.ErrNoRows) {
				c.Missing = true
				basis = append(basis, c)
				continue
			}
			if err != nil {
				return "", err
			}
			c.Statement, c.Rationale, c.Status, c.SupersededBy, c.Confidence, c.Verification = row.Statement, row.Rationale, row.Status, row.SupersededByRef, row.Confidence, row.Verification
			for _, link := range row.Links {
				er, _ := domain.ParseRecordRef(link.EvidenceRef, domain.RecordEvidence)
				e, err := store.GetEvidence(ctx, q, inv.ID, er.Num, true)
				if err != nil {
					return "", err
				}
				c.Evidence = append(c.Evidence, evidenceBasis{e.Ref, link.Relation, e.Title, e.Summary, e.RawDetail, e.SourceType, e.SourceRef, e.SourceLabel, e.CapturedAt, e.Verification, e.Metadata})
			}
			basis = append(basis, c)
		}
	}
	data, err := json.Marshal(basis)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func conclusionReviewToken(inv domain.Investigation, basis string) string {
	data, _ := json.Marshal([]string{inv.Ref, inv.Conclusion, string(inv.ConclusionConfidence), inv.ConclusionRationale, string(inv.ConclusionVerification), basis})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
