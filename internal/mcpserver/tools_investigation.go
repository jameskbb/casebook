package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
)

// Read tools render their data as compact text content as well as
// structuredContent: some clients (Claude Code) show the model only the
// content, others (Codex) only structuredContent.

func init() {
	registerTool("casebook_get_active_investigation", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_get_active_investigation",
			Title:       "Get the active investigation",
			Description: getActiveDescription,
			Annotations: readOnlyTool(),
		}, h.getActiveInvestigation)
	})
	registerTool("casebook_get_investigation", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_get_investigation",
			Title:       "Get an investigation",
			Description: getInvestigationDescription,
			Annotations: readOnlyTool(),
		}, h.getInvestigation)
	})
	registerTool("casebook_create_investigation", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_create_investigation",
			Title:       "Create an investigation",
			Description: createInvestigationDescription,
			Annotations: mutatingTool(false),
		}, h.createInvestigation)
	})
	registerTool("casebook_set_active_investigation", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_set_active_investigation",
			Title:       "Set the active investigation",
			Description: setActiveDescription,
			Annotations: mutatingTool(true),
		}, h.setActiveInvestigation)
	})
	registerTool("casebook_list_investigations", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_list_investigations",
			Title:       "List investigations",
			Description: listInvestigationsDescription,
			Annotations: readOnlyTool(),
		}, h.listInvestigations)
	})
	registerTool("casebook_search_investigations", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_search_investigations",
			Title:       "Search investigations",
			Description: searchInvestigationsDescription,
			Annotations: readOnlyTool(),
		}, h.searchInvestigations)
	})
	registerTool("casebook_update_investigation", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_update_investigation",
			Title:       "Update an investigation",
			Description: updateInvestigationDescription,
			Annotations: mutatingTool(true),
		}, h.updateInvestigation)
	})
	registerTool("casebook_update_conclusion", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_update_conclusion",
			Title:       "Update the conclusion",
			Description: updateConclusionDescription,
			Annotations: mutatingTool(true),
		}, h.updateConclusion)
	})
}

// countsOf computes record counts from a detail.
func countsOf(d app.InvestigationDetail) app.RecordCounts {
	c := app.RecordCounts{
		Evidence:   len(d.Evidence),
		Claims:     len(d.Claims),
		Hypotheses: len(d.Hypotheses),
		Questions:  len(d.Questions),
	}
	for _, q := range d.Questions {
		if q.Status == app.QuestionStatusOpen {
			c.OpenQuestions++
		}
	}
	return c
}

// casebook_get_active_investigation

const getActiveDescription = `Report which case (an investigation record) is active: the shared default that every tool uses when investigation_ref is omitted or empty, set from any Casebook client. ` +
	`Call it at the start of a session to see what you are working on. ` +
	`A project directory can have its own active case, which wins over the global one for servers started there; active_dir then names that directory. ` +
	`When none is set it returns active false with a message (not an error); pick one with casebook_list_investigations and casebook_set_active_investigation, or pass investigation_ref to other tools. ` +
	`Otherwise it returns the header (ref, title, problem statement, status, conclusion and its confidence) and record counts; long texts are cut only to stay under 64 KiB, and truncated_fields names them. ` +
	`For the full working context call casebook_get_context.`

const noActiveMessage = "No active investigation is set. Call casebook_list_investigations, then casebook_set_active_investigation, or pass investigation_ref to other tools."

type getActiveInput struct{}

// activeInvestigation is the header of the active investigation.
type activeInvestigation struct {
	conclusionAccountability
	Ref                  string                  `json:"ref"`
	Title                string                  `json:"title"`
	ProblemStatement     string                  `json:"problem_statement"`
	Status               app.InvestigationStatus `json:"status"`
	Conclusion           string                  `json:"conclusion"`
	ConclusionConfidence app.Confidence          `json:"conclusion_confidence"`
	UpdatedAt            string                  `json:"updated_at"`
	Counts               app.RecordCounts        `json:"counts"`
	TruncatedFields      []string                `json:"truncated_fields,omitempty" jsonschema:"header fields cut to fit the 64 KiB response budget: problem_statement, conclusion"`
}

type activeOutput struct {
	Active        bool                 `json:"active"`
	Message       string               `json:"message,omitempty"`
	Investigation *activeInvestigation `json:"investigation,omitempty"`
	ActiveDir     string               `json:"active_dir,omitempty" jsonschema:"the project directory this investigation is active for, when it comes from a per-directory setting rather than the global one"`
}

