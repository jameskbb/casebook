package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
)

// Claims and hypotheses share one implementation, the way internal/app does:
// the field structs below differ only in their status enum, and stmtKind
// holds the per-kind app calls so the add, update and link plumbing is
// written once. The checkpoint batch embeds the field structs (converted
// with toInput and toPatch for app.CheckpointBatch), so none of them
// carries investigation_ref or a record ref.

// linkInput asks for one evidence link when adding a claim or hypothesis.
type linkInput struct {
	EvidenceRef string       `json:"evidence_ref" jsonschema:"evidence ref such as E-001 (case-insensitive), or qualified INV-0001/E-001; must be in the same investigation"`
	Relation    app.Relation `json:"relation" jsonschema:"how the evidence bears on the statement"`
}

func linkInputs(links []linkInput) []app.EvidenceLinkInput {
	out := make([]app.EvidenceLinkInput, len(links))
	for i, l := range links {
		out[i] = app.EvidenceLinkInput{EvidenceRef: l.EvidenceRef, Relation: l.Relation}
	}
	return out
}

// claimFields are the claim input fields, shared by casebook_add_claim and
// the checkpoint claims batch.
type claimFields struct {
	Statement    string                `json:"statement" jsonschema:"what you believe is true, one or two sentences (at most 2000 bytes)"`
	Confidence   app.Confidence        `json:"confidence,omitempty" jsonschema:"how confident you are; defaults to unknown"`
	Rationale    string                `json:"rationale,omitempty" jsonschema:"why that confidence (the claim's confidence rationale, at most 2000 bytes)"`
	Status       app.ClaimStatus       `json:"status,omitempty" jsonschema:"defaults to active; superseded is not allowed here (use casebook_update_claim with superseded_by)"`
	Verification statementVerification `json:"verification,omitempty" jsonschema:"defaults to ai_generated; disputed also makes the status disputed; only a person can set human_verified"`
	Links        []linkInput           `json:"links,omitempty" jsonschema:"evidence to link in the same call, each with relation supports or contradicts"`
}

func (f claimFields) toInput() app.ClaimInput {
	return app.ClaimInput{
		Statement:           f.Statement,
		Confidence:          f.Confidence,
		ConfidenceRationale: f.Rationale,
		Status:              f.Status,
		Verification:        app.Verification(f.Verification),
		Links:               linkInputs(f.Links),
	}
}

// hypothesisFields are the hypothesis input fields, shared by
// casebook_add_hypothesis and the checkpoint hypotheses batch.
type hypothesisFields struct {
	Statement    string                `json:"statement" jsonschema:"the candidate explanation, one or two sentences (at most 2000 bytes)"`
	Confidence   app.Confidence        `json:"confidence,omitempty" jsonschema:"how likely it seems; defaults to unknown"`
	Rationale    string                `json:"rationale,omitempty" jsonschema:"why that confidence (at most 2000 bytes)"`
	Status       app.HypothesisStatus  `json:"status,omitempty" jsonschema:"defaults to open; superseded is not allowed here (use casebook_update_hypothesis with superseded_by)"`
	Verification statementVerification `json:"verification,omitempty" jsonschema:"defaults to ai_generated; only a person can set human_verified"`
	Links        []linkInput           `json:"links,omitempty" jsonschema:"evidence to link in the same call, each with relation supports or contradicts"`
}

func (f hypothesisFields) toInput() app.HypothesisInput {
	return app.HypothesisInput{
		Statement:    f.Statement,
		Confidence:   f.Confidence,
		Rationale:    f.Rationale,
		Status:       f.Status,
		Verification: app.Verification(f.Verification),
		Links:        linkInputs(f.Links),
	}
}

