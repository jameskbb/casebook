// Package demo writes the demo investigations used for the README images,
// the screenshot pass and the Windows smoke (D-08). The featured case is
// INV-0002 "Northeast margin down 6 points in Q3": three sessions over four
// days, Claude Code (Monday, Thursday) and Codex (Tuesday), and one person
// who confirms the cause with the regional sales-ops lead and verifies
// C-003. INV-0001 is the flaky test, an earlier finished case; INV-0003 is
// the checkout outage of October 2, blocked on the payment provider; the
// others fill the sidebar with a resolved and an archived investigation.
//
// Everything is written through public internal/app methods with the
// actors a real session would use, so the History reads as it would after
// real work. The desktop --seed-demo flag and desktop/tools/aiwrite are the
// only callers.
package demo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jameskbb/casebook/internal/app"
)

// FeaturedRef is the investigation the README images show.
const FeaturedRef = "INV-0002"

// Stage says how much of the story Seed writes.
type Stage string

const (
	// StageFull writes the whole story, ending with the person's Verify on
	// C-003 and the conclusion raised to High.
	StageFull Stage = "full"
	// StageBeforeThursday stops before Thursday's Claude Code session, so
	// ThursdayCheckpoint can be written live from another process (the
	// split hero image and the live-write recording).
	StageBeforeThursday Stage = "before-thursday"
)

// ParseStage accepts "" (full), "full" and "before-thursday".
func ParseStage(s string) (Stage, error) {
	switch Stage(s) {
	case "", StageFull:
		return StageFull, nil
	case StageBeforeThursday:
		return StageBeforeThursday, nil
	}
	return "", fmt.Errorf("unknown demo stage %q (want full or before-thursday)", s)
}

// Actors of the demo.
var (
	Person     = app.ActorDesktop()
	CLI        = app.ActorCLI()
	ClaudeCode = app.ActorMCP("claude-code")
	Codex      = app.ActorMCP("codex")
)

// Clock hands out increasing timestamps so the demo shows realistic
// relative times ("3 hours ago", "Monday") instead of one instant. Jump
// moves it forward to a scene's time; every call then advances one second.
// Pass Now to app.WithClock.
type Clock struct {
	mu   sync.Mutex
	next time.Time
}

// NewClock starts a clock at start.
func NewClock(start time.Time) *Clock { return &Clock{next: start} }

// Now returns the next timestamp.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.next
	c.next = c.next.Add(time.Second)
	return t
}

// Jump moves the clock forward to t (never back).
func (c *Clock) Jump(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t.After(c.next) {
		c.next = t
	}
}

// Start is where a seed clock should start for a given now: before the
// earliest scene.
func Start(now time.Time) time.Time { return now.Add(-10 * 24 * time.Hour) }

// Seed writes the demo into a, which must be empty and opened with
// app.WithClock(clock.Now). Scenes are placed between about nine days and
// one hour before now.
func Seed(ctx context.Context, a *app.App, clock *Clock, now time.Time, stage Stage) error {
	s := &seeder{ctx: ctx, a: a, clock: clock, now: now}
	s.run(stage)
	return s.err
}

// ThursdayNote is the note of Thursday's Claude Code checkpoint.
const ThursdayNote = "The July 6 playbook made white-glove onboarding free on every deal above $250k (E-008). " +
	"Seven Northeast deals received it with no onboarding fee (E-009). " +
	"Added C-003, marked H-003 supported and resolved Q-002."