func (h *handlers) getActiveInvestigation(ctx context.Context, _ call, _ getActiveInput) (activeOutput, string, error) {
	d, err := h.app.GetInvestigationDetail(ctx, "", app.DetailOptions{ActivityLimit: 1})
	if errors.Is(err, app.ErrNoActiveInvestigation) {
		return activeOutput{Active: false, Message: noActiveMessage}, noActiveMessage, nil
	}
	if err != nil {
		return activeOutput{}, "", err
	}
	inv := d.Investigation
	info, err := h.app.ActiveInvestigationInfo(ctx)
	if err != nil {
		return activeOutput{}, "", err
	}
	for _, n := range headerCuts {
		hd := cutHeader(inv, n)
		out := activeOutput{Active: true, ActiveDir: info.Dir, Investigation: &activeInvestigation{
			conclusionAccountability: conclusionAccountabilityOf(inv),
			Ref:                      inv.Ref,
			Title:                    inv.Title,
			ProblemStatement:         hd.problem,
			Status:                   inv.Status,
			Conclusion:               hd.conclusion,
			ConclusionConfidence:     inv.ConclusionConfidence,
			UpdatedAt:                inv.UpdatedAt,
			Counts:                   countsOf(d),
		}}
		// The rationale is not part of this view, so it is never reported.
		for _, f := range hd.truncated {
			if f != "conclusion_rationale" {
				out.Investigation.TruncatedFields = append(out.Investigation.TruncatedFields, f)
			}
		}
		text := renderActive(*out.Investigation)
		if out.ActiveDir != "" {
			text += "\nActive for the project directory " + out.ActiveDir + "."
		}
		if resultBytes(text, out) <= responseTarget {
			return out, text, nil
		}
	}
	// Unreachable within the app limits (see headerCuts).
	return activeOutput{}, "", fmt.Errorf("casebook: investigation %s is too large to return within the 64 KiB response budget", inv.Ref)
}

func renderActive(a activeInvestigation) string {
	c := a.Counts
	var b strings.Builder
	fmt.Fprintf(&b, "Active investigation %s %q (%s): %d evidence, %d claims, %d hypotheses, %d open questions.",
		a.Ref, a.Title, a.Status, c.Evidence, c.Claims, c.Hypotheses, c.OpenQuestions)
	if a.ProblemStatement != "" {
		fmt.Fprintf(&b, "\nProblem: %s", app.TruncateLine(a.ProblemStatement, 0))
	}
	if a.Conclusion != "" {
		fmt.Fprintf(&b, "\nConclusion (%s, %s): %s", a.ConclusionConfidence, a.ConclusionVerification, app.TruncateLine(a.Conclusion, 0))
		fmt.Fprintf(&b, "\nConclusion needs review: %t", a.ConclusionNeedsReview)
	}
	if len(a.TruncatedFields) > 0 {
		fmt.Fprintf(&b, "\n(cut to fit the 64 KiB response budget: %s)", strings.Join(a.TruncatedFields, ", "))
	}
	return b.String()
}

// headerCuts are the byte bounds for the long header texts (problem
// statement, conclusion, conclusion rationale) that the investigation read
// tools try in order, 0 meaning whole (D-16). The texts are cut only when the
// lists alone cannot bring a result under the budget. At the last bound the
// header is a few KiB even when every character needs a six-byte JSON escape.
var headerCuts = []int{0, 1000, textLineBytes}

// investigationHeader holds the long header texts of an investigation, cut
// to a headerCuts bound, and the names of the fields that were cut.
type investigationHeader struct {
	problem, conclusion, rationale string
	truncated                      []string
}

func cutHeader(inv app.Investigation, n int) investigationHeader {
	var h investigationHeader
	var cut [3]bool
	h.problem, cut[0] = cutText(inv.ProblemStatement, n)
	h.conclusion, cut[1] = cutText(inv.Conclusion, n)
	h.rationale, cut[2] = cutText(inv.ConclusionRationale, n)
	for i, name := range []string{"problem_statement", "conclusion", "conclusion_rationale"} {
		if cut[i] {
			h.truncated = append(h.truncated, name)
		}
	}
	return h
}

// casebook_get_investigation

const getInvestigationDescription = `Read one case (an investigation record) in compact form: header fields, conclusion, record counts, claims and hypotheses (status, confidence, verification, supporting and contradicting evidence refs, one-line text), questions, an evidence index (ref, title, source, verification, link counts) and up to 10 recent activity rows. ` +
	`Use it to review records before updating them; for a resume briefing use casebook_get_context. ` +
	`` +
	`sections picks lists (claims, hypotheses, questions, evidence, activity; empty means all). limit caps each list (default 50, max 200); cut lists say truncated with a total. ` +
	`List texts are cut to 240 bytes (full evidence: casebook_get_evidence). To stay under 64 KiB lists shrink, then header texts are cut (see truncated_fields).`