// claimPatchFields are the claim patch fields, shared by
// casebook_update_claim and the checkpoint claim_updates batch. Omitted or
// null means unchanged.
type claimPatchFields struct {
	Statement    *string                `json:"statement,omitempty" jsonschema:"new statement"`
	Confidence   *app.Confidence        `json:"confidence,omitempty" jsonschema:"new confidence"`
	Rationale    *string                `json:"rationale,omitempty" jsonschema:"new confidence rationale"`
	Status       *app.ClaimStatus       `json:"status,omitempty" jsonschema:"new status; to supersede pass superseded_by"`
	Verification *statementVerification `json:"verification,omitempty" jsonschema:"new verification; disputed also disputes the claim, and any other value on a disputed claim makes it active again; only a person can set human_verified"`
	SupersededBy *string                `json:"superseded_by,omitempty" jsonschema:"ref of the claim that replaces this one (same investigation); this one becomes superseded"`
}

func (p claimPatchFields) empty() bool {
	return p.Statement == nil && p.Confidence == nil && p.Rationale == nil && p.Status == nil &&
		p.Verification == nil && p.SupersededBy == nil
}

func (p claimPatchFields) toPatch() app.ClaimPatch {
	return app.ClaimPatch{
		Statement:           p.Statement,
		ConfidenceRationale: p.Rationale,
		Confidence:          p.Confidence,
		Status:              p.Status,
		Verification:        verificationPtr(p.Verification),
		SupersededBy:        p.SupersededBy,
	}
}

// hypothesisPatchFields are the hypothesis patch fields, shared by
// casebook_update_hypothesis and the checkpoint hypothesis_updates batch.
type hypothesisPatchFields struct {
	Statement    *string                `json:"statement,omitempty" jsonschema:"new statement"`
	Confidence   *app.Confidence        `json:"confidence,omitempty" jsonschema:"new confidence"`
	Rationale    *string                `json:"rationale,omitempty" jsonschema:"new rationale"`
	Status       *app.HypothesisStatus  `json:"status,omitempty" jsonschema:"new status, for example supported or disproven"`
	Verification *statementVerification `json:"verification,omitempty" jsonschema:"new verification state; only a person can set human_verified; a new statement resets human_verified or source_verified to ai_generated unless you pass one"`
	SupersededBy *string                `json:"superseded_by,omitempty" jsonschema:"ref of the hypothesis that replaces this one (same investigation); this one becomes superseded"`
}

func (p hypothesisPatchFields) empty() bool {
	return p.Statement == nil && p.Confidence == nil && p.Rationale == nil && p.Status == nil &&
		p.Verification == nil && p.SupersededBy == nil
}

func (p hypothesisPatchFields) toPatch() app.HypothesisPatch {
	return app.HypothesisPatch{
		Statement:    p.Statement,
		Rationale:    p.Rationale,
		Confidence:   p.Confidence,
		Status:       p.Status,
		Verification: verificationPtr(p.Verification),
		SupersededBy: p.SupersededBy,
	}
}

// stmtState is the kind-independent view of a claim or hypothesis that the
// shared plumbing compares and reports.
type stmtState struct {
	ref              string
	investigationRef string
	superseded       bool
	// fields holds the patchable values by tool field name (stmtFieldOrder).
	fields map[string]string
	// links maps evidence ref to relation.
	links  map[string]app.Relation
	record statementRecord
}

// stmtFieldOrder is the order changed field names are reported in.
var stmtFieldOrder = []string{"statement", "confidence", "rationale", "status", "verification", "superseded_by"}

func claimState(c app.Claim) stmtState {
	return newStmtState(c.Ref, c.InvestigationRef, c.Status == app.ClaimStatusSuperseded, c.Statement,
		string(c.Confidence), c.ConfidenceRationale, string(c.Status), string(c.Verification), c.SupersededBy,
		c.Links, claimRecordOf(c, false))
}

func hypothesisState(h app.Hypothesis) stmtState {
	return newStmtState(h.Ref, h.InvestigationRef, h.Status == app.HypothesisStatusSuperseded, h.Statement,
		string(h.Confidence), h.Rationale, string(h.Status), string(h.Verification), h.SupersededBy,
		h.Links, hypothesisRecordOf(h, false))
}