// ThursdayCheckpoint is Thursday's Claude Code session, saved in one
// casebook_checkpoint call on INV-0002: E-008 (checked against the source),
// E-009, C-003 linked to E-007, E-008 and E-009, H-003 supported at high
// confidence, Q-002 resolved, and the conclusion at medium.
func ThursdayCheckpoint() app.CheckpointInput {
	return app.CheckpointInput{
		Note: ThursdayNote,
		Evidence: []app.CheckpointEvidence{
			{Key: "playbook", Evidence: app.EvidenceInput{
				Title:        "July 6 playbook update: white-glove onboarding included free on every deal above $250k",
				Summary:      `The Northeast sales playbook revised on July 6 (v7) adds "White-glove onboarding is included at no charge on every deal with ACV above $250k" to the standard offer; v6 (April 2) listed white-glove onboarding as a priced add-on at $120k to $180k.`,
				SourceType:   app.SourceTypeDocumentation,
				SourceRef:    "Sales shared drive: Northeast Sales Playbook v7 (2026-07-06), section 4",
				SourceLabel:  "Sales playbook v7",
				Verification: app.VerificationSourceVerified,
				RawDetail: "Northeast Sales Playbook, section 4 \"Standard offer\"\n" +
					"- v6 (2026-04-02): White-glove onboarding: priced add-on, $120k to $180k by deal size.\n" +
					"+ v7 (2026-07-06): White-glove onboarding is included at no charge on every deal with ACV above $250k.",
				Metadata: `{"revised":"2026-07-06","threshold_usd":250000}`,
			}},
			{Key: "dealdesk", Evidence: app.EvidenceInput{
				Title:       "Deal desk: seven Northeast deals above $250k since July 6, onboarding fee $0 on every one",
				Summary:     "The deal desk approvals export lists seven Northeast deals above $250k closed from July 6 to September 30, every one on the white-glove tier with an onboarding fee of $0; the four Q2 deals of that size carried fees of $120k to $180k.",
				SourceType:  app.SourceTypeFile,
				SourceRef:   "salesops/exports/northeast-approvals-2026-Q3.csv",
				SourceLabel: "Deal desk approvals export",
				RawDetail: "deal_id,account,signed,acv_usd,onboarding_tier,onboarding_fee_usd\n" +
					"NE-2311,Pinecrest Logistics,2026-07-08,1120000,white-glove,0\n" +
					"NE-2318,Pinecrest Logistics,2026-07-22,760000,white-glove,0\n" +
					"NE-2320,Pellston Health,2026-07-14,540000,white-glove,0\n" +
					"NE-2327,Ridgeway Mutual,2026-07-29,410000,white-glove,0\n" +
					"NE-2340,Cobalt Freight,2026-08-11,330000,white-glove,0\n" +
					"NE-2352,Marlowe Energy,2026-09-02,290000,white-glove,0\n" +
					"NE-2361,Tilden Schools,2026-09-24,265000,white-glove,0",
			}},
		},
		Claims: []app.ClaimInput{{
			Statement:           "The July 6 playbook update made white-glove onboarding free on every deal above $250k; seven Northeast deals got it and none was charged.",
			Confidence:          app.ConfidenceHigh,
			ConfidenceRationale: "The playbook text and the deal desk records agree; the three accounts in E-007 are among the seven.",
			Links:               []app.EvidenceLinkInput{supports("E-007"), supports("#playbook"), supports("#dealdesk")},
		}},
		HypothesisUpdates: []app.HypothesisUpdate{{
			Ref: "H-003",
			Patch: app.HypothesisPatch{
				Status:     ptr(app.HypothesisStatusSupported),
				Confidence: ptr(app.ConfidenceHigh),
			},
		}},
		QuestionUpdates: []app.QuestionUpdate{{
			Ref:        "Q-002",
			Resolution: ptr("The Northeast sales playbook revised on July 6 made white-glove onboarding free on every deal above $250k (E-008)."),
		}},
		Conclusion: &app.ConclusionInput{
			Conclusion: Conclusion,
			Confidence: app.ConfidenceMedium,
			Rationale:  "C-002 sizes the cost and C-003 explains it; the playbook text is the only word so far on who meant it for whom, and H-004 is still open.",
		},
	}
}

// Conclusion is the featured case's conclusion text.
const Conclusion = "White-glove onboarding, given away free on every deal above $250k since July, drove most of the margin drop."

// PersonRationale is the person's rationale when raising the conclusion
// to High.
const PersonRationale = "The playbook text (E-008), the deal desk records (E-009) and the sales-ops lead (E-010) agree; travel on the same accounts (H-004) and $400k of smaller lines are still open."

// seeder carries the first error through the scenes so the script reads
// top to bottom.
type seeder struct {
	ctx   context.Context
	a     *app.App
	clock *Clock
	now   time.Time
	err   error
}

// at moves the clock to now minus ago.
func (s *seeder) at(ago time.Duration) { s.clock.Jump(s.now.Add(-ago)) }

func (s *seeder) check(step string, err error) bool {
	if s.err != nil {
		return false
	}
	if err != nil {
		s.err = fmt.Errorf("seed %s: %w", step, err)
		return false
	}
	return true
}

func supports(ref string) app.EvidenceLinkInput {
	return app.EvidenceLinkInput{EvidenceRef: ref, Relation: app.RelationSupports}
}

func contradicts(ref string) app.EvidenceLinkInput {
	return app.EvidenceLinkInput{EvidenceRef: ref, Relation: app.RelationContradicts}
}

func ptr[T any](v T) *T { return &v }

const (
	hour = time.Hour
	day  = 24 * time.Hour
)

// run writes the scenes in time order (the clock never moves back). Refs follow creation order:
// INV-0001 the flaky test (last week), INV-0002 the Northeast margin
// (Monday), INV-0003 the checkout outage (blocked), INV-0004 SSO accounts (resolved),
// INV-0005 the export job (archived).
func (s *seeder) run(stage Stage) {
	s.flakyTest()
	s.marginMonday()
	s.checkoutOutage()
	s.marginTuesday()
	s.ssoAccounts()
	s.checkoutBlocked()
	s.exportJob()
	if s.err != nil || stage == StageBeforeThursday {
		return
	}
	s.at(3 * hour)
	_, err := s.a.CheckpointBatch(s.ctx, ClaudeCode, FeaturedRef, ThursdayCheckpoint())
	if !s.check("Thursday checkpoint", err) {
		return
	}
	s.person()
}

