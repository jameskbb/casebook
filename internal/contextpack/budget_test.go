package contextpack

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/domain"
)

var dropOrder = []string{"recent_changes", "statement_rationales", "evidence_index", "deferred_questions", "rationale", "disputed_claims", "disproven_hypotheses"}

var headerParts = []string{
	"Investigation: INV-0007\n", "Title: Widget count mismatch\n", "Status: Investigating\n",
	"Primary question:\nWhy do the two widget counts differ?\n",
	"Current conclusion:\nCounts use different cutoffs.\n", "Conclusion confidence:\nMedium\n",
}

// longSnapshot is testSnapshot with 400-byte statements and questions so
// shortening matters.
func longSnapshot() Snapshot {
	s := testSnapshot()
	long := func(tag string) string { return tag + " " + strings.Repeat("lengthy words ", 28) + "end." }
	for i := range s.Claims {
		s.Claims[i].Statement = long(s.Claims[i].Ref)
	}
	for i := range s.Hypotheses {
		s.Hypotheses[i].Statement = long(s.Hypotheses[i].Ref)
	}
	s.Questions[0].Question = long("Q-001")
	return s
}

// minimal reports whether the budget removed everything it may remove.
func minimal(p Pack) bool {
	return len(p.EstablishedClaims) == 0 && len(p.DisputedClaims) == 0 && len(p.Hypotheses) == 0 &&
		len(p.OpenQuestions) == 0 && len(p.DeferredQuestions) == 0 && len(p.DisprovenHypotheses) == 0 &&
		len(p.EvidenceIndex) == 0 && len(p.RecentChanges) == 0 && p.Conclusion.Rationale == "" && countRationales(&p) == 0
}

func longestText(p Pack) int {
	n := 0
	for _, c := range p.EstablishedClaims {
		n = max(n, len(c.Statement))
	}
	for _, c := range p.DisputedClaims {
		n = max(n, len(c.Statement))
	}
	for _, h := range p.Hypotheses {
		n = max(n, len(h.Statement))
	}
	for _, q := range p.OpenQuestions {
		n = max(n, len(q.Question))
	}
	return n
}

// anyShortened reports whether a budget-shortenable text ends in "...".
func anyShortened(p Pack) bool {
	var texts []string
	for _, c := range p.EstablishedClaims {
		texts = append(texts, c.Statement)
	}
	for _, c := range p.DisputedClaims {
		texts = append(texts, c.Statement)
	}
	for _, h := range p.Hypotheses {
		texts = append(texts, h.Statement)
	}
	for _, q := range p.OpenQuestions {
		texts = append(texts, q.Question)
	}
	for _, s := range texts {
		if strings.HasSuffix(s, "...") {
			return true
		}
	}
	return false
}