func newStmtState(ref, invRef string, superseded bool, statement, confidence, rationale, status, verification,
	supersededBy string, links []app.EvidenceLink, rec statementRecord) stmtState {
	s := stmtState{
		ref:              ref,
		investigationRef: invRef,
		superseded:       superseded,
		fields: map[string]string{
			"statement":     statement,
			"confidence":    confidence,
			"rationale":     rationale,
			"status":        status,
			"verification":  verification,
			"superseded_by": supersededBy,
		},
		links:  make(map[string]app.Relation, len(links)),
		record: rec,
	}
	for _, l := range links {
		s.links[l.EvidenceRef] = l.Relation
	}
	return s
}

// stmtKind holds the per-kind app calls. Every func takes the investigation
// and record refs as given; callers pass canonical refs read back first.
type stmtKind struct {
	noun   string // claim or hypothesis
	get    func(ctx context.Context, a *app.App, invRef, ref string) (stmtState, error)
	link   func(ctx context.Context, a *app.App, actor app.Actor, invRef, ref, evidenceRef string, rel app.Relation) (stmtState, error)
	unlink func(ctx context.Context, a *app.App, actor app.Actor, invRef, ref, evidenceRef string) (stmtState, error)
}

var claimKind = stmtKind{
	noun: "claim",
	get: func(ctx context.Context, a *app.App, invRef, ref string) (stmtState, error) {
		c, err := a.GetClaim(ctx, invRef, ref)
		return claimState(c), err
	},
	link: func(ctx context.Context, a *app.App, actor app.Actor, invRef, ref, evidenceRef string, rel app.Relation) (stmtState, error) {
		c, err := a.LinkEvidenceToClaim(ctx, actor, invRef, ref, evidenceRef, rel)
		return claimState(c), err
	},
	unlink: func(ctx context.Context, a *app.App, actor app.Actor, invRef, ref, evidenceRef string) (stmtState, error) {
		c, err := a.UnlinkEvidenceFromClaim(ctx, actor, invRef, ref, evidenceRef)
		return claimState(c), err
	},
}

var hypothesisKind = stmtKind{
	noun: "hypothesis",
	get: func(ctx context.Context, a *app.App, invRef, ref string) (stmtState, error) {
		h, err := a.GetHypothesis(ctx, invRef, ref)
		return hypothesisState(h), err
	},
	link: func(ctx context.Context, a *app.App, actor app.Actor, invRef, ref, evidenceRef string, rel app.Relation) (stmtState, error) {
		h, err := a.LinkEvidenceToHypothesis(ctx, actor, invRef, ref, evidenceRef, rel)
		return hypothesisState(h), err
	},
	unlink: func(ctx context.Context, a *app.App, actor app.Actor, invRef, ref, evidenceRef string) (stmtState, error) {
		h, err := a.UnlinkEvidenceFromHypothesis(ctx, actor, invRef, ref, evidenceRef)
		return hypothesisState(h), err
	},
}

// createdResult is the mutation result of an add.
func createdResult(s stmtState) mutationResult[statementRecord] {
	return mutationResult[statementRecord]{
		Ref:              s.ref,
		InvestigationRef: s.investigationRef,
		Action:           "created",
		Changed:          []string{},
		Record:           s.record,
	}
}

// addClaim adds a claim with its evidence links in one app call.
func addClaim(ctx context.Context, a *app.App, actor app.Actor, invRef string, f claimFields) (mutationResult[statementRecord], error) {
	c, err := a.AddClaim(ctx, actor, invRef, f.toInput())
	if err != nil {
		return mutationResult[statementRecord]{}, err
	}
	return createdResult(claimState(c)), nil
}

// addHypothesis adds a hypothesis with its evidence links in one app call.
func addHypothesis(ctx context.Context, a *app.App, actor app.Actor, invRef string, f hypothesisFields) (mutationResult[statementRecord], error) {
	h, err := a.AddHypothesis(ctx, actor, invRef, f.toInput())
	if err != nil {
		return mutationResult[statementRecord]{}, err
	}
	return createdResult(hypothesisState(h)), nil
}