// flakyTest is INV-0001, a small engineering case worked and resolved last
// week: H-001 (midnight) disproven, C-001 (the shared cart cache) verified
// by the person, conclusion High.
func (s *seeder) flakyTest() {
	a, ctx := s.a, s.ctx
	s.at(9 * day)
	inv, err := a.CreateInvestigation(ctx, ClaudeCode, app.CreateInvestigationInput{
		Title:            "TestCheckoutTotals flakes in CI",
		ProblemStatement: "Why does TestCheckoutTotals fail about once in 20 CI runs and never locally?",
		Status:           app.InvestigationStatusInvestigating,
	})
	if !s.check("INV-0001", err) {
		return
	}
	_, err = a.CheckpointBatch(ctx, ClaudeCode, inv.Ref, app.CheckpointInput{
		Note: "23 of the last 400 CI runs fail on the totals assertion (E-001). " +
			"Failure times are spread across the day (E-002), so H-001 (midnight) is disproven.",
		Evidence: []app.CheckpointEvidence{
			{Key: "log", Evidence: app.EvidenceInput{
				Title:       "23 of the last 400 CI runs fail on the totals assertion",
				Summary:     "All 23 failures are the same assertion: expected 3 line items, got 2.",
				SourceType:  app.SourceTypeFile,
				SourceRef:   "ci/logs/checkout-2026-10.txt",
				SourceLabel: "CI failure log",
				RawDetail: "--- FAIL: TestCheckoutTotals (0.02s)\n" +
					"    totals_test.go:64: line items: got 2, want 3\n" +
					"FAIL\tshop/internal/checkout\t1.204s",
			}},
			{Key: "times", Evidence: app.EvidenceInput{
				Title:       "Failure times are spread across the day; none within 30 min of 00:00 UTC",
				Summary:     "The 23 failing runs started between 01:12 and 22:47 UTC with no cluster at any hour; the closest to midnight started at 01:12.",
				SourceType:  app.SourceTypeSQLQuery,
				SourceRef:   "ci/queries/checkout_failures.sql",
				SourceLabel: "CI runs query",
			}},
		},
		Hypotheses: []app.HypothesisInput{{
			Statement:  "The failure is a midnight boundary: the fixture's date flips between seeding and assertion.",
			Confidence: app.ConfidenceMedium,
			Links:      []app.EvidenceLinkInput{contradicts("#times")},
		}},
		HypothesisUpdates: []app.HypothesisUpdate{{
			Ref:   "H-001",
			Patch: app.HypothesisPatch{Status: ptr(app.HypothesisStatusDisproven)},
		}},
	})
	if !s.check("INV-0001 checkpoint", err) {
		return
	}

	s.at(8*day + 20*hour)
	race, err := a.AddEvidence(ctx, ClaudeCode, inv.Ref, app.EvidenceInput{
		Title:       "go test -race reproduces a data race on cartCache, 50 of 50 runs",
		Summary:     "go test -race -count=50 ./internal/checkout reports a data race on cartCache in every run: TestCheckoutDiscounts writes the map while TestCheckoutTotals reads it.",
		SourceType:  app.SourceTypeAIToolOutput,
		SourceRef:   "go test -race -count=50 ./internal/checkout",
		SourceLabel: "Local race run",
	})
	if !s.check("INV-0001 E-003", err) {
		return
	}
	c, err := a.AddClaim(ctx, ClaudeCode, inv.Ref, app.ClaimInput{
		Statement:           "TestCheckoutTotals fails when TestCheckoutDiscounts runs concurrently and overwrites the package-level cartCache map.",
		Confidence:          app.ConfidenceHigh,
		ConfidenceRationale: "Reproduced 50 of 50 times with -race.",
		Links:               []app.EvidenceLinkInput{supports(race.Ref)},
	})
	if !s.check("INV-0001 C-001", err) {
		return
	}
	_, err = a.UpdateConclusion(ctx, ClaudeCode, inv.Ref, app.ConclusionInput{
		Conclusion: "Two parallel tests share a package-level cart cache; the slower 4-core runners widen the race window.",
		Confidence: app.ConfidenceMedium,
	})
	if !s.check("INV-0001 conclusion", err) {
		return
	}

	s.at(8 * day)
	_, err = a.SetClaimVerification(ctx, Person, inv.Ref, c.Ref, app.VerificationHumanVerified)
	if !s.check("INV-0001 verify C-001", err) {
		return
	}
	_, err = a.UpdateConclusion(ctx, Person, inv.Ref, app.ConclusionInput{
		Conclusion: "Two parallel tests share a package-level cart cache; the slower 4-core runners widen the race window.",
		Confidence: app.ConfidenceHigh,
		Rationale:  "The race detector reproduces it (E-003).",
	})
	if !s.check("INV-0001 conclusion High", err) {
		return
	}
	_, err = a.UpdateInvestigation(ctx, Person, inv.Ref, app.InvestigationPatch{Status: ptr(app.InvestigationStatusResolved)})
	s.check("INV-0001 resolve", err)
}