func TestBudgetDropOrderAndInvariants(t *testing.T) {
	for name, s := range map[string]Snapshot{"short": testSnapshot(), "long": longSnapshot()} {
		full := RenderMarkdown(Build(s, Options{}))
		prevOmitted := 0
		sawShortening, sawItemRemoval := false, false
		for limit := len(full) + 40; limit >= 1; limit -= 3 {
			p := Build(s, Options{MaxBytes: limit})
			md := RenderMarkdown(p)
			ctx := fmt.Sprintf("%s limit %d", name, limit)

			if p.MaxBytes != limit {
				t.Fatalf("%s: MaxBytes = %d", ctx, p.MaxBytes)
			}
			if n := len(p.OmittedSections); n > len(dropOrder) || !reflect.DeepEqual(p.OmittedSections, dropOrder[:n]) {
				t.Fatalf("%s: omitted %v is not a prefix of %v", ctx, p.OmittedSections, dropOrder)
			}
			if len(p.OmittedSections) < prevOmitted {
				t.Fatalf("%s: fewer sections omitted at a smaller budget", ctx)
			}
			prevOmitted = len(p.OmittedSections)
			for _, h := range headerParts {
				if !strings.Contains(md, h) {
					t.Fatalf("%s: header part %q missing", ctx, h)
				}
			}
			if len(md) > limit && !minimal(p) {
				t.Fatalf("%s: %d bytes over budget but not minimal", ctx, len(md))
			}
			if limit >= len(full) {
				if p.Truncated || md != full {
					t.Fatalf("%s: fits but was trimmed", ctx)
				}
				continue
			}
			if !p.Truncated || !strings.Contains(md, fmt.Sprintf("(Trimmed to fit %d bytes", limit)) {
				t.Fatalf("%s: trimmed pack lacks the footer:\n%s", ctx, md)
			}

			// Statements are shortened only after the five optional
			// sections are gone, disputed claims are dropped only after
			// shortening to 60 bytes, and disproven hypotheses go only
			// after disputed claims (AR-07).
			if anyShortened(p) {
				sawShortening = true
				if len(p.OmittedSections) < 5 {
					t.Fatalf("%s: statements shortened before sections dropped: %v", ctx, p.OmittedSections)
				}
			}
			if len(p.OmittedSections) >= 6 && longestText(p) > 60 {
				t.Fatalf("%s: disputed claims dropped before statements reached 60 bytes", ctx)
			}
			o := p.Omitted
			if o.DisprovenHypotheses > 0 && len(p.OmittedSections) < 6 {
				t.Fatalf("%s: disproven hypotheses removed before disputed claims", ctx)
			}
			// Items go from open questions, then hypotheses, then claims.
			if o.Hypotheses > 0 && len(p.OpenQuestions) != 0 {
				t.Fatalf("%s: hypotheses removed while open questions remain", ctx)
			}
			if o.EstablishedClaims > 0 && (len(p.Hypotheses) != 0 || len(p.OpenQuestions) != 0) {
				t.Fatalf("%s: claims removed while hypotheses or questions remain", ctx)
			}
			if o.OpenQuestions+o.Hypotheses+o.EstablishedClaims > 0 {
				sawItemRemoval = true
				if len(p.OmittedSections) != 7 {
					t.Fatalf("%s: items removed before disputed claims dropped", ctx)
				}
			}
		}
		if name == "long" && !sawShortening {
			t.Fatalf("%s: no budget exercised statement shortening", name)
		}
		if !sawItemRemoval {
			t.Fatalf("%s: no budget exercised item removal", name)
		}
	}
}

func TestBudgetFirstStepDropsRecentChanges(t *testing.T) {
	s := testSnapshot()
	full := Build(s, Options{})
	p := Build(s, Options{MaxBytes: len(RenderMarkdown(full)) - 1})
	if !reflect.DeepEqual(p.OmittedSections, []string{"recent_changes"}) || len(p.RecentChanges) != 0 {
		t.Fatalf("omitted = %v, recent = %d", p.OmittedSections, len(p.RecentChanges))
	}
	if !reflect.DeepEqual(p.EvidenceIndex, full.EvidenceIndex) || !reflect.DeepEqual(p.EstablishedClaims, full.EstablishedClaims) {
		t.Fatal("first budget step changed more than recent changes")
	}
	md := RenderMarkdown(p)
	if strings.Contains(md, "Recent material changes:") || !strings.HasSuffix(md, "\n\n(Trimmed to fit "+fmt.Sprint(p.MaxBytes)+" bytes; omitted: recent_changes)\n") {
		t.Fatalf("markdown:\n%s", md)
	}
}