// createdText is the content text of an add:
// "Added claim C-004 to INV-0001 (medium, ai_generated; supports E-001; contradicts E-003)."
func createdText(noun string, out mutationResult[statementRecord]) string {
	parts := []string{string(out.Record.Confidence) + ", " + string(out.Record.Verification)}
	if len(out.Record.Supports) > 0 {
		parts = append(parts, "supports "+strings.Join(out.Record.Supports, ", "))
	}
	if len(out.Record.Contradicts) > 0 {
		parts = append(parts, "contradicts "+strings.Join(out.Record.Contradicts, ", "))
	}
	if out.Record.LinksOmitted > 0 {
		parts = append(parts, fmt.Sprintf("%d more links omitted", out.Record.LinksOmitted))
	}
	return fmt.Sprintf("Added %s %s to %s (%s).", noun, out.Ref, out.InvestigationRef, strings.Join(parts, "; "))
}

// casebook_add_claim

type addClaimInput struct {
	InvestigationRef string `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	claimFields
}

const addClaimDescription = `Record a claim: something you believe is true about the problem, backed by evidence. ` +
	`Link evidence in the same call with links [{evidence_ref, relation}] where relation is supports or contradicts; the evidence must be in the same case (an investigation record). ` +
	`status defaults to active and confidence to unknown (low, medium, high); rationale says why that confidence. Verification follows the caller: AI writes are ai_generated. ` +
	`To replace a claim later, add the new one and call casebook_update_claim on the old one with superseded_by. ` +
	`Returns the new C ref (C-001 style) and a compact record with status, confidence, verification and linked evidence refs; the statement is not echoed.`

func init() {
	registerTool("casebook_add_claim", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_add_claim",
			Title:       "Add claim",
			Description: addClaimDescription,
			Annotations: mutatingTool(false),
		}, h.addClaim)
	})
}

func (h *handlers) addClaim(ctx context.Context, c call, in addClaimInput) (mutationResult[statementRecord], string, error) {
	out, err := addClaim(ctx, h.app, c.Actor, in.InvestigationRef, in.claimFields)
	if err != nil {
		return out, "", err
	}
	return out, createdText("claim", out), nil
}

// applyUpdate is the update plumbing shared by claims and hypotheses: it
// rejects an empty patch, reads the record, writes by its canonical refs (so
// a concurrent active switch cannot redirect the write) and reports the
// changed field names.
func (k stmtKind) applyUpdate(ctx context.Context, a *app.App, invRef, ref string, empty bool,
	update func(invRef, ref string) (stmtState, app.UpdateOutcome, error)) (mutationResult[statementRecord], error) {
	var zero mutationResult[statementRecord]
	if empty {
		// A tool shape rule, not engine re-validation: an empty patch is
		// almost always a mistake by the caller.
		return zero, app.NewValidationError("patch",
			"pass at least one of statement, confidence, rationale, status, verification or superseded_by")
	}
	before, err := k.get(ctx, a, invRef, ref)
	if err != nil {
		return zero, err
	}
	after, outcome, err := update(before.investigationRef, before.ref)
	if err != nil {
		return zero, err
	}
	changed := changedFields(stmtFieldOrder, before.fields, after.fields)
	action := "unchanged"
	switch {
	case after.superseded && !before.superseded:
		action = "superseded"
	case len(changed) > 0:
		action = "updated"
	}
	return mutationResult[statementRecord]{
		Ref:              after.ref,
		InvestigationRef: after.investigationRef,
		Action:           action,
		Changed:          changed,
		Record:           after.record,
		resetFields:      resetFieldsOf(outcome),
	}, nil
}

// applyClaimUpdate patches a claim; rationale is the confidence rationale.
func applyClaimUpdate(ctx context.Context, a *app.App, actor app.Actor, invRef, ref string, p claimPatchFields) (mutationResult[statementRecord], error) {
	return claimKind.applyUpdate(ctx, a, invRef, ref, p.empty(), func(invRef, ref string) (stmtState, app.UpdateOutcome, error) {
		c, outcome, err := a.UpdateClaimOutcome(ctx, actor, invRef, ref, p.toPatch())
		return claimState(c), outcome, err
	})
}

// applyHypothesisUpdate patches a hypothesis.
func applyHypothesisUpdate(ctx context.Context, a *app.App, actor app.Actor, invRef, ref string, p hypothesisPatchFields) (mutationResult[statementRecord], error) {
	return hypothesisKind.applyUpdate(ctx, a, invRef, ref, p.empty(), func(invRef, ref string) (stmtState, app.UpdateOutcome, error) {
		h, outcome, err := a.UpdateHypothesisOutcome(ctx, actor, invRef, ref, p.toPatch())
		return hypothesisState(h), outcome, err
	})
}

// updateText is the content text of an update.
func updateText(out mutationResult[statementRecord]) string {
	switch out.Action {
	case "superseded":
		return fmt.Sprintf("%s is superseded by %s.", out.Ref, out.Record.SupersededBy)
	case "unchanged":
		return fmt.Sprintf("No change to %s.", out.Ref)
	default:
		text := fmt.Sprintf("Updated %s: %s.", out.Ref, strings.Join(out.Changed, ", "))
		if out.VerificationReset {
			text += " " + resetSentence(out.Ref, out.PreviousVerification)
		}
		return text
	}
}

// linkOutcome is a link tool result plus what the content text names.
type linkOutcome struct {
	result      mutationResult[statementRecord]
	evidenceRef string // canonical evidence ref; "" when unchanged
	relation    app.Relation
}

// applyLink links (relation set) or unlinks (unlink true) one evidence and
// reports linked, relinked, unchanged or unlinked by comparing the link sets
// before and after. Exactly one of relation and unlink must be given.
func (k stmtKind) applyLink(ctx context.Context, a *app.App, actor app.Actor, invRef, ref, evidenceRef string,
	rel app.Relation, unlink bool) (linkOutcome, error) {
	if (rel != "") == unlink {
		return linkOutcome{}, app.NewValidationError("relation",
			"pass relation (supports or contradicts) to link, or unlink: true to remove the link")
	}
	before, err := k.get(ctx, a, invRef, ref)
	if err != nil {
		return linkOutcome{}, err
	}
	var after stmtState
	if unlink {
		after, err = k.unlink(ctx, a, actor, before.investigationRef, before.ref, evidenceRef)
	} else {
		after, err = k.link(ctx, a, actor, before.investigationRef, before.ref, evidenceRef, rel)
	}
	if err != nil {
		return linkOutcome{}, err
	}
	o := linkOutcome{
		result: mutationResult[statementRecord]{
			Ref:              after.ref,
			InvestigationRef: after.investigationRef,
			Action:           "unchanged",
			Changed:          []string{},
			Record:           after.record,
		},
		relation: rel,
	}
	if unlink {
		for e, r := range before.links {
			if _, ok := after.links[e]; !ok {
				o.evidenceRef, o.relation = e, r
			}
		}
		o.result.Action = "unlinked"
	} else {
		for e, r := range after.links {
			old, ok := before.links[e]
			switch {
			case !ok:
				o.evidenceRef, o.result.Action = e, "linked"
			case old != r:
				o.evidenceRef, o.result.Action = e, "relinked"
			}
		}
	}
	if o.result.Action != "unchanged" {
		o.result.Changed = []string{"links"}
	}
	return o, nil
}

// linkText is the content text of a link tool.
func linkText(o linkOutcome) string {
	ref := o.result.Ref
	switch o.result.Action {
	case "linked":
		return fmt.Sprintf("Linked %s to %s as %s.", o.evidenceRef, ref, o.relation)
	case "relinked":
		return fmt.Sprintf("Relinked %s to %s as %s.", o.evidenceRef, ref, o.relation)
	case "unlinked":
		return fmt.Sprintf("Unlinked %s from %s.", o.evidenceRef, ref)
	default:
		return fmt.Sprintf("No change to %s.", ref)
	}
}

// casebook_update_claim

type updateClaimInput struct {
	InvestigationRef string `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	ClaimRef         string `json:"claim_ref" jsonschema:"claim ref such as C-001 (case-insensitive), or qualified INV-0001/C-001"`
	claimPatchFields
}

