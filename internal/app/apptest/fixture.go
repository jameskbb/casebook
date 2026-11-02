package apptest

import (
	"context"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

// Fixture holds the refs SeedCloudSpend created, taken from the DTOs the
// app returned.
type Fixture struct {
	InvestigationRef       string
	Evidence               []string // E-001..E-004
	Claims                 []string // C-001..C-003
	Hypotheses             []string // H-001..H-002
	Questions              []string // Q-001
	SupersededClaimRef     string   // C-003, superseded by C-002
	DisprovenHypothesisRef string   // H-002
}

// SeedCloudSpend builds the demo case investigation through public
// internal/app methods only (D-27):
//
//   - INV "September cloud bill up 38%" (status investigating, active)
//   - E-001 code, E-002 documentation, E-003 sql_query (metadata
//     {"affected_records":214}), E-004 user_statement
//   - C-001 high, supported by E-001 and E-004
//   - C-002 high, supported by E-002 and E-003
//   - C-003 low, contradicted by E-003, superseded by C-002
//   - H-001 medium, open; H-002 low, contradicted by E-003, disproven
//   - Q-001 open
//   - conclusion "Debug logging requested for payments only reached every
//     service and drove most of the increase." high
//
// Use it on a fresh App (refs assume an empty database). With apptest.Open's
// fixed clock the result is deterministic. It fails the test on any error.
func SeedCloudSpend(t testing.TB, a *app.App) Fixture {
	t.Helper()
	ctx := context.Background()
	must := func(step string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("apptest: seed %s: %v", step, err)
		}
	}
	var f Fixture

	inv, err := a.CreateInvestigation(ctx, Human, app.CreateInvestigationInput{
		Title:            "September cloud bill up 38%",
		ProblemStatement: "Why did the cloud bill rise 38% in September when traffic was flat?",
		Status:           app.InvestigationStatusInvestigating,
	})
	must("investigation", err)
	f.InvestigationRef = inv.Ref
	ref := inv.Ref

	evidence := []struct {
		actor app.Actor
		in    app.EvidenceInput
	}{
		{AI, app.EvidenceInput{
			Title:       "Service chart default log level set to debug",
			Summary:     "A platform change merged on August 28 changed the shared service chart's default log level from info to debug, so every service picks up debug logging at its next deploy.",
			SourceType:  app.SourceTypeCode,
			SourceRef:   "platform-charts/service/values.yaml",
			SourceLabel: "Platform Helm chart",
			RawDetail: "--- a/platform-charts/service/values.yaml\n" +
				"+++ b/platform-charts/service/values.yaml\n" +
				" logging:\n" +
				"-  level: info\n" +
				"+  level: debug\n" +
				"   format: json",
		}},
		{AI, app.EvidenceInput{
			Title:       "Logs are billed per GB ingested in the month",
			Summary:     "The provider's pricing page bills logs per GB ingested, with no volume discount below 500 TB a month, so a jump in log volume lands on the same month's bill in full.",
			SourceType:  app.SourceTypeDocumentation,
			SourceRef:   "Provider pricing page, Logs and monitoring section",
			SourceLabel: "Provider pricing page",
		}},
		{AI, app.EvidenceInput{
			Title:       "Log ingestion is 71% of the September increase",
			Summary:     "In the billing export, log ingestion rose from $12,100 to $108,500, which is $96,400 of the $135,800 month-over-month increase, spread across 214 services with none above 4% of the total; compute and network moved less than 2%.",
			SourceType:  app.SourceTypeSQLQuery,
			SourceRef:   "finops/queries/month_over_month_by_line_item.sql",
			SourceLabel: "Billing export query",
			RawDetail: "SELECT line_item,\n" +
				"       SUM(CASE WHEN usage_month = '2026-09' THEN cost END)\n" +
				"     - SUM(CASE WHEN usage_month = '2026-08' THEN cost END) AS delta_usd\n" +
				"FROM billing.monthly_costs\n" +
				"WHERE usage_month IN ('2026-08', '2026-09')\n" +
				"GROUP BY line_item\n" +
				"ORDER BY delta_usd DESC;\n" +
				"-- log_ingestion +96,400 | traces +29,800 | other +8,300 | compute +1,900 | network -600",
			Metadata: `{"affected_records":214}`,
		}},
		{Human, app.EvidenceInput{
			Title:       "Head of platform: debug was requested for payments only",
			Summary:     "The head of platform engineering said the payments team asked for debug logs for the card network audit, and the change went in as the chart default instead of in the payments values file.",
			SourceType:  app.SourceTypeUserStatement,
			SourceLabel: "Head of platform, in person",
		}},
	}
	for _, e := range evidence {
		ev, err := a.AddEvidence(ctx, e.actor, ref, e.in)
		must("evidence "+e.in.Title, err)
		f.Evidence = append(f.Evidence, ev.Ref)
	}
	e1, e2, e3, e4 := f.Evidence[0], f.Evidence[1], f.Evidence[2], f.Evidence[3]
	supports := func(r string) app.EvidenceLinkInput {
		return app.EvidenceLinkInput{EvidenceRef: r, Relation: app.RelationSupports}
	}
	contradicts := func(r string) app.EvidenceLinkInput {
		return app.EvidenceLinkInput{EvidenceRef: r, Relation: app.RelationContradicts}
	}

	claims := []struct {
		actor app.Actor
		in    app.ClaimInput
	}{
		{Human, app.ClaimInput{
			Statement:           "A shared platform default moved every service to debug logging on August 28; only payments asked for it.",
			Confidence:          app.ConfidenceHigh,
			ConfidenceRationale: "The chart diff and the head of platform agree.",
			Links:               []app.EvidenceLinkInput{supports(e1), supports(e4)},
		}},
		{AI, app.ClaimInput{
			Statement:           "Log ingestion, billed per GB, is $96,400 of the $135,800 increase; compute and network are flat.",
			Confidence:          app.ConfidenceHigh,
			ConfidenceRationale: "The pricing model and the billing export agree.",
			Links:               []app.EvidenceLinkInput{supports(e2), supports(e3)},
		}},
		{AI, app.ClaimInput{
			Statement:  "The increase comes from the August autoscaling change adding compute nodes.",
			Confidence: app.ConfidenceLow,
			Links:      []app.EvidenceLinkInput{contradicts(e3)},
		}},
	}
	for _, c := range claims {
		got, err := a.AddClaim(ctx, c.actor, ref, c.in)
		must("claim", err)
		f.Claims = append(f.Claims, got.Ref)
	}
	superseded, err := a.SupersedeClaim(ctx, AI, ref, f.Claims[2], f.Claims[1])
	must("supersede claim", err)
	f.SupersededClaimRef = superseded.Ref

	h1, err := a.AddHypothesis(ctx, AI, ref, app.HypothesisInput{
		Statement:  "The new tracing agent may also be shipping some services' logs twice.",
		Confidence: app.ConfidenceMedium,
		Status:     app.HypothesisStatusOpen,
	})
	must("hypothesis H-001", err)
	h2, err := a.AddHypothesis(ctx, AI, ref, app.HypothesisInput{
		Statement:  "Data transfer from the region move doubled the network line.",
		Confidence: app.ConfidenceLow,
		Rationale:  "Network spend fell $600 month over month.",
		Links:      []app.EvidenceLinkInput{contradicts(e3)},
	})
	must("hypothesis H-002", err)
	disproven := app.HypothesisStatusDisproven
	h2, err = a.UpdateHypothesis(ctx, AI, ref, h2.Ref, app.HypothesisPatch{Status: &disproven})
	must("disprove hypothesis", err)
	f.Hypotheses = []string{h1.Ref, h2.Ref}
	f.DisprovenHypothesisRef = h2.Ref

	q, err := a.AddQuestion(ctx, Human, ref, app.QuestionInput{
		Question: "Can every service except payments go back to info level today?",
	})
	must("question", err)
	f.Questions = []string{q.Ref}

	_, err = a.UpdateConclusion(ctx, Human, ref, app.ConclusionInput{
		Conclusion: "Debug logging requested for payments only reached every service and drove most of the increase.",
		Confidence: app.ConfidenceHigh,
		Rationale:  "The change, the pricing page and the billing export agree; the remaining 29% is still open.",
	})
	must("conclusion", err)
	_, err = a.SetActiveInvestigation(ctx, Human, ref)
	must("set active", err)
	return f
}