func TestBudgetShortensWithEllipsis(t *testing.T) {
	s := longSnapshot()
	for limit := len(RenderMarkdown(Build(s, Options{}))); limit > 0; limit -= 5 {
		p := Build(s, Options{MaxBytes: limit})
		if longestText(p) >= 400 {
			continue
		}
		// First shortening level: 240 bytes, ending in "...".
		if len(p.OmittedSections) != 5 {
			t.Fatalf("limit %d: omitted %v at first shortening", limit, p.OmittedSections)
		}
		for _, c := range p.EstablishedClaims {
			if len(c.Statement) > 240 || !strings.HasSuffix(c.Statement, "...") || !strings.HasPrefix(c.Statement, c.Ref+" ") {
				t.Fatalf("limit %d: %s statement %q", limit, c.Ref, c.Statement)
			}
		}
		if md := RenderMarkdown(p); len(md) > limit {
			t.Fatalf("limit %d: %d bytes", limit, len(md))
		}
		return
	}
	t.Fatal("no limit shortened statements")
}

func TestBudgetTinyKeepsHeader(t *testing.T) {
	p := Build(testSnapshot(), Options{MaxBytes: 10})
	md := RenderMarkdown(p)
	want := PackOmitted{EstablishedClaims: 3, DisputedClaims: 1, Hypotheses: 2, OpenQuestions: 1, DisprovenHypotheses: 1, EvidenceOutsideFindings: 1}
	if p.Omitted != want || !minimal(p) {
		t.Fatalf("omitted = %+v minimal=%v", p.Omitted, minimal(p))
	}
	for _, h := range headerParts {
		if !strings.Contains(md, h) {
			t.Fatalf("header part %q missing:\n%s", h, md)
		}
	}
	for _, line := range []string{
		"Current claims:\n(3 more current claims omitted)\n",
		"Disputed claims:\n(1 more disputed claim omitted)\n",
		"Known disproven hypotheses:\n(1 more disproven hypothesis omitted)\n",
		"Hypotheses:\n(2 more hypotheses omitted)\n",
		"Open questions:\n(1 more open question omitted)\n",
		"(Trimmed to fit 10 bytes; omitted: recent_changes, statement_rationales, evidence_index, deferred_questions, rationale, disputed_claims, disproven_hypotheses)\n",
	} {
		if !strings.Contains(md, line) {
			t.Fatalf("markdown lacks %q:\n%s", line, md)
		}
	}
	if strings.Contains(md, "Conclusion rationale:") {
		t.Fatalf("dropped sections rendered:\n%s", md)
	}
	if p.Conclusion.Text != "Counts use different cutoffs." || p.Conclusion.Confidence != domain.ConfidenceMedium {
		t.Fatalf("conclusion changed: %+v", p.Conclusion)
	}
}

func TestBudgetCompactDefault(t *testing.T) {
	big := testSnapshot()
	for i := 0; i < 40; i++ {
		big.Claims = append(big.Claims, domain.Claim{
			Ref: fmt.Sprintf("C-%03d", 300+i), Statement: strings.Repeat("A long established claim. ", 12),
			Confidence: domain.ConfidenceMedium, Status: domain.ClaimStatusActive, Verification: domain.VerificationAIGenerated,
		})
	}
	if n := len(RenderMarkdown(Build(big, Options{}))); n <= DefaultCompactMaxBytes {
		t.Fatalf("big snapshot renders %d bytes; test needs more than %d", n, DefaultCompactMaxBytes)
	}
	p := Build(big, Options{Compact: true})
	if md := RenderMarkdown(p); !p.Truncated || p.MaxBytes != DefaultCompactMaxBytes || len(md) > DefaultCompactMaxBytes {
		t.Fatalf("compact: truncated=%v max=%d len=%d", p.Truncated, p.MaxBytes, len(md))
	}
	if p := Build(big, Options{Compact: true, MaxBytes: 2500}); p.MaxBytes != 2500 || len(RenderMarkdown(p)) > 2500 {
		t.Fatalf("compact with MaxBytes: max=%d len=%d", p.MaxBytes, len(RenderMarkdown(p)))
	}
	if p := Build(big, Options{}); p.Truncated || p.MaxBytes != 0 {
		t.Fatal("no budget without Compact or MaxBytes")
	}
	small := Build(testSnapshot(), Options{Compact: true})
	if small.Truncated || RenderMarkdown(small) != wantMarkdown {
		t.Fatal("compact trimmed a pack that already fits")
	}
}