const updateClaimDescription = `Revise a claim as understanding changes: statement, confidence (unknown, low, medium, high), rationale (the claim's confidence rationale), status (active, disputed, rejected) or verification (ai_generated, source_verified, disputed; only a person sets human_verified, but you may dispute it). A new statement resets a verified claim to ai_generated unless you pass source_verified. ` +
	`Status and verification stay in step: disputed on either one disputes the claim, and verifying a disputed claim makes it active again. ` +
	`Patch semantics: an omitted or null field is unchanged; pass at least one. ` +
	`To replace a claim, add the new one, then set superseded_by to its ref (C-002 style) from the same case (an investigation record): this one becomes superseded and leaves the context pack. ` +
	`` +
	`Returns action updated, superseded or unchanged with the changed field names and a compact record; text is not echoed.`

// casebook_update_hypothesis

type updateHypothesisInput struct {
	InvestigationRef string `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	HypothesisRef    string `json:"hypothesis_ref" jsonschema:"hypothesis ref such as H-001 (case-insensitive), or qualified INV-0001/H-001"`
	hypothesisPatchFields
}

const updateHypothesisDescription = `Record what testing a hypothesis showed: set status supported or disproven (or back to open), or change statement, confidence (unknown, low, medium, high), rationale or verification. ` +
	`Patch semantics: an omitted or null field is unchanged; pass at least one. Disproven hypotheses stay in the context pack as one-liners naming the evidence linked as contradicting them, so nobody re-tests them: link that evidence with relation contradicts. ` +
	`To replace a hypothesis, add the new one, then set superseded_by to its ref (H-002 style) from the same case (an investigation record). ` +
	`` +
	`Returns action updated, superseded or unchanged with the changed field names and a compact record.`

// casebook_link_evidence_to_claim and casebook_link_evidence_to_hypothesis

type linkClaimInput struct {
	InvestigationRef string       `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	ClaimRef         string       `json:"claim_ref" jsonschema:"claim ref such as C-001 (case-insensitive), or qualified INV-0001/C-001"`
	EvidenceRef      string       `json:"evidence_ref" jsonschema:"evidence ref such as E-001 (case-insensitive), or qualified INV-0001/E-001; same investigation"`
	Relation         app.Relation `json:"relation,omitempty" jsonschema:"supports or contradicts; omit when unlink is true"`
	Unlink           bool         `json:"unlink,omitempty" jsonschema:"true removes the link instead; omit relation then"`
}

