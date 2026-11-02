package contextpack

import (
	"slices"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// oneLineBytes caps the one-line text of disproven hypotheses.
const oneLineBytes = 200

// nonMaterial lists activity actions that never appear in recent changes:
// they record navigation, notes or posts to Evernote, not investigation
// state. The Evernote integration is gone, but "evernote.post" and
// "evernote.unlink" stay so History rows recorded by older versions keep
// rendering the same way and never count as material changes.
var nonMaterial = []string{"active.set", "active.clear", "active.bind", "active.unbind", "checkpoint",
	"evernote.post", "evernote.unlink"}

// Build turns a snapshot into a pack (D-25):
//
//   - active claims become current claims and disputed claims a compact
//     disputed section; superseded and rejected claims are left out;
//   - open and supported hypotheses are listed with their status, disproven
//     ones only as one-liners with the evidence that contradicted them,
//     superseded ones not at all;
//   - open questions are listed, deferred ones compactly with their note,
//     resolved ones not at all;
//   - supporting and contradicting evidence refs come from each record's
//     links; the evidence index holds only evidence those included claims
//     and hypotheses reference, plus the evidence contradicting a disproven
//     hypothesis;
//   - recent changes are the newest RecentChanges (default 10) material
//     activity rows, newest first.
//
// Inputs are copied and sorted by ref number (activity by id), never trusted
// for order. When o.MaxBytes > 0 or o.Compact, the size budget is applied
// (see applyBudget).
func Build(s Snapshot, o Options) Pack {
	inv := s.Investigation
	if inv.ConclusionVerification == "" {
		inv.ConclusionVerification = domain.VerificationAIGenerated
	}
	p := Pack{
		Format:  Format,
		Version: Version,
		Investigation: PackInvestigation{
			Ref:             inv.Ref,
			Title:           inv.Title,
			Status:          inv.Status,
			PrimaryQuestion: inv.ProblemStatement,
		},
		Conclusion: PackConclusion{
			Text:                inv.Conclusion,
			Confidence:          inv.ConclusionConfidence,
			Rationale:           inv.ConclusionRationale,
			Verification:        inv.ConclusionVerification,
			SupportingClaims:    conclusionClaimBasis(inv.ConclusionSupportingClaims, s.Claims),
			ContradictingClaims: conclusionClaimBasis(inv.ConclusionContradictingClaims, s.Claims),
			ReviewedAt:          inv.ConclusionReviewedAt,
			ReviewBasis:         inv.ConclusionReviewBasis,
			NeedsReview:         inv.ConclusionNeedsReview || (inv.Conclusion != "" && inv.ConclusionReviewedAt == ""),
		},
		EstablishedClaims:    []PackClaim{},
		DisputedClaims:       []PackClaim{},
		Hypotheses:           []PackHypothesis{},
		OpenQuestions:        []PackQuestion{},
		DeferredQuestions:    []PackQuestion{},
		DisprovenHypotheses:  []PackOneLine{},
		EvidenceIndex:        []PackEvidence{},
		EvidenceVerification: []PackEvidenceVerification{},
		RecentChanges:        []PackChange{},
		OmittedSections:      []string{},
	}

	referenced := []string{}

	claims := slices.Clone(s.Claims)
	sortByRef(claims, domain.RecordClaim, func(c domain.Claim) string { return c.Ref })
	for _, c := range claims {
		state := claimState(c)
		if state == "" {
			continue
		}
		supporting, contradicting := linkRefs(c.Links)
		referenced = append(referenced, supporting...)
		referenced = append(referenced, contradicting...)
		pc := PackClaim{
			Ref:                   c.Ref,
			Statement:             c.Statement,
			Confidence:            c.Confidence,
			Verification:          c.Verification,
			Rationale:             domain.TruncateLine(c.ConfidenceRationale, RationaleBytes),
			SupportingEvidence:    supporting,
			ContradictingEvidence: contradicting,
			Imported:              c.Imported,
		}
		if state == domain.ClaimStatusActive {
			p.EstablishedClaims = append(p.EstablishedClaims, pc)
		} else {
			p.DisputedClaims = append(p.DisputedClaims, pc)
		}
	}

	hyps := slices.Clone(s.Hypotheses)
	sortByRef(hyps, domain.RecordHypothesis, func(h domain.Hypothesis) string { return h.Ref })
	for _, h := range hyps {
		if h.Verification == domain.VerificationSuperseded {
			continue // superseded on either axis (ADR-0013)
		}
		switch h.Status {
		case domain.HypothesisStatusOpen, domain.HypothesisStatusSupported:
			supporting, contradicting := linkRefs(h.Links)
			referenced = append(referenced, supporting...)
			referenced = append(referenced, contradicting...)
			p.Hypotheses = append(p.Hypotheses, PackHypothesis{
				Ref:                   h.Ref,
				Statement:             h.Statement,
				Confidence:            h.Confidence,
				Status:                h.Status,
				Verification:          h.Verification,
				Rationale:             domain.TruncateLine(h.Rationale, RationaleBytes),
				SupportingEvidence:    supporting,
				ContradictingEvidence: contradicting,
				Imported:              h.Imported,
			})
		case domain.HypothesisStatusDisproven:
			_, contradicting := linkRefs(h.Links)
			referenced = append(referenced, contradicting...)
			p.DisprovenHypotheses = append(p.DisprovenHypotheses, PackOneLine{
				Ref:                   h.Ref,
				Text:                  domain.TruncateLine(h.Statement, oneLineBytes),
				ContradictingEvidence: contradicting,
				Imported:              h.Imported,
				Verification:          h.Verification,
			})
		}
	}

	questions := slices.Clone(s.Questions)
	sortByRef(questions, domain.RecordQuestion, func(q domain.Question) string { return q.Ref })
	for _, q := range questions {
		switch q.Status {
		case domain.QuestionStatusOpen:
			p.OpenQuestions = append(p.OpenQuestions, PackQuestion{Ref: q.Ref, Question: q.Question, Imported: q.Imported})
		case domain.QuestionStatusDeferred:
			p.DeferredQuestions = append(p.DeferredQuestions, PackQuestion{Ref: q.Ref, Question: q.Question, Note: q.DeferNote, Imported: q.Imported})
		}
	}

	evidence := slices.Clone(s.Evidence)
	sortByRef(evidence, domain.RecordEvidence, func(e domain.Evidence) string { return e.Ref })
	for _, e := range evidence {
		if !slices.Contains(referenced, e.Ref) {
			p.Omitted.EvidenceOutsideFindings++
			continue
		}
		if n := len(p.EvidenceIndex); n > 0 && p.EvidenceIndex[n-1].Ref == e.Ref {
			continue
		}
		p.EvidenceVerification = append(p.EvidenceVerification, PackEvidenceVerification{Ref: e.Ref, Verification: e.Verification})
		p.EvidenceIndex = append(p.EvidenceIndex, PackEvidence{
			Ref:          e.Ref,
			Verification: e.Verification,
			Title:        e.Title,
			SourceLabel:  e.SourceLabel,
			SourceType:   e.SourceType,
			Imported:     e.Imported,
		})
	}

	want := o.RecentChanges
	if want <= 0 {
		want = DefaultRecentChanges
	}
	activity := slices.Clone(s.Activity)
	slices.SortStableFunc(activity, func(a, b domain.Activity) int {
		switch {
		case a.ID > b.ID:
			return -1
		case a.ID < b.ID:
			return 1
		}
		return strings.Compare(b.CreatedAt, a.CreatedAt)
	})
	for _, a := range activity {
		if len(p.RecentChanges) == want {
			break
		}
		if slices.Contains(nonMaterial, a.Action) {
			continue
		}
		p.RecentChanges = append(p.RecentChanges, PackChange{
			At:        a.CreatedAt,
			Actor:     a.Actor,
			Action:    a.Action,
			TargetRef: a.TargetRef,
			Summary:   a.Summary,
			Imported:  a.Imported,
		})
	}

	limit := o.MaxBytes
	if limit <= 0 && o.Compact {
		limit = DefaultCompactMaxBytes
	}
	if limit > 0 {
		applyBudget(&p, limit)
	}
	return p
}

// claimState places a claim by both its status and its verification, so a
// claim never shows under Current claims while either axis says
// disputed, superseded or rejected (ADR-0013; the engine keeps the two axes
// in agreement, this guards snapshots that predate it). It returns active
// (current), disputed, or "" for claims the pack leaves out.
func claimState(c domain.Claim) domain.ClaimStatus {
	switch {
	case c.Status == domain.ClaimStatusSuperseded, c.Status == domain.ClaimStatusRejected,
		c.Verification == domain.VerificationSuperseded:
		return ""
	case c.Status == domain.ClaimStatusDisputed, c.Verification == domain.VerificationDisputed:
		return domain.ClaimStatusDisputed
	case c.Status == domain.ClaimStatusActive:
		return domain.ClaimStatusActive
	}
	return ""
}

// refKey orders refs of type t by number; unparsable refs sort last by text.
type refKey struct {
	bad bool
	num int64
	ref string
}

func keyOf(t domain.RecordType, ref string) refKey {
	rr, err := domain.ParseRecordRef(ref, t)
	if err != nil {
		return refKey{bad: true, ref: ref}
	}
	return refKey{num: rr.Num, ref: ref}
}

func compareKeys(a, b refKey) int {
	switch {
	case a.bad != b.bad:
		if a.bad {
			return 1
		}
		return -1
	case a.num < b.num:
		return -1
	case a.num > b.num:
		return 1
	}
	return strings.Compare(a.ref, b.ref)
}

// sortByRef sorts items in place by the ref number of type t.
func sortByRef[T any](items []T, t domain.RecordType, ref func(T) string) {
	slices.SortStableFunc(items, func(a, b T) int {
		return compareKeys(keyOf(t, ref(a)), keyOf(t, ref(b)))
	})
}

// sortRefs sorts and de-duplicates evidence refs by number.
func sortRefs(refs []string) []string {
	sortByRef(refs, domain.RecordEvidence, func(r string) string { return r })
	return slices.Compact(refs)
}

// linkRefs splits links into supporting and contradicting evidence refs,
// each sorted by ref number and non-nil.
func linkRefs(links []domain.EvidenceLink) (supporting, contradicting []string) {
	supporting, contradicting = []string{}, []string{}
	for _, l := range links {
		switch l.Relation {
		case domain.RelationSupports:
			supporting = append(supporting, l.EvidenceRef)
		case domain.RelationContradicts:
			contradicting = append(contradicting, l.EvidenceRef)
		}
	}
	return sortRefs(supporting), sortRefs(contradicting)
}

func conclusionClaimBasis(refs []string, claims []domain.Claim) []PackConclusionClaim {
	out := []PackConclusionClaim{}
	for _, ref := range refs {
		row := PackConclusionClaim{Ref: ref, Missing: true}
		for _, c := range claims {
			if c.Ref == ref {
				row.Status, row.Verification, row.Missing = c.Status, c.Verification, false
				break
			}
		}
		out = append(out, row)
	}
	sortByRef(out, domain.RecordClaim, func(c PackConclusionClaim) string { return c.Ref })
	return out
}