// marginMonday is Monday's Claude Code session on INV-0002: it opens the
// case, makes it active and saves E-001 to E-004, H-001, H-002, C-001 (the
// sales VP's hunch, at low confidence) and Q-001 in one checkpoint.
func (s *seeder) marginMonday() {
	a, ctx := s.a, s.ctx
	if s.err != nil {
		return
	}
	s.at(3*day + 2*hour)
	inv, err := a.CreateInvestigation(ctx, ClaudeCode, app.CreateInvestigationInput{
		Title:            "Northeast margin down 6 points in Q3",
		ProblemStatement: "Why did gross margin in the Northeast fall from 41% to 35% in Q3 when revenue was up 4%?",
		Status:           app.InvestigationStatusInvestigating,
	})
	if !s.check(FeaturedRef, err) {
		return
	}
	_, err = a.SetActiveInvestigation(ctx, ClaudeCode, inv.Ref)
	if !s.check(FeaturedRef+" set active", err) {
		return
	}
	s.at(3*day + hour)
	_, err = a.CheckpointBatch(ctx, ClaudeCode, inv.Ref, app.CheckpointInput{
		Note: "Northeast Q3 revenue up 4% and cost of delivery up 15%, gross margin 41% to 35% (E-001, E-002). " +
			"Two candidates: the July contractor hires (E-004, H-001) and FX on the Canadian accounts (H-002). " +
			"The sales VP's view that deep discounts on the quarter's two biggest deals explain the drop is recorded as C-001 at Low confidence (E-003). " +
			"Q-001 asks which cost lines moved.",
		Evidence: []app.CheckpointEvidence{
			{Key: "pnl", Evidence: app.EvidenceInput{
				Title:       "Northeast Q3: revenue $27.0M (+4%), cost of delivery $17.6M (+15%), gross margin 35% against 41%",
				Summary:     "The Q3 regional P&L shows Northeast revenue of $27.0M against $26.0M in Q2 and cost of delivery of $17.6M against $15.3M, taking gross margin from 41.2% to 34.8%; the P&L carries cost of delivery as one line.",
				SourceType:  app.SourceTypeFile,
				SourceRef:   "finance/pnl/northeast-2026-Q3.xlsx",
				SourceLabel: "Q3 regional P&L",
				Metadata:    `{"q2_revenue_usd":26000000,"q3_revenue_usd":27000000,"q2_cost_usd":15300000,"q3_cost_usd":17600000}`,
			}},
			{Key: "bookings", Evidence: app.EvidenceInput{
				Title:       "Q3 bookings: 47 new Northeast deals, seven above $250k, the two biggest with Pinecrest Logistics",
				Summary:     "The Q3 bookings report lists 47 new Northeast deals, seven with ACV above $250k, including the two biggest, both with Pinecrest Logistics, signed July 8 and July 22.",
				SourceType:  app.SourceTypeFile,
				SourceRef:   "salesops/reports/northeast-bookings-2026-Q3.csv",
				SourceLabel: "Q3 bookings report",
			}},
			{Key: "salesvp", Evidence: app.EvidenceInput{
				Title:       "Sales VP: we cut prices hard to win the quarter's two biggest deals",
				Summary:     "The sales VP said in the Monday pipeline call that the region cut prices hard to win the quarter's two biggest deals in July and expects that to explain the margin drop.",
				SourceType:  app.SourceTypeUserStatement,
				SourceLabel: "Sales VP, in the pipeline call",
			}},
			{Key: "contractors", Evidence: app.EvidenceInput{
				Title:       "Onboarding team added four contractors in July",
				Summary:     "The contractor roster shows four implementation contractors starting in the Northeast onboarding team on July 13 and July 20, at $135 to $145 an hour.",
				SourceType:  app.SourceTypeFile,
				SourceRef:   "people/contractors/northeast-onboarding-2026.csv",
				SourceLabel: "Contractor roster",
				RawDetail: "contractor,role,start,rate_usd_per_hour\n" +
					"Ardent Consulting (3 seats),implementation lead,2026-07-13,145\n" +
					"Northline Staffing (1 seat),implementation engineer,2026-07-20,135",
			}},
		},
		Hypotheses: []app.HypothesisInput{
			{
				Statement:  "July contractor hires raised the cost of each onboarding in the Northeast.",
				Confidence: app.ConfidenceMedium,
				Links:      []app.EvidenceLinkInput{supports("#contractors")},
			},
			{
				Statement:  "The weaker Canadian dollar inflated delivery cost on the Canadian accounts.",
				Confidence: app.ConfidenceLow,
			},
		},
		Claims: []app.ClaimInput{{
			Statement:           "Deep discounts on the quarter's two biggest deals caused most of the margin drop.",
			Confidence:          app.ConfidenceLow,
			ConfidenceRationale: "The sales VP's reading of the quarter; not yet checked against the deal desk.",
			Links:               []app.EvidenceLinkInput{supports("#salesvp")},
		}},
		Questions: []app.QuestionInput{{
			Question: "Which cost of delivery lines moved? The P&L has one line for cost of delivery; the cost ledger has it by line.",
		}},
	})
	s.check(FeaturedRef+" Monday checkpoint", err)
}