type linkHypothesisInput struct {
	InvestigationRef string       `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	HypothesisRef    string       `json:"hypothesis_ref" jsonschema:"hypothesis ref such as H-001 (case-insensitive), or qualified INV-0001/H-001"`
	EvidenceRef      string       `json:"evidence_ref" jsonschema:"evidence ref such as E-001 (case-insensitive), or qualified INV-0001/E-001; same investigation"`
	Relation         app.Relation `json:"relation,omitempty" jsonschema:"supports or contradicts; omit when unlink is true"`
	Unlink           bool         `json:"unlink,omitempty" jsonschema:"true removes the link instead; omit relation then"`
}

func linkDescription(noun string) string {
	return `Tie one piece of evidence to a ` + noun + `: pass relation supports or contradicts to link, or unlink: true to remove the link (exactly one of the two). ` +
		`One call per evidence and ` + noun + ` pair; linking again with the other relation updates it, and the same relation again changes nothing. ` +
		`Both must be in the same case (an investigation record). ` +
		`Returns action linked, relinked, unchanged or unlinked and a compact record listing the supporting and contradicting evidence refs.`
}

// casebook_add_hypothesis

type addHypothesisInput struct {
	InvestigationRef string `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	hypothesisFields
}

const addHypothesisDescription = `Record a hypothesis: a candidate explanation that is still being tested. Record each one when you start testing it, including ones you expect to rule out, so the next session knows it was considered. ` +
	`Status defaults to open and confidence to unknown (low, medium, high); link evidence in the same call with links [{evidence_ref, relation}] where relation is supports or contradicts; the evidence must be in the same case (an investigation record). ` +
	`Mark it supported or disproven later with casebook_update_hypothesis; disproven hypotheses stay in the context pack as one-liners so they are not re-tried. ` +
	`Returns the new H ref (H-001 style) and a compact record with linked evidence refs; the statement is not echoed.`

