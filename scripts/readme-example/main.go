// Command readme-example builds the README's Northeast margin investigation
// in a throwaway database and prints its context pack, so the example in the
// README is real engine output rather than hand-written text.
//
// Usage (from the repository root):
//
//	go run ./scripts/readme-example                    # compact pack, as casebook context --format compact
//	go run ./scripts/readme-example -format md         # full Markdown pack
//	go run ./scripts/readme-example -format json
//	go run ./scripts/readme-example -db ./demo.db      # keep the database (it must not exist yet)
//
// Every write goes through internal/app with the actors a real run would
// have: Claude Code and Codex over MCP ({ai, claude-code}, {ai, codex}) and
// a person in the desktop app ({human, casebook-ui}). The clock is fixed, so
// the output is byte for byte the same on every run.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jameskbb/casebook/internal/app"
)

func main() {
	format := flag.String("format", "compact", "output format: compact, md or json")
	maxBytes := flag.Int("max-bytes", 0, "Markdown size budget (0: the format's default)")
	dbPath := flag.String("db", "", "database file to create (default: a temporary file that is removed)")
	flag.Parse()

	if err := run(*dbPath, *format, *maxBytes, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "readme-example:", err)
		os.Exit(1)
	}
}

// run builds the scenario in dbPath (a temporary directory when empty) and
// writes the rendered pack of INV-0002 to w.
func run(dbPath, format string, maxBytes int, w io.Writer) error {
	if dbPath == "" {
		dir, err := os.MkdirTemp("", "casebook-readme-example-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		dbPath = filepath.Join(dir, "casebook.db")
	} else if _, err := os.Stat(dbPath); err == nil {
		return fmt.Errorf("%s already exists; pass a new path", dbPath)
	}

	clk := &clock{}
	a, err := app.Open(dbPath, app.WithClock(clk.now))
	if err != nil {
		return err
	}
	defer a.Close()

	if err := build(context.Background(), a, clk); err != nil {
		return err
	}

	name, compact := format, false
	switch format {
	case "compact":
		name, compact = "md", true
	case "md", "markdown", "json":
	default:
		return fmt.Errorf("unknown format %q (want compact, md or json)", format)
	}
	text, err := a.RenderContext(context.Background(), "INV-0002", name,
		app.ContextOptions{Compact: compact, MaxBytes: maxBytes})
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, text)
	return err
}

// clock is a deterministic clock: day sets the date and time, and each call
// advances by one minute so every change has its own timestamp.
type clock struct{ t time.Time }

func (c *clock) now() time.Time {
	t := c.t
	c.t = c.t.Add(time.Minute)
	return t
}

func (c *clock) at(day, hour, minute int) {
	c.t = time.Date(2026, time.October, day, hour, minute, 0, 0, time.UTC)
}

var (
	claude = app.ActorMCP("claude-code")
	codex  = app.ActorMCP("codex")
	person = app.ActorDesktop()
)

// step records the first error so build reads as the story it tells.
type step struct {
	ctx context.Context
	a   *app.App
	err error
}

func (s *step) do(what string, f func() error) {
	if s.err != nil {
		return
	}
	if err := f(); err != nil {
		s.err = fmt.Errorf("%s: %w", what, err)
	}
}

func ptr[T any](v T) *T { return &v }

func supports(refs ...string) []app.EvidenceLinkInput {
	out := make([]app.EvidenceLinkInput, 0, len(refs))
	for _, r := range refs {
		out = append(out, app.EvidenceLinkInput{EvidenceRef: r, Relation: app.RelationSupports})
	}
	return out
}

