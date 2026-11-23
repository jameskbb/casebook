// Package demo writes the demo questions used for the README images, the
// screenshot pass and the Windows smoke. Every example is an everyday
// business question in plain words. The featured one is INV-0002 "Why did
// calls go up 30% in October when sales stayed the same?": three sessions
// over four days, Claude Code (Monday, Thursday) and Codex (Tuesday), and
// one person who reads the billing team's notice, talks to the billing team
// lead and checks the answer. INV-0001 is the March payroll question, answered
// last week (its answer is not checked yet); INV-0003 is the help desk
// question, every source checked and still without an answer;
// the others fill the list with an answered and an archived question.
//
// Everything is written through public internal/app methods with the
// actors a real session would use, so "How we got here" reads as it would
// after real work. The desktop --seed-demo flag and desktop/tools/aiwrite
// are the only callers.
package demo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jameskbb/casebook/internal/app"
)

// FeaturedRef is the question the README images show.
const FeaturedRef = "INV-0002"

// FeaturedTitle is the featured question.
const FeaturedTitle = "Why did calls go up 30% in October when sales stayed the same?"

// Stage says how much of the story Seed writes.
type Stage string

const (
	// StageFull writes the whole story, ending with the person's check of
	// the answer.
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
const ThursdayNote = "The billing team's notice says 1,900 customers were charged twice on October 3. " +
	"Most October calls about a bill mention being charged twice. " +
	"That explains the extra calls."

// ThursdayCheckpoint is Thursday's Claude Code session, saved in one
// casebook_checkpoint call on INV-0002: E-007 (checked against the source),
// E-008, C-003 linked to E-006, E-007 and E-008, H-003 supported, H-004
// added, Q-002 resolved, and the answer.
func ThursdayCheckpoint() app.CheckpointInput {
	return app.CheckpointInput{
		Note: ThursdayNote,
		Evidence: []app.CheckpointEvidence{
			{Key: "notice", Evidence: app.EvidenceInput{
				Title:        "Notice dated October 4: 1,900 customers were charged twice on October 3",
				Summary:      "The billing team emailed all staff on October 4. A mistake in the bills sent on October 3 charged 1,900 customers twice. The team said refunds would go out within a week.",
				SourceType:   app.SourceTypeDocumentation,
				SourceRef:    "Email to all staff, October 4, subject: Customers charged twice on October 3",
				SourceLabel:  "Email from the billing team",
				Verification: app.VerificationSourceVerified,
				RawDetail: "From: Billing team\n" +
					"Date: October 4\n" +
					"Subject: Customers charged twice on October 3\n\n" +
					"The bills we sent yesterday charged 1,900 customers twice. We found the mistake this morning.\n" +
					"Refunds will go out within a week. Customers may call about it.",
				Metadata: `{"customers_charged_twice":1900,"date":"October 3"}`,
			}},
			{Key: "callnotes", Evidence: app.EvidenceInput{
				Title:       "Call notes: 9 in 10 October calls about a bill mention being charged twice",
				Summary:     "Of the 1,070 calls about a bill in October, 960 have a note that says the customer was charged twice. In September none did.",
				SourceType:  app.SourceTypeFile,
				SourceRef:   "Call log, October, notes column",
				SourceLabel: "Call log notes",
			}},
		},
		Claims: []app.ClaimInput{{
			Statement:           "A billing mistake on October 3 charged some customers twice.",
			Confidence:          app.ConfidenceHigh,
			ConfidenceRationale: "The billing team's notice and the call notes say the same thing. The extra calls start on October 3.",
			Links:               []app.EvidenceLinkInput{supports("E-006"), supports("#notice"), supports("#callnotes")},
		}},
		Hypotheses: []app.HypothesisInput{{
			Statement:  "Customers who were charged twice are less likely to buy again.",
			Confidence: app.ConfidenceLow,
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
			Resolution: ptr("A mistake in the bills sent on October 3 charged 1,900 customers twice."),
		}},
		Conclusion: &app.ConclusionInput{
			Conclusion: Conclusion,
			Confidence: app.ConfidenceMedium,
			Rationale:  "The call log and the sales report agree that calls to buy stayed the same. The billing mistake explains the extra calls. Nobody has checked the billing team's notice yet.",
		},
	}
}

// Conclusion is the featured question's answer.
const Conclusion = "The extra calls were customers asking about a bill. A billing mistake on October 3 charged some customers twice. About the same number of people called to buy, so sales stayed the same."

// PersonRationale is the person's why when they review the answer.
const PersonRationale = "The billing team's notice, the call log and the billing team lead agree. Whether those customers buy less later is not tested yet."

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

// run writes the scenes in time order (the clock never moves back). Refs
// follow creation order: INV-0001 March payroll (last week, answered),
// INV-0002 the call center calls (Monday), INV-0003 help desk tickets (no
// answer yet), INV-0004 customers who stopped ordering (answered), INV-0005
// the office supplies order (archived).
func (s *seeder) run(stage Stage) {
	s.payroll()
	s.callsMonday()
	s.helpDesk()
	s.callsTuesday()
	s.lostCustomers()
	s.helpDeskStuck()
	s.suppliesOrder()
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

// payroll is INV-0001, a small question worked and answered last week:
// H-001 (overtime) ruled out, every source checked by the person, the
// answer rewritten by them and not checked yet.
func (s *seeder) payroll() {
	a, ctx := s.a, s.ctx
	s.at(9 * day)
	inv, err := a.CreateInvestigation(ctx, ClaudeCode, app.CreateInvestigationInput{
		Title:            "Why was March payroll $18,000 over budget?",
		ProblemStatement: "March payroll was $418,000. The budget was $400,000. Where did the extra $18,000 go?",
		Status:           app.InvestigationStatusInvestigating,
	})
	if !s.check("INV-0001", err) {
		return
	}
	_, err = a.CheckpointBatch(ctx, ClaudeCode, inv.Ref, app.CheckpointInput{
		Note: "March payroll was $418,000 against a budget of $400,000. " +
			"Overtime was the same in February and March, so extra overtime is ruled out.",
		Evidence: []app.CheckpointEvidence{
			{Key: "payroll", Evidence: app.EvidenceInput{
				Title:       "March payroll: $418,000 paid, $400,000 in the budget",
				Summary:     "The March payroll report shows $418,000 paid. The budget for March was $400,000.",
				SourceType:  app.SourceTypeFile,
				SourceRef:   "Payroll report, March",
				SourceLabel: "Payroll report",
				RawDetail: "February: $400,000 paid, $400,000 in the budget\n" +
					"March: $418,000 paid, $400,000 in the budget",
			}},
			{Key: "overtime", Evidence: app.EvidenceInput{
				Title:       "Overtime pay: $6,000 in February, $6,000 in March",
				Summary:     "The payroll report lists $6,000 of overtime pay in February and $6,000 in March.",
				SourceType:  app.SourceTypeFile,
				SourceRef:   "Payroll report, February and March",
				SourceLabel: "Payroll report",
			}},
		},
		Hypotheses: []app.HypothesisInput{{
			Statement:  "Extra overtime in March pushed payroll over budget.",
			Confidence: app.ConfidenceMedium,
			Links:      []app.EvidenceLinkInput{contradicts("#overtime")},
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
	hires, err := a.AddEvidence(ctx, ClaudeCode, inv.Ref, app.EvidenceInput{
		Title:       "Two new hires started on March 1. The budget had them starting on April 1",
		Summary:     "The list of start dates shows two new hires who started on March 1. The budget planned for both to start on April 1. Together they are paid $18,000 a month.",
		SourceType:  app.SourceTypeFile,
		SourceRef:   "New hire start dates, this year",
		SourceLabel: "List of start dates",
	})
	if !s.check("INV-0001 E-003", err) {
		return
	}
	_, err = a.AddClaim(ctx, ClaudeCode, inv.Ref, app.ClaimInput{
		Statement:           "Two new hires started a month earlier than the budget assumed.",
		Confidence:          app.ConfidenceHigh,
		ConfidenceRationale: "Their pay for March is $18,000, the same as the amount over budget.",
		Links:               []app.EvidenceLinkInput{supports(hires.Ref)},
	})
	if !s.check("INV-0001 C-001", err) {
		return
	}
	const answer = "Two new hires started a month earlier than the budget assumed."
	_, err = a.UpdateConclusion(ctx, ClaudeCode, inv.Ref, app.ConclusionInput{
		Conclusion: answer,
		Confidence: app.ConfidenceMedium,
	})
	if !s.check("INV-0001 answer", err) {
		return
	}

	s.at(8 * day)
	for _, ref := range []string{"E-001", "E-002", "E-003"} {
		_, err = a.SetEvidenceVerification(ctx, Person, inv.Ref, ref, app.VerificationHumanVerified)
		if !s.check("INV-0001 check "+ref, err) {
			return
		}
	}
	_, err = a.UpdateConclusion(ctx, Person, inv.Ref, app.ConclusionInput{
		Conclusion: answer,
		Confidence: app.ConfidenceHigh,
		Rationale:  "The two start dates explain the whole $18,000.",
	})
	if !s.check("INV-0001 answer by the person", err) {
		return
	}
	_, err = a.UpdateInvestigation(ctx, Person, inv.Ref, app.InvestigationPatch{Status: ptr(app.InvestigationStatusResolved)})
	s.check("INV-0001 resolve", err)
}

// callsMonday is Monday's Claude Code session on INV-0002: it opens the
// question, makes it current and saves E-001, E-002, H-001, H-002, C-001
// (the sales manager's view) and Q-001 in one checkpoint.
func (s *seeder) callsMonday() {
	a, ctx := s.a, s.ctx
	if s.err != nil {
		return
	}
	s.at(3*day + 2*hour)
	inv, err := a.CreateInvestigation(ctx, ClaudeCode, app.CreateInvestigationInput{
		Title:            FeaturedTitle,
		ProblemStatement: "Calls to the call center went from 2,400 in September to 3,120 in October. Sales stayed the same. Why?",
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
		Note: "Read the call log and the October sales report. Calls went from 2,400 to 3,120 while calls to buy and sales stayed about the same. " +
			"The sales manager thinks callers waited too long and hung up. " +
			"Two more ideas to test: a new ad and a price increase. " +
			"Still to find out: what the extra calls were about.",
		Evidence: []app.CheckpointEvidence{
			{Key: "buying", Evidence: app.EvidenceInput{
				Title:        "Calls to buy: 2,000 in September, 2,050 in October. Sales: $510,000 in September, $512,000 in October",
				Summary:      "The call log counts 2,000 calls to buy in September and 2,050 in October. The October sales report shows $510,000 of sales in September and $512,000 in October.",
				SourceType:   app.SourceTypeFile,
				SourceRef:    "Call log, September and October; October sales report",
				SourceLabel:  "Call log and October sales report",
				Verification: app.VerificationSourceVerified,
				Metadata:     `{"calls_to_buy_september":2000,"calls_to_buy_october":2050,"sales_september_usd":510000,"sales_october_usd":512000}`,
			}},
			{Key: "manager", Evidence: app.EvidenceInput{
				Title:       "Sales manager: we were two people short in October",
				Summary:     "The sales manager said in the Monday sales meeting that the call center was two people short in October. She thinks callers waited too long and hung up before buying.",
				SourceType:  app.SourceTypeUserStatement,
				SourceLabel: "Sales manager, in the Monday sales meeting",
			}},
		},
		Hypotheses: []app.HypothesisInput{
			{
				Statement:  "A new ad brought in more callers.",
				Confidence: app.ConfidenceMedium,
			},
			{
				Statement:  "We raised prices, so callers did not buy.",
				Confidence: app.ConfidenceLow,
			},
		},
		Claims: []app.ClaimInput{{
			Statement:           "Callers waited too long and hung up before buying.",
			Confidence:          app.ConfidenceLow,
			ConfidenceRationale: "This is the sales manager's view. It has not been compared with the phone system report.",
			Links:               []app.EvidenceLinkInput{supports("#manager")},
		}},
		Questions: []app.QuestionInput{{
			Question: "What were the extra calls about?",
		}},
	})
	s.check(FeaturedRef+" Monday checkpoint", err)
}

// callsTuesday is Tuesday's Codex session on INV-0002: the phone system
// report (E-003) contradicts C-001, the marketing calendar (E-004) and the
// price list (E-005) rule out H-001 and H-002, and the call log by reason
// (E-006) answers Q-001; C-002 says calls to buy stayed the same, H-003
// suspects billing and Q-002 asks what happened.
func (s *seeder) callsTuesday() {
	a, ctx := s.a, s.ctx
	if s.err != nil {
		return
	}
	s.at(2*day + 2*hour)
	wait, err := a.AddEvidence(ctx, Codex, FeaturedRef, app.EvidenceInput{
		Title:       "The average wait was 40 seconds in September and 40 seconds in October",
		Summary:     "The phone system report shows callers waited 40 seconds on average in September and 40 seconds in October. 3% of callers hung up while waiting in both months.",
		SourceType:  app.SourceTypeFile,
		SourceRef:   "Phone system report, September and October",
		SourceLabel: "Phone system report",
	})
	if !s.check(FeaturedRef+" E-003", err) {
		return
	}
	_, err = a.LinkEvidenceToClaim(ctx, Codex, FeaturedRef, "C-001", wait.Ref, app.RelationContradicts)
	if !s.check(FeaturedRef+" E-003 contradicts C-001", err) {
		return
	}
	// The source alone puts C-001 where sources disagree; an AI may say so too.
	_, err = a.SetClaimVerification(ctx, Codex, FeaturedRef, "C-001", app.VerificationDisputed)
	if !s.check(FeaturedRef+" sources disagree on C-001", err) {
		return
	}
	for _, r := range []struct {
		hyp string
		ev  app.EvidenceInput
	}{
		{"H-001", app.EvidenceInput{
			Title:       "No new ads ran in October",
			Summary:     "The marketing calendar lists the same two ads in September and October. Nothing new started in October.",
			SourceType:  app.SourceTypeFile,
			SourceRef:   "Marketing calendar, September and October",
			SourceLabel: "Marketing calendar",
		}},
		{"H-002", app.EvidenceInput{
			Title:       "Prices were the same in September and October",
			Summary:     "The price list has not changed since July 1.",
			SourceType:  app.SourceTypeFile,
			SourceRef:   "Price list, last changed July 1",
			SourceLabel: "Price list",
		}},
	} {
		ev, err := a.AddEvidence(ctx, Codex, FeaturedRef, r.ev)
		if !s.check(FeaturedRef+" source against "+r.hyp, err) {
			return
		}
		_, err = a.LinkEvidenceToHypothesis(ctx, Codex, FeaturedRef, r.hyp, ev.Ref, app.RelationContradicts)
		if !s.check(FeaturedRef+" "+ev.Ref+" contradicts "+r.hyp, err) {
			return
		}
	}
	s.at(2*day + hour)
	_, err = a.CheckpointBatch(ctx, Codex, FeaturedRef, app.CheckpointInput{
		Note: "Ruled out a new ad and a price increase. " +
			"The phone system report disagrees with the sales manager about waiting. " +
			"The call log by reason shows the extra calls were about a bill. " +
			"Calls to buy stayed about the same. Something may have gone wrong with billing in early October. Still to find out: what happened.",
		Evidence: []app.CheckpointEvidence{
			{Key: "reasons", Evidence: app.EvidenceInput{
				Title:       "Calls about a bill: 400 in September, 1,070 in October",
				Summary:     "The call log gives a reason for every call. Calls about a bill went from 400 in September to 1,070 in October. Calls to buy went from 2,000 to 2,050. The extra bill calls start on October 3.",
				SourceType:  app.SourceTypeFile,
				SourceRef:   "Call log, September and October, by reason",
				SourceLabel: "Call log",
				RawDetail: "reason,september,october\n" +
					"To buy,2000,2050\n" +
					"About a bill,400,1070\n" +
					"All calls,2400,3120",
				Metadata: `{"calls_about_a_bill_september":400,"calls_about_a_bill_october":1070,"all_calls_september":2400,"all_calls_october":3120}`,
			}},
		},
		Claims: []app.ClaimInput{{
			Statement:           "About the same number of people called to buy.",
			Confidence:          app.ConfidenceHigh,
			ConfidenceRationale: "The call log and the sales report agree.",
			Links:               []app.EvidenceLinkInput{supports("E-001"), supports("#reasons")},
		}},
		Hypotheses: []app.HypothesisInput{{
			Statement:  "Something went wrong with billing in early October.",
			Confidence: app.ConfidenceMedium,
			Links:      []app.EvidenceLinkInput{supports("#reasons")},
		}},
		HypothesisUpdates: []app.HypothesisUpdate{
			{Ref: "H-001", Patch: app.HypothesisPatch{Status: ptr(app.HypothesisStatusDisproven)}},
			{Ref: "H-002", Patch: app.HypothesisPatch{Status: ptr(app.HypothesisStatusDisproven)}},
		},
		QuestionUpdates: []app.QuestionUpdate{{
			Ref:        "Q-001",
			Resolution: ptr("They were about a bill: 400 calls in September, 1,070 in October."),
		}},
		Questions: []app.QuestionInput{{
			Question: "What happened with billing in early October?",
		}},
	})
	s.check(FeaturedRef+" Tuesday checkpoint", err)
}

// person is the person's part on Thursday afternoon: two things to find
// out (one left for later), a check of the billing team's notice (E-007),
// the billing team lead's word as E-009 linked to C-003
// and a check of the answer.
func (s *seeder) person() {
	a, ctx := s.a, s.ctx
	s.at(80 * time.Minute)
	_, err := a.AddQuestion(ctx, Person, FeaturedRef, app.QuestionInput{
		Question: "Have all 1,900 customers been paid back?",
	})
	if !s.check(FeaturedRef+" Q-003", err) {
		return
	}
	s.at(78 * time.Minute)
	q4, err := a.AddQuestion(ctx, Person, FeaturedRef, app.QuestionInput{
		Question: "Should we expect fewer sales in November from the customers who were charged twice?",
	})
	if !s.check(FeaturedRef+" Q-004", err) {
		return
	}
	_, err = a.DeferQuestion(ctx, Person, FeaturedRef, q4.Ref, "Waiting for the November sales report on December 1.")
	if !s.check(FeaturedRef+" defer Q-004", err) {
		return
	}
	s.at(76 * time.Minute)
	_, err = a.SetEvidenceVerification(ctx, Person, FeaturedRef, "E-007", app.VerificationHumanVerified)
	if !s.check(FeaturedRef+" check E-007", err) {
		return
	}
	s.at(75 * time.Minute)
	e9, err := a.AddEvidence(ctx, Person, FeaturedRef, app.EvidenceInput{
		Title:       "Billing team lead: the mistake was fixed on October 5",
		Summary:     "The billing team lead said in person that the bills sent on October 3 charged 1,900 customers twice, and that the mistake was fixed on October 5.",
		SourceType:  app.SourceTypeUserStatement,
		SourceLabel: "Billing team lead, in person",
	})
	if !s.check(FeaturedRef+" E-009", err) {
		return
	}
	_, err = a.LinkEvidenceToClaim(ctx, Person, FeaturedRef, "C-003", e9.Ref, app.RelationSupports)
	if !s.check(FeaturedRef+" E-009 supports C-003", err) {
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
	if !s.check(FeaturedRef+" answer by the person", err) {
		return
	}
	_, err = a.UpdateConclusion(ctx, Person, FeaturedRef, app.ConclusionInput{
		Conclusion:          Conclusion,
		Confidence:          app.ConfidenceHigh,
		Rationale:           PersonRationale,
		Verification:        app.VerificationHumanVerified,
		Review:              true,
		ReviewBasis:         "Read the billing team's notice and the call log by reason, and spoke to the billing team lead. Sources disagree about waiting time. Whether those customers buy less later is not tested yet.",
		ExpectedReviewToken: conclusion.ConclusionReviewToken,
	})
	s.check(FeaturedRef+" answer reviewed", err)
}

// helpDesk opens INV-0003 on Monday afternoon.
func (s *seeder) helpDesk() {
	a, ctx := s.a, s.ctx
	if s.err != nil {
		return
	}
	s.at(3*day - 2*hour)
	inv, err := a.CreateInvestigation(ctx, CLI, app.CreateInvestigationInput{
		Title:            "Why are help desk tickets taking longer to close this month?",
		ProblemStatement: "Help desk tickets took 2 days to close in September. In October they take 5 days. What changed?",
		Status:           app.InvestigationStatusInvestigating,
	})
	if !s.check("INV-0003", err) {
		return
	}
	s.at(3*day - 4*hour)
	e1, err := a.AddEvidence(ctx, Codex, inv.Ref, app.EvidenceInput{
		Title:       "Tickets took 2 days to close in September and 5 days in October",
		Summary:     "The help desk report shows tickets took 2 days to close on average in September and 5 days in October. 900 tickets came in during September and 1,400 in October.",
		SourceType:  app.SourceTypeFile,
		SourceRef:   "Help desk report, September and October",
		SourceLabel: "Help desk report",
	})
	if !s.check("INV-0003 E-001", err) {
		return
	}
	e2, err := a.AddEvidence(ctx, Codex, inv.Ref, app.EvidenceInput{
		Title:       "Password reset tickets: 120 in September, 610 in October",
		Summary:     "The help desk report by ticket type shows 120 password reset tickets in September and 610 in October. Every other type stayed about the same.",
		SourceType:  app.SourceTypeFile,
		SourceRef:   "Help desk report, by ticket type",
		SourceLabel: "Help desk report",
	})
	if !s.check("INV-0003 E-002", err) {
		return
	}
	_, err = a.AddClaim(ctx, Codex, inv.Ref, app.ClaimInput{
		Statement:           "Almost all of the 500 extra tickets in October are password resets.",
		Confidence:          app.ConfidenceMedium,
		ConfidenceRationale: "The two help desk reports agree. Nobody has said why resets went up.",
		Links:               []app.EvidenceLinkInput{supports(e1.Ref), supports(e2.Ref)},
	})
	if !s.check("INV-0003 C-001", err) {
		return
	}
	_, err = a.AddHypothesis(ctx, Codex, inv.Ref, app.HypothesisInput{
		Statement:  "The new password rules caused the extra resets.",
		Confidence: app.ConfidenceLow,
	})
	s.check("INV-0003 H-001", err)
}

// helpDeskStuck leaves INV-0003 without an answer since yesterday: two
// things the AI could not find out, and both sources checked by the person.
func (s *seeder) helpDeskStuck() {
	a, ctx := s.a, s.ctx
	if s.err != nil {
		return
	}
	s.at(30 * hour)
	for _, q := range []string{
		"When did the new password rules start?",
		"Did the help desk have fewer people in October?",
	} {
		_, err := a.AddQuestion(ctx, Codex, "INV-0003", app.QuestionInput{Question: q})
		if !s.check("INV-0003 thing to find out", err) {
			return
		}
	}
	// The person opened both sources. Every source is checked and the
	// answer is still to write.
	s.at(29 * hour)
	for _, ref := range []string{"E-001", "E-002"} {
		_, err := a.SetEvidenceVerification(ctx, Person, "INV-0003", ref, app.VerificationHumanVerified)
		if !s.check("INV-0003 check "+ref, err) {
			return
		}
	}
	_, err := a.UpdateInvestigation(ctx, CLI, "INV-0003", app.InvestigationPatch{Status: ptr(app.InvestigationStatusBlocked)})
	s.check("INV-0003 blocked", err)
}

// lostCustomers is INV-0004, asked and answered within a few hours on
// Tuesday. Nobody has checked its sources.
func (s *seeder) lostCustomers() {
	a, ctx := s.a, s.ctx
	if s.err != nil {
		return
	}
	s.at(2*day - 2*hour)
	inv, err := a.CreateInvestigation(ctx, Person, app.CreateInvestigationInput{
		Title:            "Which customers stopped ordering after the price change?",
		ProblemStatement: "Prices went up on July 1. Which customers have not ordered since, and how much did they buy before?",
		Status:           app.InvestigationStatusInvestigating,
	})
	if !s.check("INV-0004", err) {
		return
	}
	s.at(2*day - 3*hour)
	var refs []string
	for _, e := range []app.EvidenceInput{
		{
			Title:       "Eleven customers ordered in April, May or June and have not ordered since July 1",
			Summary:     "The order list shows eleven customers with an order in April, May or June and no order from July 1 to October 31.",
			SourceType:  app.SourceTypeFile,
			SourceRef:   "Order list, April to October",
			SourceLabel: "Order list",
		},
		{
			Title:       "Those eleven customers were 2% of sales from January to June",
			Summary:     "The sales report by customer shows the eleven customers bought $61,000 of the $3,050,000 sold from January to June.",
			SourceType:  app.SourceTypeFile,
			SourceRef:   "Sales report by customer, January to June",
			SourceLabel: "Sales report",
		},
		{
			Title:       "Nine of the eleven ordered less than once a month before the change",
			Summary:     "The order list shows nine of the eleven customers placed five orders or fewer from January to June.",
			SourceType:  app.SourceTypeFile,
			SourceRef:   "Order list, January to June",
			SourceLabel: "Order list",
		},
	} {
		ev, err := a.AddEvidence(ctx, Codex, inv.Ref, e)
		if !s.check("INV-0004 source", err) {
			return
		}
		refs = append(refs, ev.Ref)
	}
	_, err = a.AddClaim(ctx, Codex, inv.Ref, app.ClaimInput{
		Statement:           "Eleven small customers stopped ordering after the price change. Together they were 2% of sales.",
		Confidence:          app.ConfidenceHigh,
		ConfidenceRationale: "The order list and the sales report agree.",
		Links:               []app.EvidenceLinkInput{supports(refs[0]), supports(refs[1]), supports(refs[2])},
	})
	if !s.check("INV-0004 C-001", err) {
		return
	}
	s.at(2*day - 6*hour)
	_, err = a.UpdateConclusion(ctx, Person, inv.Ref, app.ConclusionInput{
		Conclusion: "Eleven small customers, together 2% of sales.",
		Confidence: app.ConfidenceHigh,
		Rationale:  "The order list names the eleven customers and the sales report gives what they bought.",
	})
	if !s.check("INV-0004 answer", err) {
		return
	}
	_, err = a.UpdateInvestigation(ctx, Person, inv.Ref, app.InvestigationPatch{Status: ptr(app.InvestigationStatusResolved)})
	s.check("INV-0004 resolve", err)
}

// suppliesOrder is INV-0005, answered and archived yesterday; nothing is
// deleted.
func (s *seeder) suppliesOrder() {
	a, ctx := s.a, s.ctx
	if s.err != nil {
		return
	}
	s.at(28 * hour)
	inv, err := a.CreateInvestigation(ctx, CLI, app.CreateInvestigationInput{
		Title:            "Why did the office supplies order double in August?",
		ProblemStatement: "We usually spend $2,100 a month on office supplies. The August order was $4,200. Why?",
		Status:           app.InvestigationStatusInvestigating,
	})
	if !s.check("INV-0005", err) {
		return
	}
	ev, err := a.AddEvidence(ctx, ClaudeCode, inv.Ref, app.EvidenceInput{
		Title:       "Office supplies orders: none in July, $4,200 in August",
		Summary:     "The list of orders shows no office supplies order in July and one order of $4,200 in August. Every other month this year has one order of about $2,100.",
		SourceType:  app.SourceTypeFile,
		SourceRef:   "List of orders, this year",
		SourceLabel: "List of orders",
	})
	if !s.check("INV-0005 E-001", err) {
		return
	}
	_, err = a.AddClaim(ctx, ClaudeCode, inv.Ref, app.ClaimInput{
		Statement:  "The August order covered July and August.",
		Confidence: app.ConfidenceHigh,
		Links:      []app.EvidenceLinkInput{supports(ev.Ref)},
	})
	if !s.check("INV-0005 C-001", err) {
		return
	}
	s.at(27 * hour)
	_, err = a.UpdateConclusion(ctx, Person, inv.Ref, app.ConclusionInput{
		Conclusion: "Nobody placed the July order, so the August order covered two months.",
		Confidence: app.ConfidenceHigh,
		Rationale:  "The list of orders shows both months.",
	})
	if !s.check("INV-0005 answer", err) {
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