type getInvestigationInput struct {
	InvestigationRef string                 `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	Sections         []investigationSection `json:"sections,omitempty" jsonschema:"lists to include; empty means all"`
	Limit            int                    `json:"limit,omitempty" jsonschema:"maximum items per list (default 50, max 200)"`
}

// investigationView is the casebook_get_investigation output. A list is
// omitted when its section was not requested.
type investigationView struct {
	conclusionAccountability
	Ref                  string                     `json:"ref"`
	Title                string                     `json:"title"`
	ProblemStatement     string                     `json:"problem_statement"`
	Status               app.InvestigationStatus    `json:"status"`
	IsActive             bool                       `json:"is_active"`
	Archived             bool                       `json:"archived"`
	Conclusion           string                     `json:"conclusion"`
	ConclusionConfidence app.Confidence             `json:"conclusion_confidence"`
	ConclusionRationale  string                     `json:"conclusion_rationale"`
	CreatedAt            string                     `json:"created_at"`
	UpdatedAt            string                     `json:"updated_at"`
	Counts               app.RecordCounts           `json:"counts"`
	Claims               *itemList[statementRecord] `json:"claims,omitempty"`
	Hypotheses           *itemList[statementRecord] `json:"hypotheses,omitempty"`
	Questions            *itemList[questionRecord]  `json:"questions,omitempty"`
	Evidence             *itemList[evidenceRecord]  `json:"evidence,omitempty"`
	RecentActivity       *[]activityRecord          `json:"recent_activity,omitempty"`
	Truncated            bool                       `json:"truncated"`
	TruncatedFields      []string                   `json:"truncated_fields,omitempty" jsonschema:"header fields cut to fit the 64 KiB response budget: problem_statement, conclusion, conclusion_rationale"`
}

func (h *handlers) getInvestigation(ctx context.Context, _ call, in getInvestigationInput) (investigationView, string, error) {
	limit, err := limitParam("limit", in.Limit, defaultDetailLimit, maxDetailLimit)
	if err != nil {
		return investigationView{}, "", err
	}
	d, err := h.app.GetInvestigationDetail(ctx, in.InvestigationRef, app.DetailOptions{ActivityLimit: maxRecentActivity})
	if err != nil {
		return investigationView{}, "", err
	}
	sections := in.Sections
	if len(sections) == 0 {
		sections = allInvestigationSections()
	}
	// Lists halve first; only when one item per list still does not fit are
	// the long header texts cut, and the lists start over at the limit.
	for _, n := range headerCuts {
		lim := limit
		for {
			v := buildInvestigationView(d, sections, lim, n)
			text := renderInvestigationView(v)
			if resultBytes(text, v) <= responseTarget {
				return v, text, nil
			}
			if lim <= 1 {
				break
			}
			lim /= 2
		}
	}
	return investigationView{}, "", fmt.Errorf("casebook: investigation %s is too large to return within the 64 KiB response budget; pass sections to read fewer lists",
		d.Investigation.Ref)
}

func buildInvestigationView(d app.InvestigationDetail, sections []investigationSection, limit, headerCut int) investigationView {
	inv := d.Investigation
	hd := cutHeader(inv, headerCut)
	v := investigationView{
		conclusionAccountability: conclusionAccountabilityOf(inv),
		Ref:                      inv.Ref,
		Title:                    inv.Title,
		ProblemStatement:         hd.problem,
		Status:                   inv.Status,
		IsActive:                 inv.IsActive,
		Archived:                 inv.Archived,
		Conclusion:               hd.conclusion,
		ConclusionConfidence:     inv.ConclusionConfidence,
		ConclusionRationale:      hd.rationale,
		CreatedAt:                inv.CreatedAt,
		UpdatedAt:                inv.UpdatedAt,
		Counts:                   countsOf(d),
		Truncated:                len(hd.truncated) > 0,
		TruncatedFields:          hd.truncated,
	}
	want := func(s investigationSection) bool { return slices.Contains(sections, s) }
	if want(sectionClaims) {
		recs := make([]statementRecord, 0, len(d.Claims))
		for _, c := range d.Claims {
			recs = append(recs, claimRecordOf(c, true))
		}
		l := newItemList(recs, limit)
		v.Claims = &l
		v.Truncated = v.Truncated || l.Truncated
	}
	if want(sectionHypotheses) {
		recs := make([]statementRecord, 0, len(d.Hypotheses))
		for _, hy := range d.Hypotheses {
			recs = append(recs, hypothesisRecordOf(hy, true))
		}
		l := newItemList(recs, limit)
		v.Hypotheses = &l
		v.Truncated = v.Truncated || l.Truncated
	}
	if want(sectionQuestions) {
		recs := make([]questionRecord, 0, len(d.Questions))
		for _, q := range d.Questions {
			recs = append(recs, questionRecordOf(q, true))
		}
		l := newItemList(recs, limit)
		v.Questions = &l
		v.Truncated = v.Truncated || l.Truncated
	}
	if want(sectionEvidence) {
		recs := make([]evidenceRecord, 0, len(d.Evidence))
		for _, e := range d.Evidence {
			recs = append(recs, evidenceRecordOf(e))
		}
		l := newItemList(recs, limit)
		v.Evidence = &l
		v.Truncated = v.Truncated || l.Truncated
	}
	if want(sectionActivity) {
		n := min(len(d.RecentActivity), maxRecentActivity, limit)
		rows := make([]activityRecord, 0, n)
		for _, a := range d.RecentActivity[:n] {
			rows = append(rows, activityRecordOf(a))
		}
		v.RecentActivity = &rows
	}
	return v
}

// importedMark is " (imported)" for a row copied from an imported file, so
// its verification is not read as this database's own (AR-09).
func importedMark(imported bool) string {
	if imported {
		return " (imported)"
	}
	return ""
}

// statementLine renders one claim or hypothesis.
func statementLine(r statementRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s, %s, %s%s] %s", r.Ref, r.Status, r.Confidence, r.Verification, importedMark(r.Imported), r.Text)
	var notes []string
	if len(r.Supports) > 0 {
		notes = append(notes, "supports "+strings.Join(r.Supports, ", "))
	}
	if len(r.Contradicts) > 0 {
		notes = append(notes, "contradicts "+strings.Join(r.Contradicts, ", "))
	}
	if r.LinksOmitted > 0 {
		notes = append(notes, fmt.Sprintf("%d more links omitted", r.LinksOmitted))
	}
	if r.SupersededBy != "" {
		notes = append(notes, "superseded by "+r.SupersededBy)
	}
	if len(notes) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(notes, "; "))
	}
	return b.String()
}

// renderInvestigationView is the compact plain-text form of a view.
func renderInvestigationView(v investigationView) string {
	var b strings.Builder
	line := func(format string, args ...any) {
		fmt.Fprintf(&b, format, args...)
		b.WriteByte('\n')
	}
	active := ""
	if v.IsActive {
		active = ", active"
	}
	line("%s %q (%s%s)", v.Ref, v.Title, v.Status, active)
	if v.ProblemStatement != "" {
		line("Problem: %s", app.TruncateLine(v.ProblemStatement, 0))
	}
	if v.Conclusion != "" {
		line("Conclusion (%s, %s): %s", v.ConclusionConfidence, v.ConclusionVerification, app.TruncateLine(v.Conclusion, 0))
		line("Conclusion review: reviewed %s; needs review %t; basis %s", v.ConclusionReviewedAt, v.ConclusionNeedsReview, app.TruncateLine(v.ConclusionReviewBasis, 0))
		line("Conclusion claims: supports %s; contradicts %s", strings.Join(v.ConclusionSupportingClaims, ", "), strings.Join(v.ConclusionContradictingClaims, ", "))
		if v.ConclusionRationale != "" {
			line("Rationale: %s", app.TruncateLine(v.ConclusionRationale, 0))
		}
	} else {
		line("Conclusion: none yet")
	}
	c := v.Counts
	line("Counts: %d evidence, %d claims, %d hypotheses, %d questions (%d open)",
		c.Evidence, c.Claims, c.Hypotheses, c.Questions, c.OpenQuestions)

	var truncated []string
	note := func(n, total int, what string) {
		truncated = append(truncated, fmt.Sprintf("(truncated: %d of %d %s shown)", n, total, what))
	}
	if v.Claims != nil {
		line("\nClaims (%d):", v.Claims.Total)
		for _, r := range v.Claims.Items {
			line("%s", statementLine(r))
		}
		if v.Claims.Truncated {
			note(len(v.Claims.Items), v.Claims.Total, "claims")
		}
	}
	if v.Hypotheses != nil {
		line("\nHypotheses (%d):", v.Hypotheses.Total)
		for _, r := range v.Hypotheses.Items {
			line("%s", statementLine(r))
		}
		if v.Hypotheses.Truncated {
			note(len(v.Hypotheses.Items), v.Hypotheses.Total, "hypotheses")
		}
	}
	if v.Questions != nil {
		line("\nQuestions (%d):", v.Questions.Total)
		for _, q := range v.Questions.Items {
			s := fmt.Sprintf("%s [%s] %s", q.Ref, q.Status, q.Text)
			if q.Resolution != "" {
				s += " (resolution: " + q.Resolution + ")"
			}
			if q.DeferNote != "" {
				s += " (deferred: " + q.DeferNote + ")"
			}
			line("%s", s)
		}
		if v.Questions.Truncated {
			note(len(v.Questions.Items), v.Questions.Total, "questions")
		}
	}
	if v.Evidence != nil {
		line("\nEvidence (%d):", v.Evidence.Total)
		for _, e := range v.Evidence.Items {
			s := fmt.Sprintf("%s [%s, %s%s] %s", e.Ref, e.SourceType, e.Verification, importedMark(e.Imported), e.Title)
			if e.SourceLabel != "" {
				s += " (" + e.SourceLabel + ")"
			}
			line("%s", s)
		}
		if v.Evidence.Truncated {
			note(len(v.Evidence.Items), v.Evidence.Total, "evidence")
		}
	}
	if v.RecentActivity != nil {
		line("\nRecent activity:")
		for _, a := range *v.RecentActivity {
			line("%s %s %s by %s/%s: %s", a.CreatedAt, a.Action, a.TargetRef, a.Actor.Kind, a.Actor.Client, a.Summary)
		}
	}
	if len(v.TruncatedFields) > 0 {
		truncated = append(truncated, fmt.Sprintf("(cut to fit the 64 KiB response budget: %s)", strings.Join(v.TruncatedFields, ", ")))
	}
	if len(truncated) > 0 {
		b.WriteByte('\n')
		for _, t := range truncated {
			line("%s", t)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// casebook_create_investigation

const createInvestigationDescription = `Start a new case (an investigation record): one problem or line of inquiry that evidence, claims, hypotheses, questions and a conclusion will hang off. ` +
	`Use it when the person asks for a case, or agrees when you offer one because a task turned into an investigation (do not open one unasked). Search first (casebook_search_investigations) so you continue an existing case instead of opening a duplicate. ` +
	`title is a short one-line name (at most 200 bytes); problem_statement is the primary question. ` +
	`status defaults to new and cannot be archived. ` +
	`make_active true also makes it the active case, shared by the desktop app, the CLI and every MCP client, so later calls that omit investigation_ref write here. ` +
	`Returns the new ref (INV-0001 style) and a compact record.`

type createInvestigationInput struct {
	Title            string                  `json:"title" jsonschema:"short one-line name of the investigation (at most 200 bytes)"`
	ProblemStatement string                  `json:"problem_statement,omitempty" jsonschema:"the primary question the investigation answers"`
	Status           app.InvestigationStatus `json:"status,omitempty" jsonschema:"initial status; defaults to new; archived is not allowed"`
	MakeActive       bool                    `json:"make_active,omitempty" jsonschema:"also make it the active investigation used when investigation_ref is omitted"`
}

func (h *handlers) createInvestigation(ctx context.Context, c call, in createInvestigationInput) (mutationResult[investigationRecord], string, error) {
	inv, err := h.app.CreateInvestigation(ctx, c.Actor, app.CreateInvestigationInput{
		Title:            in.Title,
		ProblemStatement: in.ProblemStatement,
		Status:           in.Status,
		MakeActive:       in.MakeActive,
	})
	if err != nil {
		return mutationResult[investigationRecord]{}, "", err
	}
	out := mutationResult[investigationRecord]{
		Ref:              inv.Ref,
		InvestigationRef: inv.Ref,
		Action:           "created",
		Changed:          []string{},
		Record:           investigationRecordOf(inv),
	}
	text := fmt.Sprintf("Created investigation %s %q (status %s).", inv.Ref, inv.Title, inv.Status)
	if inv.IsActive {
		text += " It is now the active investigation."
	}
	return out, text, nil
}

// casebook_set_active_investigation

const setActiveDescription = `Make a case (an investigation record) the active one. The active case is shared by the desktop app, the CLI and every MCP client, and every tool that omits investigation_ref (or passes it empty) uses it, so switch here before working on another case. ` +
	`When this server's project directory has its own active case, that per-directory setting changes instead of the global one. ` +
	`investigation_ref is required. ` +
	`Find refs with casebook_list_investigations or casebook_search_investigations. ` +
	`Returns action activated, or unchanged when it already was active, with a compact record.`

type setActiveInput struct {
	InvestigationRef string `json:"investigation_ref" jsonschema:"investigation ref such as INV-0001 to make active"`
}

func (h *handlers) setActiveInvestigation(ctx context.Context, c call, in setActiveInput) (mutationResult[investigationRecord], string, error) {
	var zero mutationResult[investigationRecord]
	prev, err := h.app.GetActiveInvestigation(ctx)
	if err != nil {
		return zero, "", err
	}
	inv, err := h.app.SetActiveInvestigation(ctx, c.Actor, in.InvestigationRef)
	if err != nil {
		return zero, "", err
	}
	out := mutationResult[investigationRecord]{
		Ref:              inv.Ref,
		InvestigationRef: inv.Ref,
		Action:           "activated",
		Changed:          []string{"is_active"},
		Record:           investigationRecordOf(inv),
	}
	if prev != nil && prev.Ref == inv.Ref {
		out.Action = "unchanged"
		out.Changed = []string{}
	}
	return out, fmt.Sprintf("Active investigation is now %s %q.", inv.Ref, inv.Title), nil
}

// casebook_list_investigations

const listInvestigationsDescription = `List cases (investigation records) in compact form, newest updated first: ref (INV-0001 style), title, status, conclusion confidence, updated time, whether it is the active one, and record counts. ` +
	`Use it to find a case to continue or to pick one for casebook_set_active_investigation. ` +
	`Archived cases are hidden unless include_archived is true or status is archived; status filters to one status. ` +
	`limit defaults to 20 (max 100); a cut list says truncated with the total. Beyond 1000 matches total is 1000 and total_is_lower_bound is true. active_ref names the active case ("" when none). ` +
	`For one case's records call casebook_get_investigation; to find text inside records call casebook_search_investigations.`

// listScanLimit is how many summaries the list tool reads (the app maximum),
// so total is exact below that many investigations and a lower bound at it.
const listScanLimit = 1000

type listInvestigationsInput struct {
	IncludeArchived bool                    `json:"include_archived,omitempty" jsonschema:"also list archived investigations"`
	Status          app.InvestigationStatus `json:"status,omitempty" jsonschema:"only investigations with this status; omit for any"`
	Limit           int                     `json:"limit,omitempty" jsonschema:"maximum items (default 20, max 100)"`
}

// investigationSummaryOut is one compact list item.
type investigationSummaryOut struct {
	Ref                  string                  `json:"ref"`
	Title                string                  `json:"title"`
	Status               app.InvestigationStatus `json:"status"`
	ConclusionConfidence app.Confidence          `json:"conclusion_confidence"`
	UpdatedAt            string                  `json:"updated_at"`
	IsActive             bool                    `json:"is_active"`
	Archived             bool                    `json:"archived"`
	Counts               app.RecordCounts        `json:"counts"`
}

type listInvestigationsOut struct {
	Investigations []investigationSummaryOut `json:"investigations"`
	Total          int                       `json:"total"`
	// TotalIsLowerBound is true when the scan reached listScanLimit: there
	// may be more investigations than Total.
	TotalIsLowerBound bool   `json:"total_is_lower_bound" jsonschema:"true when total is a lower bound: there may be more matches than total"`
	Truncated         bool   `json:"truncated"`
	ActiveRef         string `json:"active_ref"`
}

func (h *handlers) listInvestigations(ctx context.Context, _ call, in listInvestigationsInput) (listInvestigationsOut, string, error) {
	limit, err := limitParam("limit", in.Limit, defaultListLimit, maxListLimit)
	if err != nil {
		return listInvestigationsOut{}, "", err
	}
	sums, err := h.app.ListInvestigations(ctx, app.ListOptions{
		IncludeArchived: in.IncludeArchived,
		Status:          string(in.Status),
		Limit:           listScanLimit,
	})
	if err != nil {
		return listInvestigationsOut{}, "", err
	}
	active, err := h.app.GetActiveInvestigation(ctx)
	if err != nil {
		return listInvestigationsOut{}, "", err
	}
	activeRef := ""
	if active != nil {
		activeRef = active.Ref
	}
	for {
		out := buildInvestigationList(sums, limit, activeRef)
		text := renderInvestigationList(out, in.IncludeArchived || in.Status != "")
		if resultBytes(text, out) <= responseTarget || limit <= 1 {
			return out, text, nil
		}
		limit /= 2
	}
}

func buildInvestigationList(sums []app.InvestigationSummary, limit int, activeRef string) listInvestigationsOut {
	n := min(len(sums), limit)
	items := make([]investigationSummaryOut, 0, n)
	for _, s := range sums[:n] {
		items = append(items, investigationSummaryOut{
			Ref:                  s.Ref,
			Title:                s.Title,
			Status:               s.Status,
			ConclusionConfidence: s.ConclusionConfidence,
			UpdatedAt:            s.UpdatedAt,
			IsActive:             s.IsActive,
			Archived:             s.Archived,
			Counts:               s.Counts,
		})
	}
	lowerBound := len(sums) >= listScanLimit
	return listInvestigationsOut{
		Investigations:    items,
		Total:             len(sums),
		TotalIsLowerBound: lowerBound,
		Truncated:         len(sums) > n || lowerBound,
		ActiveRef:         activeRef,
	}
}

// renderInvestigationList is the compact plain-text form of a list; filtered
// says whether include_archived or status narrowed or widened it.
func renderInvestigationList(out listInvestigationsOut, filtered bool) string {
	if out.Total == 0 {
		if filtered {
			return "No investigations match."
		}
		return "No investigations yet. Call casebook_create_investigation."
	}
	var b strings.Builder
	for _, it := range out.Investigations {
		active := ""
		if it.IsActive {
			active = ", active"
		}
		c := it.Counts
		fmt.Fprintf(&b, "%s [%s, %s%s] %s (%d evidence, %d claims, %d hypotheses, %d open questions; updated %s)\n",
			it.Ref, it.Status, it.ConclusionConfidence, active, it.Title,
			c.Evidence, c.Claims, c.Hypotheses, c.OpenQuestions, it.UpdatedAt)
	}
	if out.Truncated {
		fmt.Fprintf(&b, "Showing %d of %s.", len(out.Investigations), totalText(out.Total, out.TotalIsLowerBound))
	}
	return strings.TrimRight(b.String(), "\n")
}

// casebook_search_investigations

const searchInvestigationsDescription = `Full-text search over titles, problem statements and conclusions of cases (investigation records), evidence titles and summaries (not raw detail), claim and hypothesis statements, and questions. ` +
	`Use it to find earlier work or a record to read or link. ` +
	`query is plain words, not query syntax: every word must match; the last also matches as a prefix. ` +
	`Unlike other tools, investigation_ref omitted or empty searches ALL cases, not the active one; pass a ref such as INV-0001 to search one. ` +
	`Archived cases are skipped unless include_archived is true or the ref names one. ` +
	`limit defaults to 20 (max 100); best hits first. At most 100 hits are counted; total_is_lower_bound true means maybe more. ` +
	`Each hit gives investigation_ref, record_type, record_ref (E-001, C-001, H-001, Q-001 or the INV ref) and a snippet with matches in **; read records with casebook_get_investigation or casebook_get_evidence.`

// searchScanLimit is how many hits the search tool reads (the app maximum),
// so total is exact below that many hits and a lower bound at it.
const searchScanLimit = 100

type searchInput struct {
	Query            string `json:"query" jsonschema:"plain words to search for"`
	InvestigationRef string `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001 to search one investigation; omit to search all"`
	IncludeArchived  bool   `json:"include_archived,omitempty" jsonschema:"also search archived investigations"`
	Limit            int    `json:"limit,omitempty" jsonschema:"maximum hits (default 20, max 100)"`
}

// searchHitOut is one compact search hit (best first; the rank is dropped).
type searchHitOut struct {
	InvestigationRef   string         `json:"investigation_ref"`
	InvestigationTitle string         `json:"investigation_title"`
	RecordType         app.RecordType `json:"record_type"`
	RecordRef          string         `json:"record_ref"`
	Snippet            string         `json:"snippet"`
}

type searchOut struct {
	Query string         `json:"query"`
	Hits  []searchHitOut `json:"hits"`
	Total int            `json:"total"`
	// TotalIsLowerBound is true when the scan reached searchScanLimit:
	// there may be more hits than Total.
	TotalIsLowerBound bool `json:"total_is_lower_bound" jsonschema:"true when total is a lower bound: there may be more hits than total"`
	Truncated         bool `json:"truncated"`
}

func (h *handlers) searchInvestigations(ctx context.Context, _ call, in searchInput) (searchOut, string, error) {
	limit, err := limitParam("limit", in.Limit, defaultListLimit, maxListLimit)
	if err != nil {
		return searchOut{}, "", err
	}
	hits, err := h.app.Search(ctx, in.Query, app.SearchOptions{
		InvestigationRef: in.InvestigationRef,
		IncludeArchived:  in.IncludeArchived,
		Limit:            searchScanLimit,
	})
	if err != nil {
		return searchOut{}, "", err
	}
	for {
		out := buildSearchOut(in.Query, hits, limit)
		text := renderSearchOut(out)
		if resultBytes(text, out) <= responseTarget || limit <= 1 {
			return out, text, nil
		}
		limit /= 2
	}
}

func buildSearchOut(query string, hits []app.SearchHit, limit int) searchOut {
	n := min(len(hits), limit)
	out := make([]searchHitOut, 0, n)
	for _, hit := range hits[:n] {
		out = append(out, searchHitOut{
			InvestigationRef:   hit.InvestigationRef,
			InvestigationTitle: hit.InvestigationTitle,
			RecordType:         hit.RecordType,
			RecordRef:          hit.RecordRef,
			Snippet:            hit.Snippet,
		})
	}
	lowerBound := len(hits) >= searchScanLimit
	return searchOut{Query: query, Hits: out, Total: len(hits), TotalIsLowerBound: lowerBound, Truncated: len(hits) > n || lowerBound}
}

// totalText is the total of a cut list for text content: "at least N" when
// it is a lower bound.
func totalText(total int, lowerBound bool) string {
	if lowerBound {
		return fmt.Sprintf("at least %d", total)
	}
	return fmt.Sprint(total)
}

func renderSearchOut(out searchOut) string {
	if out.Total == 0 {
		return fmt.Sprintf("No matches for %q.", out.Query)
	}
	var b strings.Builder
	for _, hit := range out.Hits {
		fmt.Fprintf(&b, "%s %s %s: %s\n", hit.InvestigationRef, hit.RecordType, hit.RecordRef, app.TruncateLine(hit.Snippet, 0))
	}
	if out.Truncated {
		fmt.Fprintf(&b, "Showing %d of %s.", len(out.Hits), totalText(out.Total, out.TotalIsLowerBound))
	}
	return strings.TrimRight(b.String(), "\n")
}

// D-14: the update inputs mirror the app input field names; a nil pointer
// (omitted or JSON null) means unchanged.

// casebook_update_investigation

const updateInvestigationDescription = `Change the title, problem statement or status of a case (an investigation record). ` +
	`Patch semantics: pass only the fields to change; an omitted or null field stays unchanged; at least one is required. ` +
	`status archived archives it (it stays readable and stays active if it was); a non-archived status on an archived case unarchives it to that status. Use resolved when the work is done. ` +
	`To record the answer itself call casebook_update_conclusion. ` +
	`Returns action updated with the changed field names, or unchanged when nothing differed, and a compact record.`

type updateInvestigationInput struct {
	InvestigationRef string                   `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	Title            *string                  `json:"title,omitempty" jsonschema:"new one-line title (at most 200 bytes)"`
	ProblemStatement *string                  `json:"problem_statement,omitempty" jsonschema:"new problem statement"`
	Status           *app.InvestigationStatus `json:"status,omitempty" jsonschema:"new status; archived archives"`
}

var investigationFieldOrder = []string{"title", "problem_statement", "status"}

func investigationFields(inv app.Investigation) map[string]string {
	return map[string]string{
		"title":             inv.Title,
		"problem_statement": inv.ProblemStatement,
		"status":            string(inv.Status),
	}
}

func (h *handlers) updateInvestigation(ctx context.Context, c call, in updateInvestigationInput) (mutationResult[investigationRecord], string, error) {
	var zero mutationResult[investigationRecord]
	if in.Title == nil && in.ProblemStatement == nil && in.Status == nil {
		// A tool shape rule, not engine re-validation: an empty patch is
		// almost always a mistake by the caller.
		return zero, "", app.NewValidationError("patch", "pass at least one of title, problem_statement or status")
	}
	before, err := h.app.GetInvestigation(ctx, in.InvestigationRef)
	if err != nil {
		return zero, "", err
	}
	// Write by the canonical ref so a concurrent active switch cannot
	// redirect the update.
	after, err := h.app.UpdateInvestigation(ctx, c.Actor, before.Ref, app.InvestigationPatch{
		Title:            in.Title,
		ProblemStatement: in.ProblemStatement,
		Status:           in.Status,
	})
	if err != nil {
		return zero, "", err
	}
	changed := changedFields(investigationFieldOrder, investigationFields(before), investigationFields(after))
	out := mutationResult[investigationRecord]{
		Ref:              after.Ref,
		InvestigationRef: after.Ref,
		Action:           "updated",
		Changed:          changed,
		Record:           investigationRecordOf(after),
	}
	if len(changed) == 0 {
		out.Action = "unchanged"
		return out, fmt.Sprintf("No change to %s.", after.Ref), nil
	}
	return out, fmt.Sprintf("Updated %s: %s.", after.Ref, strings.Join(changed, ", ")), nil
}

// casebook_update_conclusion

const updateConclusionDescription = `Record the conclusion, confidence and rationale together; empty conclusion clears them. Omitted confidence means unknown. ` +
	`Select supporting_claims and contradicting_claims by ref; omitted arrays preserve selection, [] clears one. ` +
	`Only a person can human-verify or record a review. AI text edits reset checked verification; edits clear review. Changed selected claims or evidence flag review needed while preserving the historical statement label. ` +
	`Returns updated with changed fields, or unchanged.`

type updateConclusionInput struct {
	InvestigationRef    string                `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	Conclusion          string                `json:"conclusion" jsonschema:"the conclusion; an empty string clears it"`
	Confidence          app.Confidence        `json:"confidence,omitempty" jsonschema:"confidence in the conclusion; omit for unknown"`
	Rationale           string                `json:"rationale,omitempty" jsonschema:"why the conclusion holds, naming the supporting evidence and claims"`
	Verification        statementVerification `json:"verification,omitempty" jsonschema:"ai_generated, source_verified or disputed; only a person can human-verify a conclusion"`
	SupportingClaims    []string              `json:"supporting_claims,omitempty" jsonschema:"selected claim refs supporting the conclusion in this investigation; omitted preserves selection"`
	ContradictingClaims []string              `json:"contradicting_claims,omitempty" jsonschema:"selected claim refs contradicting the conclusion in this investigation; omitted preserves selection"`
}

var conclusionFieldOrder = []string{"conclusion", "conclusion_confidence", "conclusion_rationale", "conclusion_verification", "conclusion_supporting_claims", "conclusion_contradicting_claims", "conclusion_reviewed_at", "conclusion_review_basis", "conclusion_needs_review"}

func conclusionFields(inv app.Investigation) map[string]string {
	return map[string]string{
		"conclusion":                      inv.Conclusion,
		"conclusion_confidence":           string(inv.ConclusionConfidence),
		"conclusion_rationale":            inv.ConclusionRationale,
		"conclusion_verification":         string(inv.ConclusionVerification),
		"conclusion_supporting_claims":    strings.Join(inv.ConclusionSupportingClaims, ","),
		"conclusion_contradicting_claims": strings.Join(inv.ConclusionContradictingClaims, ","),
		"conclusion_reviewed_at":          inv.ConclusionReviewedAt,
		"conclusion_review_basis":         inv.ConclusionReviewBasis,
		"conclusion_needs_review":         fmt.Sprint(inv.ConclusionNeedsReview),
	}
}

func (h *handlers) updateConclusion(ctx context.Context, c call, in updateConclusionInput) (mutationResult[investigationRecord], string, error) {
	var zero mutationResult[investigationRecord]
	before, err := h.app.GetInvestigation(ctx, in.InvestigationRef)
	if err != nil {
		return zero, "", err
	}
	after, err := h.app.UpdateConclusion(ctx, c.Actor, before.Ref, app.ConclusionInput{
		Conclusion:   in.Conclusion,
		Confidence:   in.Confidence,
		Rationale:    in.Rationale,
		Verification: app.Verification(in.Verification), SupportingClaims: in.SupportingClaims, ContradictingClaims: in.ContradictingClaims,
	})
	if err != nil {
		return zero, "", err
	}
	changed := changedFields(conclusionFieldOrder, conclusionFields(before), conclusionFields(after))
	out := mutationResult[investigationRecord]{
		Ref:              after.Ref,
		InvestigationRef: after.Ref,
		Action:           "updated",
		Changed:          changed,
		Record:           investigationRecordOf(after),
	}
	switch {
	case len(changed) == 0:
		out.Action = "unchanged"
		return out, fmt.Sprintf("No change to the conclusion of %s.", after.Ref), nil
	case after.Conclusion == "":
		return out, fmt.Sprintf("Cleared the conclusion of %s.", after.Ref), nil
	default:
		return out, fmt.Sprintf("Updated the conclusion of %s (confidence %s).", after.Ref, after.ConclusionConfidence), nil
	}
}