// marginTuesday is Tuesday's Codex session on INV-0002: the cost ledger
// (E-005) disproves H-001 and H-002 and answers Q-001; discount rates
// (E-006) dispute C-001; implementation cost by account (E-007) points at
// three accounts moved to white-glove onboarding in July (H-003, Q-002);
// C-002 sizes the cost.
func (s *seeder) marginTuesday() {
	a, ctx := s.a, s.ctx
	if s.err != nil {
		return
	}
	s.at(2*day + 2*hour)
	e5, err := a.AddEvidence(ctx, Codex, FeaturedRef, app.EvidenceInput{
		Title:       "Implementation services are $1.9M of the $2.3M cost rise; contractor rate flat, hours up 78%",
		Summary:     "In the cost ledger, implementation services rose from $2.4M to $4.3M, which is $1.9M of the $2.3M quarter-over-quarter rise; travel and expenses +$210k, customer support +$110k, hosting +$90k, FX revaluation +$40k, everything else -$50k. The blended contractor rate is $141 an hour against $140; hours are 30,400 against 17,100.",
		SourceType:  app.SourceTypeSQLQuery,
		SourceRef:   "finance/queries/cost_of_delivery_by_line_qoq.sql",
		SourceLabel: "Cost ledger query",
		RawDetail: "SELECT cost_line,\n" +
			"       SUM(CASE WHEN fiscal_quarter = '2026-Q3' THEN amount_usd END)\n" +
			"     - SUM(CASE WHEN fiscal_quarter = '2026-Q2' THEN amount_usd END) AS delta_usd\n" +
			"FROM finance.cost_of_delivery\n" +
			"WHERE region = 'Northeast' AND fiscal_quarter IN ('2026-Q2', '2026-Q3')\n" +
			"GROUP BY cost_line\n" +
			"ORDER BY delta_usd DESC;\n" +
			"-- implementation_services +1,900,000 | travel_and_expenses +210,000 | customer_support +110,000 | hosting +90,000 | fx_revaluation +40,000 | other -50,000\n" +
			"-- implementation_services: 30,400 hours at $141/h (Q2: 17,100 hours at $140/h)",
		Metadata: `{"cost_rise_usd":2300000,"implementation_services_usd":1900000,"q2_cost_usd":15300000,"q3_cost_usd":17600000}`,
	})
	if !s.check(FeaturedRef+" E-005", err) {
		return
	}
	for _, h := range []string{"H-001", "H-002"} {
		_, err = a.LinkEvidenceToHypothesis(ctx, Codex, FeaturedRef, h, e5.Ref, app.RelationContradicts)
		if !s.check(FeaturedRef+" E-005 contradicts "+h, err) {
			return
		}
	}
	s.at(2*day + hour)
	_, err = a.CheckpointBatch(ctx, Codex, FeaturedRef, app.CheckpointInput{
		Note: "The cost ledger settles Q-001: implementation services are $1.9M of the $2.3M rise (E-005), the contractor rate is flat and FX is $40k, so H-001 and H-002 are disproven. " +
			"Discount rates are flat at 12% (E-006), so C-001 is disputed. " +
			"Three accounts moved to white-glove onboarding in July carry $1.4M (E-007); C-002 sizes it, H-003 suspects a July policy change and Q-002 asks what it was; H-004 keeps travel open.",
		Evidence: []app.CheckpointEvidence{
			{Key: "discounts", Evidence: app.EvidenceInput{
				Title:       "Discounts are flat: 11.6% in Q3 against 11.8% in Q2; the two biggest deals got 12% and 14%",
				Summary:     "Across the 47 Northeast deals booked in Q3 the average discount is 11.6% against 11.8% over Q2's 44 deals; the two biggest deals got 12% and 14%, inside the 8% to 16% range of the last four quarters.",
				SourceType:  app.SourceTypeSQLQuery,
				SourceRef:   "salesops/queries/discount_rate_by_quarter.sql",
				SourceLabel: "Deal desk discount query",
				RawDetail: "SELECT fiscal_quarter, AVG(discount_pct) AS avg_discount, COUNT(*) AS deals\n" +
					"FROM salesops.closed_deals\n" +
					"WHERE region = 'Northeast' AND fiscal_quarter IN ('2026-Q2', '2026-Q3')\n" +
					"GROUP BY fiscal_quarter;\n" +
					"-- 2026-Q2 11.8% (44 deals) | 2026-Q3 11.6% (47 deals)\n" +
					"-- Pinecrest Logistics: NE-2311 12.0%, NE-2318 14.0%; last four quarters range 8% to 16%",
				Metadata: `{"q2_discount":0.118,"q3_discount":0.116,"biggest_deal_discounts":[0.12,0.14]}`,
			}},
			{Key: "accounts", Evidence: app.EvidenceInput{
				Title:       "Three accounts moved to white-glove onboarding in July carry $1.4M of the $1.9M",
				Summary:     "Implementation cost by account: Pinecrest Logistics, Pellston Health and Ridgeway Mutual moved to the white-glove onboarding tier in July and carry $1.4M of the $1.9M rise; three later deals above $250k add $300k together and the standard tier $200k.",
				SourceType:  app.SourceTypeSQLQuery,
				SourceRef:   "finance/queries/implementation_cost_by_account.sql",
				SourceLabel: "Implementation hours query",
				RawDetail: "SELECT account, onboarding_tier, SUM(hours * rate_usd) AS cost_usd\n" +
					"FROM finance.implementation_hours\n" +
					"WHERE region = 'Northeast' AND fiscal_quarter = '2026-Q3'\n" +
					"GROUP BY account, onboarding_tier\n" +
					"ORDER BY cost_usd DESC;\n" +
					"-- rise against Q2, white-glove since July: Pinecrest Logistics +700,000 | Pellston Health +400,000 | Ridgeway Mutual +300,000\n" +
					"-- white-glove, signed August to September: Cobalt Freight, Marlowe Energy, Tilden Schools +300,000 together\n" +
					"-- standard tier, all other accounts: +200,000",
				Metadata: `{"white_glove_accounts":3,"white_glove_rise_usd":1400000}`,
			}},
		},
		Claims: []app.ClaimInput{{
			Statement:           "Implementation services are $1.9M of the $2.3M cost rise; three accounts moved to white-glove onboarding in July carry $1.4M of it.",
			Confidence:          app.ConfidenceHigh,
			ConfidenceRationale: "The cost ledger by line and by account agree.",
			Links:               []app.EvidenceLinkInput{supports("E-001"), supports(e5.Ref), supports("#accounts")},
		}},
		Hypotheses: []app.HypothesisInput{
			{
				Statement:  "A July change in sales policy, not account-by-account decisions, put the new accounts on white-glove onboarding.",
				Confidence: app.ConfidenceMedium,
				Links:      []app.EvidenceLinkInput{supports("#accounts")},
			},
			{
				Statement:  "Travel for on-site white-glove sessions is growing with the same accounts.",
				Confidence: app.ConfidenceMedium,
				Links:      []app.EvidenceLinkInput{supports(e5.Ref)},
			},
		},
		HypothesisUpdates: []app.HypothesisUpdate{
			{Ref: "H-001", Patch: app.HypothesisPatch{Status: ptr(app.HypothesisStatusDisproven)}},
			{Ref: "H-002", Patch: app.HypothesisPatch{Status: ptr(app.HypothesisStatusDisproven)}},
		},
		QuestionUpdates: []app.QuestionUpdate{{
			Ref:        "Q-001",
			Resolution: ptr("Implementation services +$1.9M, travel +$210k, customer support +$110k, hosting +$90k, FX revaluation +$40k; everything else -$50k (E-005)."),
		}},
		Questions: []app.QuestionInput{{
			Question: "What changed in July that moved three accounts to white-glove onboarding at once?",
		}},
	})
	if !s.check(FeaturedRef+" Tuesday checkpoint", err) {
		return
	}
	_, err = a.LinkEvidenceToClaim(ctx, Codex, FeaturedRef, "C-001", "E-006", app.RelationContradicts)
	if !s.check(FeaturedRef+" E-006 contradicts C-001", err) {
		return
	}
	// An AI may raise doubt (ADR 0013); the status follows the verification.
	_, err = a.SetClaimVerification(ctx, Codex, FeaturedRef, "C-001", app.VerificationDisputed)
	s.check(FeaturedRef+" dispute C-001", err)
}