func init() {
	registerTool("casebook_update_claim", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_update_claim",
			Title:       "Update claim",
			Description: updateClaimDescription,
			Annotations: mutatingTool(true),
		}, h.updateClaim)
	})
	registerTool("casebook_link_evidence_to_claim", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_link_evidence_to_claim",
			Title:       "Link evidence to claim",
			Description: linkDescription("claim"),
			Annotations: mutatingTool(true),
		}, h.linkEvidenceToClaim)
	})
	registerTool("casebook_add_hypothesis", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_add_hypothesis",
			Title:       "Add hypothesis",
			Description: addHypothesisDescription,
			Annotations: mutatingTool(false),
		}, h.addHypothesis)
	})
	registerTool("casebook_update_hypothesis", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_update_hypothesis",
			Title:       "Update hypothesis",
			Description: updateHypothesisDescription,
			Annotations: mutatingTool(true),
		}, h.updateHypothesis)
	})
	registerTool("casebook_link_evidence_to_hypothesis", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_link_evidence_to_hypothesis",
			Title:       "Link evidence to hypothesis",
			Description: linkDescription("hypothesis"),
			Annotations: mutatingTool(true),
		}, h.linkEvidenceToHypothesis)
	})
}

func (h *handlers) updateClaim(ctx context.Context, c call, in updateClaimInput) (mutationResult[statementRecord], string, error) {
	out, err := applyClaimUpdate(ctx, h.app, c.Actor, in.InvestigationRef, in.ClaimRef, in.claimPatchFields)
	if err != nil {
		return out, "", err
	}
	return out, updateText(out), nil
}

func (h *handlers) updateHypothesis(ctx context.Context, c call, in updateHypothesisInput) (mutationResult[statementRecord], string, error) {
	out, err := applyHypothesisUpdate(ctx, h.app, c.Actor, in.InvestigationRef, in.HypothesisRef, in.hypothesisPatchFields)
	if err != nil {
		return out, "", err
	}
	return out, updateText(out), nil
}

func (h *handlers) linkEvidenceToClaim(ctx context.Context, c call, in linkClaimInput) (mutationResult[statementRecord], string, error) {
	o, err := claimKind.applyLink(ctx, h.app, c.Actor, in.InvestigationRef, in.ClaimRef, in.EvidenceRef, in.Relation, in.Unlink)
	if err != nil {
		return o.result, "", err
	}
	return o.result, linkText(o), nil
}

func (h *handlers) linkEvidenceToHypothesis(ctx context.Context, c call, in linkHypothesisInput) (mutationResult[statementRecord], string, error) {
	o, err := hypothesisKind.applyLink(ctx, h.app, c.Actor, in.InvestigationRef, in.HypothesisRef, in.EvidenceRef, in.Relation, in.Unlink)
	if err != nil {
		return o.result, "", err
	}
	return o.result, linkText(o), nil
}

func (h *handlers) addHypothesis(ctx context.Context, c call, in addHypothesisInput) (mutationResult[statementRecord], string, error) {
	out, err := addHypothesis(ctx, h.app, c.Actor, in.InvestigationRef, in.hypothesisFields)
	if err != nil {
		return out, "", err
	}
	return out, createdText("hypothesis", out), nil
}