// build writes the canonical scenario: INV-0001 is an earlier, finished
// case, and INV-0002 is "Northeast margin down 6 points in Q3" over three
// sessions and two AI clients, finished by a person.
func build(ctx context.Context, a *app.App, clk *clock) error {
	s := &step{ctx: ctx, a: a}
	const inv = "INV-0002"

	addEvidence := func(actor app.Actor, in app.EvidenceInput) {
		s.do("add evidence "+in.Title, func() error {
			_, err := a.AddEvidence(ctx, actor, inv, in)
			return err
		})
	}
	addHypothesis := func(actor app.Actor, ref string, in app.HypothesisInput) {
		s.do("add "+ref, func() error {
			_, err := a.AddHypothesis(ctx, actor, inv, in)
			return err
		})
	}
	addQuestion := func(actor app.Actor, ref, q string) {
		s.do("add "+ref, func() error {
			_, err := a.AddQuestion(ctx, actor, inv, app.QuestionInput{Question: q})
			return err
		})
	}
	addClaim := func(actor app.Actor, ref string, in app.ClaimInput) {
		s.do("add "+ref, func() error {
			_, err := a.AddClaim(ctx, actor, inv, in)
			return err
		})
	}

	// An earlier, finished case, so the margin case is INV-0002.
	clk.at(1, 9, 0)
	s.do("create INV-0001", func() error {
		_, err := a.CreateInvestigation(ctx, person, app.CreateInvestigationInput{
			Title:            "TestCheckoutTotals flakes in CI",
			ProblemStatement: "Why does TestCheckoutTotals fail about once in 20 CI runs and never locally?",
			Status:           app.InvestigationStatusResolved,
		})
		return err
	})

	// The person opens the case and makes it active.
	clk.at(5, 9, 30)
	s.do("create INV-0002", func() error {
		_, err := a.CreateInvestigation(ctx, person, app.CreateInvestigationInput{
			Title:            "Northeast margin down 6 points in Q3",
			ProblemStatement: "Why did gross margin in the Northeast fall from 41% to 35% in Q3 when revenue was up 4%?",
			Status:           app.InvestigationStatusInvestigating,
			MakeActive:       true,
		})
		return err
	})

	// Day 1, Monday: Claude Code.
	clk.at(5, 10, 0)
	addEvidence(claude, app.EvidenceInput{
		Title:       "Northeast Q3: revenue $27.0M (+4%), cost of delivery $17.6M (+15%), gross margin 35% against 41%",
		Summary:     "The Q3 regional P&L shows Northeast revenue of $27.0M against $26.0M in Q2 and cost of delivery of $17.6M against $15.3M, taking gross margin from 41.2% to 34.8%; the P&L carries cost of delivery as one line.",
		SourceType:  app.SourceTypeFile,
		SourceRef:   "finance/pnl/northeast-2026-Q3.xlsx",
		SourceLabel: "Q3 regional P&L",
		Metadata:    `{"q2_revenue_usd":26000000,"q3_revenue_usd":27000000,"q2_cost_usd":15300000,"q3_cost_usd":17600000}`,
	})
	addEvidence(claude, app.EvidenceInput{
		Title:       "Q3 bookings: 47 new Northeast deals, seven above $250k, the two biggest with Pinecrest Logistics",
		Summary:     "The Q3 bookings report lists 47 new Northeast deals, seven with ACV above $250k, including the two biggest, both with Pinecrest Logistics, signed July 8 and July 22.",
		SourceType:  app.SourceTypeFile,
		SourceRef:   "salesops/reports/northeast-bookings-2026-Q3.csv",
		SourceLabel: "Q3 bookings report",
	})
	addEvidence(claude, app.EvidenceInput{
		Title:       "Sales VP: we cut prices hard to win the quarter's two biggest deals",
		Summary:     "The sales VP said in the Monday pipeline call that the region cut prices hard to win the quarter's two biggest deals in July and expects that to explain the margin drop.",
		SourceType:  app.SourceTypeUserStatement,
		SourceLabel: "Sales VP, in the pipeline call",
	})
	addEvidence(claude, app.EvidenceInput{
		Title:   "Onboarding team added four contractors in July",
		Summary: "The contractor roster shows four implementation contractors starting in the Northeast onboarding team on July 13 and July 20, at $135 to $145 an hour.",
		RawDetail: "contractor,role,start,rate_usd_per_hour\n" +
			"Ardent Consulting (3 seats),implementation lead,2026-07-13,145\n" +
			"Northline Staffing (1 seat),implementation engineer,2026-07-20,135",
		SourceType:  app.SourceTypeFile,
		SourceRef:   "people/contractors/northeast-onboarding-2026.csv",
		SourceLabel: "Contractor roster",
	})
	addHypothesis(claude, "H-001", app.HypothesisInput{
		Statement:  "July contractor hires raised the cost of each onboarding in the Northeast.",
		Confidence: app.ConfidenceMedium,
		Links:      supports("E-004"),
	})
	addHypothesis(claude, "H-002", app.HypothesisInput{
		Statement:  "The weaker Canadian dollar inflated delivery cost on the Canadian accounts.",
		Confidence: app.ConfidenceLow,
	})
	addClaim(claude, "C-001", app.ClaimInput{
		Statement:           "Deep discounts on the quarter's two biggest deals caused most of the margin drop.",
		Confidence:          app.ConfidenceLow,
		ConfidenceRationale: "The sales VP's reading of the quarter; not yet checked against the deal desk.",
		Links:               supports("E-003"),
	})
	addQuestion(claude, "Q-001", "Which cost of delivery lines moved? The P&L has one line for cost of delivery; the cost ledger has it by line.")

	// Day 2, Tuesday: Codex, with SQL access to the cost ledger and the
	// deal desk tables.
	clk.at(6, 14, 0)
	addEvidence(codex, app.EvidenceInput{
		Title:   "Implementation services are $1.9M of the $2.3M cost rise; contractor rate flat, hours up 78%",
		Summary: "In the cost ledger, implementation services rose from $2.4M to $4.3M, which is $1.9M of the $2.3M quarter-over-quarter rise; travel and expenses +$210k, customer support +$110k, hosting +$90k, FX revaluation +$40k, everything else -$50k. The blended contractor rate is $141 an hour against $140; hours are 30,400 against 17,100.",
		RawDetail: "SELECT cost_line,\n" +
			"       SUM(CASE WHEN fiscal_quarter = '2026-Q3' THEN amount_usd END)\n" +
			"     - SUM(CASE WHEN fiscal_quarter = '2026-Q2' THEN amount_usd END) AS delta_usd\n" +
			"FROM finance.cost_of_delivery\n" +
			"WHERE region = 'Northeast' AND fiscal_quarter IN ('2026-Q2', '2026-Q3')\n" +
			"GROUP BY cost_line\n" +
			"ORDER BY delta_usd DESC;\n" +
			"-- implementation_services +1,900,000 | travel_and_expenses +210,000 | customer_support +110,000 | hosting +90,000 | fx_revaluation +40,000 | other -50,000\n" +
			"-- implementation_services: 30,400 hours at $141/h (Q2: 17,100 hours at $140/h)",
		SourceType:  app.SourceTypeSQLQuery,
		SourceRef:   "finance/queries/cost_of_delivery_by_line_qoq.sql",
		SourceLabel: "Cost ledger query",
		Metadata:    `{"cost_rise_usd":2300000,"implementation_services_usd":1900000,"q2_cost_usd":15300000,"q3_cost_usd":17600000}`,
	})
	for _, h := range []string{"H-001", "H-002"} {
		s.do("disprove "+h, func() error {
			if _, err := a.LinkEvidenceToHypothesis(ctx, codex, inv, h, "E-005", app.RelationContradicts); err != nil {
				return err
			}
			_, err := a.UpdateHypothesis(ctx, codex, inv, h, app.HypothesisPatch{
				Status: ptr(app.HypothesisStatusDisproven),
			})
			return err
		})
	}
	s.do("resolve Q-001", func() error {
		_, err := a.ResolveQuestion(ctx, codex, inv, "Q-001", "Implementation services +$1.9M, travel +$210k, customer support +$110k, hosting +$90k, FX revaluation +$40k; everything else -$50k (E-005).")
		return err
	})
	addEvidence(codex, app.EvidenceInput{
		Title:   "Discounts are flat: 11.6% in Q3 against 11.8% in Q2; the two biggest deals got 12% and 14%",
		Summary: "Across the 47 Northeast deals booked in Q3 the average discount is 11.6% against 11.8% over Q2's 44 deals; the two biggest deals got 12% and 14%, inside the 8% to 16% range of the last four quarters.",
		RawDetail: "SELECT fiscal_quarter, AVG(discount_pct) AS avg_discount, COUNT(*) AS deals\n" +
			"FROM salesops.closed_deals\n" +
			"WHERE region = 'Northeast' AND fiscal_quarter IN ('2026-Q2', '2026-Q3')\n" +
			"GROUP BY fiscal_quarter;\n" +
			"-- 2026-Q2 11.8% (44 deals) | 2026-Q3 11.6% (47 deals)\n" +
			"-- Pinecrest Logistics: NE-2311 12.0%, NE-2318 14.0%; last four quarters range 8% to 16%",
		SourceType:  app.SourceTypeSQLQuery,
		SourceRef:   "salesops/queries/discount_rate_by_quarter.sql",
		SourceLabel: "Deal desk discount query",
		Metadata:    `{"q2_discount":0.118,"q3_discount":0.116,"biggest_deal_discounts":[0.12,0.14]}`,
	})
	// An AI may raise doubt about a claim; it may not settle it.
	s.do("dispute C-001", func() error {
		if _, err := a.LinkEvidenceToClaim(ctx, codex, inv, "C-001", "E-006", app.RelationContradicts); err != nil {
			return err
		}
		_, err := a.SetClaimVerification(ctx, codex, inv, "C-001", app.VerificationDisputed)
		return err
	})
	addEvidence(codex, app.EvidenceInput{
		Title:   "Three accounts moved to white-glove onboarding in July carry $1.4M of the $1.9M",
		Summary: "Implementation cost by account: Pinecrest Logistics, Pellston Health and Ridgeway Mutual moved to the white-glove onboarding tier in July and carry $1.4M of the $1.9M rise; three later deals above $250k add $300k together and the standard tier $200k.",
		RawDetail: "SELECT account, onboarding_tier, SUM(hours * rate_usd) AS cost_usd\n" +
			"FROM finance.implementation_hours\n" +
			"WHERE region = 'Northeast' AND fiscal_quarter = '2026-Q3'\n" +
			"GROUP BY account, onboarding_tier\n" +
			"ORDER BY cost_usd DESC;\n" +
			"-- rise against Q2, white-glove since July: Pinecrest Logistics +700,000 | Pellston Health +400,000 | Ridgeway Mutual +300,000\n" +
			"-- white-glove, signed August to September: Cobalt Freight, Marlowe Energy, Tilden Schools +300,000 together\n" +
			"-- standard tier, all other accounts: +200,000",
		SourceType:  app.SourceTypeSQLQuery,
		SourceRef:   "finance/queries/implementation_cost_by_account.sql",
		SourceLabel: "Implementation hours query",
		Metadata:    `{"white_glove_accounts":3,"white_glove_rise_usd":1400000}`,
	})
	addClaim(codex, "C-002", app.ClaimInput{
		Statement:           "Implementation services are $1.9M of the $2.3M cost rise; three accounts moved to white-glove onboarding in July carry $1.4M of it.",
		Confidence:          app.ConfidenceHigh,
		ConfidenceRationale: "The cost ledger by line and by account agree.",
		Links:               supports("E-001", "E-005", "E-007"),
	})
	addHypothesis(codex, "H-003", app.HypothesisInput{
		Statement:  "A July change in sales policy, not account-by-account decisions, put the new accounts on white-glove onboarding.",
		Confidence: app.ConfidenceMedium,
		Links:      supports("E-007"),
	})
	addHypothesis(codex, "H-004", app.HypothesisInput{
		Statement:  "Travel for on-site white-glove sessions is growing with the same accounts.",
		Confidence: app.ConfidenceMedium,
		Links:      supports("E-005"),
	})
	addQuestion(codex, "Q-002", "What changed in July that moved three accounts to white-glove onboarding at once?")

	// Day 4, Thursday: Claude Code again, starting from the compact pack.
	clk.at(8, 9, 0)
	addEvidence(claude, app.EvidenceInput{
		Title:   "July 6 playbook update: white-glove onboarding included free on every deal above $250k",
		Summary: `The Northeast sales playbook revised on July 6 (v7) adds "White-glove onboarding is included at no charge on every deal with ACV above $250k" to the standard offer; v6 (April 2) listed white-glove onboarding as a priced add-on at $120k to $180k.`,
		RawDetail: "Northeast Sales Playbook, section 4 \"Standard offer\"\n" +
			"- v6 (2026-04-02): White-glove onboarding: priced add-on, $120k to $180k by deal size.\n" +
			"+ v7 (2026-07-06): White-glove onboarding is included at no charge on every deal with ACV above $250k.",
		SourceType:   app.SourceTypeDocumentation,
		SourceRef:    "Sales shared drive: Northeast Sales Playbook v7 (2026-07-06), section 4",
		SourceLabel:  "Sales playbook v7",
		Verification: app.VerificationSourceVerified,
		Metadata:     `{"revised":"2026-07-06","threshold_usd":250000}`,
	})
	addEvidence(claude, app.EvidenceInput{
		Title:   "Deal desk: seven Northeast deals above $250k since July 6, onboarding fee $0 on every one",
		Summary: "The deal desk approvals export lists seven Northeast deals above $250k closed from July 6 to September 30, every one on the white-glove tier with an onboarding fee of $0; the four Q2 deals of that size carried fees of $120k to $180k.",
		RawDetail: "deal_id,account,signed,acv_usd,onboarding_tier,onboarding_fee_usd\n" +
			"NE-2311,Pinecrest Logistics,2026-07-08,1120000,white-glove,0\n" +
			"NE-2318,Pinecrest Logistics,2026-07-22,760000,white-glove,0\n" +
			"NE-2320,Pellston Health,2026-07-14,540000,white-glove,0\n" +
			"NE-2327,Ridgeway Mutual,2026-07-29,410000,white-glove,0\n" +
			"NE-2340,Cobalt Freight,2026-08-11,330000,white-glove,0\n" +
			"NE-2352,Marlowe Energy,2026-09-02,290000,white-glove,0\n" +
			"NE-2361,Tilden Schools,2026-09-24,265000,white-glove,0",
		SourceType:  app.SourceTypeFile,
		SourceRef:   "salesops/exports/northeast-approvals-2026-Q3.csv",
		SourceLabel: "Deal desk approvals export",
	})
	addClaim(claude, "C-003", app.ClaimInput{
		Statement:           "The July 6 playbook update made white-glove onboarding free on every deal above $250k; seven Northeast deals got it and none was charged.",
		Confidence:          app.ConfidenceHigh,
		ConfidenceRationale: "The playbook text and the deal desk records agree; the three accounts in E-007 are among the seven.",
		Links:               supports("E-007", "E-008", "E-009"),
	})
	s.do("support H-003", func() error {
		_, err := a.UpdateHypothesis(ctx, claude, inv, "H-003", app.HypothesisPatch{
			Status:     ptr(app.HypothesisStatusSupported),
			Confidence: ptr(app.ConfidenceHigh),
		})
		return err
	})
	s.do("resolve Q-002", func() error {
		_, err := a.ResolveQuestion(ctx, claude, inv, "Q-002", "The Northeast sales playbook revised on July 6 made white-glove onboarding free on every deal above $250k (E-008).")
		return err
	})
	s.do("AI conclusion", func() error {
		_, err := a.UpdateConclusion(ctx, claude, inv, app.ConclusionInput{
			Conclusion: conclusion,
			Confidence: app.ConfidenceMedium,
			Rationale:  "C-002 sizes the cost and C-003 explains it; the playbook text is the only word so far on who meant it for whom, and H-004 is still open.",
		})
		return err
	})
	// The engine refuses an AI's attempt to mark its own claim human verified.
	s.do("AI human_verified refusal", func() error {
		_, err := a.SetClaimVerification(ctx, claude, inv, "C-003", app.VerificationHumanVerified)
		if err == nil {
			return errors.New("the engine let an AI mark C-003 human verified")
		}
		return nil
	})

	// Thursday afternoon: the person asks the regional sales-ops lead,
	// records what they said, verifies C-003 and raises the conclusion
	// confidence.
	clk.at(8, 16, 0)
	addQuestion(person, "Q-003", "Can onboarding be charged on the Q4 deals already signed under the July playbook?")
	addQuestion(person, "Q-004", "Should the Q3 regional forecast be restated for the onboarding that was delivered and never charged?")
	s.do("defer Q-004", func() error {
		_, err := a.DeferQuestion(ctx, person, inv, "Q-004", "Waiting for the quarter close on October 10.")
		return err
	})
	addEvidence(person, app.EvidenceInput{
		Title:       "Regional sales-ops lead: the free white-glove clause was written for the two Pinecrest deals only",
		Summary:     "The regional sales-ops lead said in person that the July 6 playbook clause was written to close the two Pinecrest deals and was never meant to apply to every deal above $250k; nobody reviewed it after Pinecrest signed.",
		SourceType:  app.SourceTypeUserStatement,
		SourceLabel: "Sales-ops lead, in person",
	})
	s.do("link E-010 to C-003", func() error {
		_, err := a.LinkEvidenceToClaim(ctx, person, inv, "C-003", "E-010", app.RelationSupports)
		return err
	})
	s.do("person verifies C-003", func() error {
		_, err := a.SetClaimVerification(ctx, person, inv, "C-003", app.VerificationHumanVerified)
		return err
	})
	s.do("person raises conclusion", func() error {
		const rationale = "The playbook text (E-008), the deal desk records (E-009) and the sales-ops lead (E-010) agree; travel on the same accounts (H-004) and $400k of smaller lines are still open."
		saved, err := a.UpdateConclusion(ctx, person, inv, app.ConclusionInput{
			Conclusion:          conclusion,
			Confidence:          app.ConfidenceHigh,
			Rationale:           rationale,
			SupportingClaims:    []string{"C-002", "C-003"},
			ContradictingClaims: []string{"C-001"},
		})
		if err != nil {
			return err
		}
		_, err = a.UpdateConclusion(ctx, person, inv, app.ConclusionInput{
			Conclusion:          conclusion,
			Confidence:          app.ConfidenceHigh,
			Rationale:           rationale,
			Verification:        app.VerificationHumanVerified,
			Review:              true,
			ReviewBasis:         "Checked the ledger, playbook, deal records and sales-ops statement; discounting is disputed and travel remains open.",
			ExpectedReviewToken: saved.ConclusionReviewToken,
		})
		return err
	})
	return s.err
}

const conclusion = "White-glove onboarding, given away free on every deal above $250k since July, drove most of the margin drop."