// person is the person's part on Thursday afternoon: two new questions
// (one deferred to the quarter close), the regional sales-ops lead's word
// as E-010 linked to C-003, Verify on C-003 and the conclusion raised to
// High.
func (s *seeder) person() {
	a, ctx := s.a, s.ctx
	s.at(80 * time.Minute)
	_, err := a.AddQuestion(ctx, Person, FeaturedRef, app.QuestionInput{
		Question: "Can onboarding be charged on the Q4 deals already signed under the July playbook?",
	})
	if !s.check(FeaturedRef+" Q-003", err) {
		return
	}
	s.at(78 * time.Minute)
	q4, err := a.AddQuestion(ctx, Person, FeaturedRef, app.QuestionInput{
		Question: "Should the Q3 regional forecast be restated for the onboarding that was delivered and never charged?",
	})
	if !s.check(FeaturedRef+" Q-004", err) {
		return
	}
	_, err = a.DeferQuestion(ctx, Person, FeaturedRef, q4.Ref, "Waiting for the quarter close on October 10.")
	if !s.check(FeaturedRef+" defer Q-004", err) {
		return
	}
	s.at(75 * time.Minute)
	e10, err := a.AddEvidence(ctx, Person, FeaturedRef, app.EvidenceInput{
		Title:       "Regional sales-ops lead: the free white-glove clause was written for the two Pinecrest deals only",
		Summary:     "The regional sales-ops lead said in person that the July 6 playbook clause was written to close the two Pinecrest deals and was never meant to apply to every deal above $250k; nobody reviewed it after Pinecrest signed.",
		SourceType:  app.SourceTypeUserStatement,
		SourceLabel: "Sales-ops lead, in person",
	})
	if !s.check(FeaturedRef+" E-010", err) {
		return
	}
	_, err = a.LinkEvidenceToClaim(ctx, Person, FeaturedRef, "C-003", e10.Ref, app.RelationSupports)
	if !s.check(FeaturedRef+" E-010 supports C-003", err) {
		return
	}
	s.at(70 * time.Minute)
	_, err = a.SetClaimVerification(ctx, Person, FeaturedRef, "C-003", app.VerificationHumanVerified)
	if !s.check(FeaturedRef+" verify C-003", err) {
		return
	}
	s.at(65 * time.Minute)
	conclusion, err := a.UpdateConclusion(ctx, Person, FeaturedRef, app.ConclusionInput{
		Conclusion:          Conclusion,
		Confidence:          app.ConfidenceHigh,
		Rationale:           PersonRationale,
		SupportingClaims:    []string{"C-002", "C-003"},
		ContradictingClaims: []string{"C-001"},
	})
	if !s.check(FeaturedRef+" conclusion High", err) {
		return
	}
	_, err = a.UpdateConclusion(ctx, Person, FeaturedRef, app.ConclusionInput{
		Conclusion:          Conclusion,
		Confidence:          app.ConfidenceHigh,
		Rationale:           PersonRationale,
		Verification:        app.VerificationHumanVerified,
		Review:              true,
		ReviewBasis:         "Checked the implementation-services ledger, July playbook, deal records and the sales-ops explanation; discounting is disputed and travel remains open.",
		ExpectedReviewToken: conclusion.ConclusionReviewToken,
	})
	s.check(FeaturedRef+" conclusion reviewed", err)
}