func TestBudgetDeterministic(t *testing.T) {
	for _, limit := range []int{50, 400, 800, 1200} {
		a := RenderMarkdown(Build(longSnapshot(), Options{MaxBytes: limit}))
		b := RenderMarkdown(Build(longSnapshot(), Options{MaxBytes: limit}))
		if a != b {
			t.Fatalf("limit %d: repeated builds differ", limit)
		}
	}
}

// TestBudgetKeepsDisprovenHypotheses reproduces AR-07: a compact pack of a
// large investigation shortens statements before it gives up the disproven
// hypotheses, which exist so they are not retried.
func TestBudgetKeepsDisprovenHypotheses(t *testing.T) {
	s := testSnapshot()
	s.Claims = nil
	for i := 1; i <= 10; i++ {
		s.Claims = append(s.Claims, domain.Claim{
			Ref: fmt.Sprintf("C-%03d", i), Statement: strings.Repeat("A long established claim statement. ", 17),
			Confidence: domain.ConfidenceHigh, Status: domain.ClaimStatusActive, Verification: domain.VerificationAIGenerated,
		})
	}
	s.Hypotheses = nil
	for i := 1; i <= 4; i++ {
		s.Hypotheses = append(s.Hypotheses, domain.Hypothesis{
			Ref: fmt.Sprintf("H-%03d", i), Statement: fmt.Sprintf("Disproven idea number %d about time zones.", i),
			Status: domain.HypothesisStatusDisproven, Verification: domain.VerificationAIGenerated,
		})
	}
	p := Build(s, Options{Compact: true})
	md := RenderMarkdown(p)
	if !p.Truncated || len(md) > DefaultCompactMaxBytes {
		t.Fatalf("compact: truncated=%v len=%d", p.Truncated, len(md))
	}
	if len(p.DisprovenHypotheses) != 4 || p.Omitted.DisprovenHypotheses != 0 {
		t.Fatalf("disproven kept = %d, omitted %d:\n%s", len(p.DisprovenHypotheses), p.Omitted.DisprovenHypotheses, md)
	}
	if !anyShortened(p) {
		t.Fatalf("expected statements to be shortened first:\n%s", md)
	}
	if !strings.Contains(md, "Known disproven hypotheses:\nH-001 ") {
		t.Fatalf("markdown lacks disproven hypotheses:\n%s", md)
	}
}

// TestBudgetDisprovenPartialRemoval: when the budget must cut into the
// disproven hypotheses, the kept ones stay and a line counts the rest.
func TestBudgetDisprovenPartialRemoval(t *testing.T) {
	s := testSnapshot()
	for i := 10; i < 20; i++ {
		s.Hypotheses = append(s.Hypotheses, domain.Hypothesis{
			Ref: fmt.Sprintf("H-%03d", i), Statement: "Another disproven idea that was tried and failed.",
			Status: domain.HypothesisStatusDisproven, Verification: domain.VerificationAIGenerated,
		})
	}
	for limit := len(RenderMarkdown(Build(s, Options{}))); limit > 0; limit-- {
		p := Build(s, Options{MaxBytes: limit})
		if p.Omitted.DisprovenHypotheses == 0 || len(p.DisprovenHypotheses) == 0 {
			continue
		}
		md := RenderMarkdown(p)
		want := fmt.Sprintf("(%d more disproven hypothes", p.Omitted.DisprovenHypotheses)
		if !strings.Contains(md, want) || len(p.DisputedClaims) != 0 {
			t.Fatalf("limit %d: partial removal not rendered:\n%s", limit, md)
		}
		return
	}
	t.Fatal("no budget removed only some disproven hypotheses")
}