// checkoutOutage opens INV-0003 on Monday afternoon.
func (s *seeder) checkoutOutage() {
	a, ctx := s.a, s.ctx
	if s.err != nil {
		return
	}
	s.at(3*day - 2*hour)
	inv, err := a.CreateInvestigation(ctx, CLI, app.CreateInvestigationInput{
		Title:            "Checkout outage on October 2",
		ProblemStatement: "Why was checkout down for 41 minutes on October 2, and why did it recover on its own?",
		Status:           app.InvestigationStatusInvestigating,
	})
	if !s.check("INV-0003", err) {
		return
	}
	s.at(3*day - 4*hour)
	e1, err := a.AddEvidence(ctx, Codex, inv.Ref, app.EvidenceInput{
		Title:       "Checkout errors ran from 14:07 to 14:48 UTC; every failure is a timeout on the payment authorization call",
		Summary:     "In the error store, checkout returned errors on every order from 14:07 to 14:48 UTC on October 2, 2,900 orders in all; every failure is a 10-second timeout on the payment provider's authorization call, and no other dependency shows an error in the window.",
		SourceType:  app.SourceTypeSQLQuery,
		SourceRef:   "observability/queries/checkout_errors_2026-10-02.sql",
		SourceLabel: "Error store query",
	})
	if !s.check("INV-0003 evidence 1", err) {
		return
	}
	e2, err := a.AddEvidence(ctx, Codex, inv.Ref, app.EvidenceInput{
		Title:       "Payment provider status page: authorization degraded 14:05 to 14:50 UTC in one region",
		Summary:     "The payment provider's status page records a degraded authorization service from 14:05 to 14:50 UTC on October 2 in the region that serves our traffic, with no cause given.",
		SourceType:  app.SourceTypeDocumentation,
		SourceRef:   "Payment provider status page, October 2",
		SourceLabel: "Payment provider status page",
	})
	if !s.check("INV-0003 evidence 2", err) {
		return
	}
	_, err = a.AddClaim(ctx, Codex, inv.Ref, app.ClaimInput{
		Statement:           "Checkout went down because the payment provider's authorization service degraded and checkout has no fallback, so every order timed out.",
		Confidence:          app.ConfidenceMedium,
		ConfidenceRationale: "The error window and the provider's status window match to the minute, but the provider has not said what failed.",
		Links:               []app.EvidenceLinkInput{supports(e1.Ref), supports(e2.Ref)},
	})
	s.check("INV-0003 claim", err)
}

// checkoutBlocked leaves INV-0003 blocked on the payment provider since
// yesterday.
func (s *seeder) checkoutBlocked() {
	a, ctx := s.a, s.ctx
	if s.err != nil {
		return
	}
	s.at(30 * hour)
	_, err := a.AddQuestion(ctx, Codex, "INV-0003", app.QuestionInput{
		Question: "Will the payment provider deliver a root cause report and a date for a fix?",
	})
	if !s.check("INV-0003 question", err) {
		return
	}
	_, err = a.UpdateInvestigation(ctx, CLI, "INV-0003", app.InvestigationPatch{Status: ptr(app.InvestigationStatusBlocked)})
	s.check("INV-0003 blocked", err)
}

// ssoAccounts is INV-0004, found and resolved within a few hours on
// Tuesday.
func (s *seeder) ssoAccounts() {
	a, ctx := s.a, s.ctx
	if s.err != nil {
		return
	}
	s.at(2*day - 2*hour)
	inv, err := a.CreateInvestigation(ctx, Person, app.CreateInvestigationInput{
		Title:            "31 former employees still have active SSO accounts",
		ProblemStatement: "Why do 31 people who left since July still have an active SSO account?",
		Status:           app.InvestigationStatusInvestigating,
	})
	if !s.check("INV-0004", err) {
		return
	}
	s.at(2*day - 3*hour)
	ev, err := a.AddEvidence(ctx, Codex, inv.Ref, app.EvidenceInput{
		Title:       "HR changes feed has returned 404 since July 14",
		Summary:     "The identity sync job's call to the HR system's employee-changes feed has returned 404 on every run since July 14, when HR moved to a new tenant URL; the job logs the error and exits 0, so no alert fired.",
		SourceType:  app.SourceTypeAPIResponse,
		SourceRef:   "identity-sync: GET /v1/employee-changes",
		SourceLabel: "HR system API",
	})
	if !s.check("INV-0004 evidence", err) {
		return
	}
	_, err = a.AddClaim(ctx, Codex, inv.Ref, app.ClaimInput{
		Statement:           "Offboarding stopped on July 14 because the HR feed moved and the sync job treats a 404 as a clean run.",
		Confidence:          app.ConfidenceHigh,
		ConfidenceRationale: "All 31 accounts belong to people who left after July 14.",
		Links:               []app.EvidenceLinkInput{supports(ev.Ref)},
	})
	if !s.check("INV-0004 claim", err) {
		return
	}
	s.at(2*day - 6*hour)
	_, err = a.UpdateConclusion(ctx, Person, inv.Ref, app.ConclusionInput{
		Conclusion: "The HR sync has failed silently since July 14, so nobody who left after that date was offboarded.",
		Confidence: app.ConfidenceHigh,
		Rationale:  "The API responses and C-001 account for all 31 accounts.",
	})
	if !s.check("INV-0004 conclusion", err) {
		return
	}
	_, err = a.UpdateInvestigation(ctx, Person, inv.Ref, app.InvestigationPatch{Status: ptr(app.InvestigationStatusResolved)})
	s.check("INV-0004 resolve", err)
}

// exportJob is INV-0005, resolved and archived yesterday; nothing is
// deleted.
func (s *seeder) exportJob() {
	a, ctx := s.a, s.ctx
	if s.err != nil {
		return
	}
	s.at(28 * hour)
	inv, err := a.CreateInvestigation(ctx, CLI, app.CreateInvestigationInput{
		Title:            "Nightly export skipped two runs",
		ProblemStatement: "Why did the nightly warehouse export skip the runs of September 29 and 30?",
		Status:           app.InvestigationStatusInvestigating,
	})
	if !s.check("INV-0005", err) {
		return
	}
	ev, err := a.AddEvidence(ctx, ClaudeCode, inv.Ref, app.EvidenceInput{
		Title:       "Scheduler paused the export during the database upgrade",
		Summary:     "The scheduler log shows the export trigger paused on September 29 at 21:58 for the database upgrade and resumed on October 1 at 07:10.",
		SourceType:  app.SourceTypeFile,
		SourceRef:   "scheduler/logs/2026-09-29.log",
		SourceLabel: "Scheduler log",
	})
	if !s.check("INV-0005 evidence", err) {
		return
	}
	_, err = a.AddClaim(ctx, ClaudeCode, inv.Ref, app.ClaimInput{
		Statement:  "The two runs were skipped because the trigger stayed paused after the upgrade window.",
		Confidence: app.ConfidenceHigh,
		Links:      []app.EvidenceLinkInput{supports(ev.Ref)},
	})
	if !s.check("INV-0005 claim", err) {
		return
	}
	s.at(27 * hour)
	_, err = a.UpdateConclusion(ctx, Person, inv.Ref, app.ConclusionInput{
		Conclusion: "The trigger was paused for the database upgrade and nobody resumed it; both runs were replayed.",
		Confidence: app.ConfidenceHigh,
		Rationale:  "The scheduler log covers both nights.",
	})
	if !s.check("INV-0005 conclusion", err) {
		return
	}
	_, err = a.UpdateInvestigation(ctx, Person, inv.Ref, app.InvestigationPatch{Status: ptr(app.InvestigationStatusResolved)})
	if !s.check("INV-0005 resolve", err) {
		return
	}
	s.at(26 * hour)
	_, err = a.ArchiveInvestigation(ctx, Person, inv.Ref)
	s.check("INV-0005 archive", err)
}
