package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// Evidence is normally kept and marked superseded or disputed through
// SetEvidenceVerification (D-11); DeleteEvidence (delete.go) exists for
// mistakes and refuses evidence that claims or hypotheses still link. raw_detail is never logged and never
// appears in activity summaries.

// evidenceFields holds sanitized evidence values.
type evidenceFields struct {
	title, summary, rawDetail, sourceRef, sourceLabel, capturedAt, metadata string
	sourceType                                                              domain.SourceType
	verification                                                            domain.Verification
}

func cleanTitle(s string) (string, error) {
	return domain.SanitizeLine("title", s, domain.MaxTitleBytes, true)
}

func cleanSummary(s string) (string, error) {
	return domain.SanitizeText("summary", s, domain.MaxEvidenceSummaryBytes, true)
}

func cleanRawDetail(s string) (string, error) {
	return domain.SanitizeText("raw_detail", s, domain.MaxRawDetailBytes, false)
}

func cleanSourceRef(s string) (string, error) {
	return domain.SanitizeLine("source_ref", s, domain.MaxSourceRefBytes, false)
}

func cleanSourceLabel(s string) (string, error) {
	return domain.SanitizeLine("source_label", s, domain.MaxSourceLabelBytes, false)
}

// cleanCapturedAt returns the UTC timestamp, or "" when s is blank (meaning
// the creation time).
func cleanCapturedAt(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return "", nil
	}
	return domain.ParseTimestamp("captured_at", s)
}

func cleanSourceType(t domain.SourceType) error {
	if t == "" {
		return domain.NewValidationError("source_type", "is required")
	}
	return t.Validate("source_type")
}

// AddEvidence records evidence with source provenance under the next E ref of
// the investigation (empty invRef means the active one) and records
// evidence.add. Verification defaults by actor kind (DefaultVerification);
// captured_at defaults to the creation time. The result omits raw_detail
// (HasRawDetail reports it).
func (a *App) AddEvidence(ctx context.Context, actor Actor, invRef string, in EvidenceInput) (Evidence, error) {
	const op = "evidence.add"
	actor, err := actor.Validate()
	if err != nil {
		return Evidence{}, a.fail(op, err)
	}
	f, err := cleanEvidenceInput(actor, in)
	if err != nil {
		return Evidence{}, a.fail(op, err)
	}

	var out Evidence
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		ev, err := a.addEvidenceTx(ctx, tx, actor, invRef, f)
		out = ev
		return err
	})
	if err != nil {
		return Evidence{}, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", out.InvestigationRef, "ref", out.Ref)
	return out, nil
}

// addEvidenceTx allocates the next E ref, inserts the evidence, records
// evidence.add, touches the investigation and reads the evidence back
// (without raw detail).
func (a *App) addEvidenceTx(ctx context.Context, tx *sql.Tx, actor domain.Actor, invRef string,
	f evidenceFields) (Evidence, error) {
	now := a.timestamp()
	inv, err := a.resolveInvestigation(ctx, tx, invRef)
	if err != nil {
		return Evidence{}, err
	}
	num, err := store.AllocateRecordNum(ctx, tx, inv.ID, domain.RecordEvidence)
	if err != nil {
		return Evidence{}, err
	}
	ref := domain.FormatRecordRef(domain.RecordEvidence, num)
	captured := f.capturedAt
	if captured == "" {
		captured = now
	}
	if _, err := store.InsertEvidence(ctx, tx, store.NewEvidence{
		InvestigationID: inv.ID,
		RefNum:          num,
		Title:           f.title,
		Summary:         f.summary,
		RawDetail:       f.rawDetail,
		SourceType:      f.sourceType,
		SourceRef:       f.sourceRef,
		SourceLabel:     f.sourceLabel,
		CapturedAt:      captured,
		CreatedAt:       now,
		CreatedBy:       actor,
		Verification:    f.verification,
		Metadata:        f.metadata,
	}); err != nil {
		return Evidence{}, err
	}
	title := domain.TruncateLine(f.title, 120)
	if err := a.recordActivity(ctx, tx, actor, inv.ID, "evidence.add", ref,
		fmt.Sprintf("Added evidence %s %q", ref, title), "", now); err != nil {
		return Evidence{}, err
	}
	if err := store.TouchInvestigation(ctx, tx, inv.ID, now); err != nil {
		return Evidence{}, err
	}
	return store.GetEvidence(ctx, tx, inv.ID, num, false)
}

// cleanEvidenceInput validates and sanitizes every AddEvidence field.
func cleanEvidenceInput(actor domain.Actor, in EvidenceInput) (evidenceFields, error) {
	var (
		f   evidenceFields
		err error
	)
	if f.title, err = cleanTitle(in.Title); err != nil {
		return f, err
	}
	if f.summary, err = cleanSummary(in.Summary); err != nil {
		return f, err
	}
	if f.rawDetail, err = cleanRawDetail(in.RawDetail); err != nil {
		return f, err
	}
	if err = cleanSourceType(in.SourceType); err != nil {
		return f, err
	}
	f.sourceType = in.SourceType
	if f.sourceRef, err = cleanSourceRef(in.SourceRef); err != nil {
		return f, err
	}
	if f.sourceLabel, err = cleanSourceLabel(in.SourceLabel); err != nil {
		return f, err
	}
	if f.capturedAt, err = cleanCapturedAt(in.CapturedAt); err != nil {
		return f, err
	}
	f.verification = in.Verification
	if f.verification == "" {
		f.verification = domain.DefaultVerification(actor.Kind)
	} else if err = f.verification.Validate("verification"); err != nil {
		return f, err
	} else if err = checkVerifier(actor.Kind, f.verification); err != nil {
		return f, err
	}
	if f.metadata, err = domain.NormalizeMetadata("metadata", in.Metadata); err != nil {
		return f, err
	}
	return f, nil
}

// GetEvidence returns one piece of evidence with its claim and hypothesis
// links. raw_detail is included only with IncludeRawDetail. evidenceRef may be
// qualified (INV-0002/E-001); see resolveRecord for how it combines with
// invRef.
func (a *App) GetEvidence(ctx context.Context, invRef, evidenceRef string, o GetEvidenceOptions) (Evidence, error) {
	var out Evidence
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		_, ev, err := a.getEvidence(ctx, tx, invRef, evidenceRef, o.IncludeRawDetail)
		if err != nil {
			return err
		}
		out = ev
		return nil
	})
	if err != nil {
		return Evidence{}, a.fail("evidence.get", err)
	}
	return out, nil
}

// getEvidence resolves and loads evidence, mapping a missing row to
// ErrNotFound naming the ref.
func (a *App) getEvidence(ctx context.Context, q store.Querier, invRef, evidenceRef string, withRaw bool) (recordTarget, domain.Evidence, error) {
	t, err := a.resolveRecord(ctx, q, invRef, evidenceRef, domain.RecordEvidence)
	if err != nil {
		return recordTarget{}, domain.Evidence{}, err
	}
	ev, err := store.GetEvidence(ctx, q, t.Inv.ID, t.Num, withRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return recordTarget{}, domain.Evidence{}, t.notFound()
	}
	return t, ev, err
}

// ListEvidence returns the investigation's evidence ordered by ref, without
// raw_detail. The slice and every link slice are non-nil.
func (a *App) ListEvidence(ctx context.Context, invRef string) ([]Evidence, error) {
	var out []Evidence
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		inv, err := a.resolveInvestigation(ctx, tx, invRef)
		if err != nil {
			return err
		}
		list, err := store.ListEvidence(ctx, tx, inv.ID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, a.fail("evidence.list", err)
	}
	return out, nil
}

// UpdateEvidence applies a patch: nil fields stay unchanged and provided
// fields follow the AddEvidence rules (an empty CapturedAt resets it to the
// creation time). When something changes it records one evidence.update row
// listing the changed field names (never their values); otherwise it returns
// the evidence unchanged with no activity.
func (a *App) UpdateEvidence(ctx context.Context, actor Actor, invRef, evidenceRef string, p EvidencePatch) (Evidence, error) {
	ev, _, err := a.UpdateEvidenceOutcome(ctx, actor, invRef, evidenceRef, p)
	return ev, err
}

// UpdateEvidenceOutcome is UpdateEvidence that also reports whether the edit
// reset a vouched verification to ai_generated (ADR-0014) and the label it
// replaced.
func (a *App) UpdateEvidenceOutcome(ctx context.Context, actor Actor, invRef, evidenceRef string, p EvidencePatch) (Evidence, UpdateOutcome, error) {
	const op = "evidence.update"
	actor, err := actor.Validate()
	if err != nil {
		return Evidence{}, UpdateOutcome{}, a.fail(op, err)
	}
	type field struct{ name, value string }
	var fields []field
	for _, c := range []struct {
		name  string
		ptr   *string
		clean func(string) (string, error)
	}{
		{"title", p.Title, cleanTitle},
		{"summary", p.Summary, cleanSummary},
		{"raw_detail", p.RawDetail, cleanRawDetail},
		{"source_ref", p.SourceRef, cleanSourceRef},
		{"source_label", p.SourceLabel, cleanSourceLabel},
		{"captured_at", p.CapturedAt, cleanCapturedAt},
		{"metadata", p.Metadata, func(s string) (string, error) { return domain.NormalizeMetadata("metadata", s) }},
	} {
		if c.ptr == nil {
			continue
		}
		v, err := c.clean(*c.ptr)
		if err != nil {
			return Evidence{}, UpdateOutcome{}, a.fail(op, err)
		}
		fields = append(fields, field{c.name, v})
	}
	if p.SourceType != nil {
		if err := cleanSourceType(*p.SourceType); err != nil {
			return Evidence{}, UpdateOutcome{}, a.fail(op, err)
		}
		fields = append(fields, field{"source_type", string(*p.SourceType)})
	}
	if p.Verification != nil {
		if err := p.Verification.Validate("verification"); err != nil {
			return Evidence{}, UpdateOutcome{}, a.fail(op, err)
		}
		if err := checkVerifier(actor.Kind, *p.Verification); err != nil {
			return Evidence{}, UpdateOutcome{}, a.fail(op, err)
		}
		fields = append(fields, field{"verification", string(*p.Verification)})
	}

	var (
		out     Evidence
		outcome UpdateOutcome
	)
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		outcome = UpdateOutcome{} // a retried transaction starts clean
		now := a.timestamp()
		t, cur, err := a.getEvidence(ctx, tx, invRef, evidenceRef, true)
		if err != nil {
			return err
		}
		current := map[string]string{
			"title": cur.Title, "summary": cur.Summary, "raw_detail": cur.RawDetail,
			"source_ref": cur.SourceRef, "source_label": cur.SourceLabel, "captured_at": cur.CapturedAt,
			"metadata": cur.Metadata, "source_type": string(cur.SourceType),
			"verification": string(cur.Verification),
		}
		var (
			set     []store.Assignment
			changed []string
		)
		for _, f := range fields {
			v := f.value
			if f.name == "captured_at" && v == "" {
				v = cur.CreatedAt
			}
			if current[f.name] == v {
				continue
			}
			set = append(set, store.Assignment{Column: f.name, Value: v})
			changed = append(changed, changedField(f.name, v))
		}
		if len(set) == 0 {
			cur.RawDetail = ""
			out = cur
			return nil
		}
		// Every field but verification is what a check vouched for.
		substance := len(set) > 1 || set[0].Column != "verification"
		note := ""
		if resetsVerification(actor.Kind, cur.Verification, substance, p.Verification != nil) {
			set = append(set, store.Assignment{Column: "verification", Value: string(domain.VerificationAIGenerated)})
			changed = append(changed, "verification") // the reset note names the value
			note = resetNote(actor.Kind)
			outcome = UpdateOutcome{VerificationReset: true, PreviousVerification: cur.Verification}
		}
		ev, err := a.writeEvidenceChange(ctx, tx, actor, t, cur.ID, set, now,
			fmt.Sprintf("Updated evidence %s: %s%s", cur.Ref, strings.Join(changed, ", "), note))
		if err != nil {
			return err
		}
		out = ev
		return nil
	})
	if err != nil {
		return Evidence{}, UpdateOutcome{}, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", out.InvestigationRef, "ref", out.Ref)
	return out, outcome, nil
}

// SetEvidenceVerification sets the verification state and records
// evidence.update ("Set verification of E-001 to disputed"). Setting the
// current value is a no-op with no activity.
func (a *App) SetEvidenceVerification(ctx context.Context, actor Actor, invRef, evidenceRef string, v Verification) (Evidence, error) {
	const op = "evidence.update"
	actor, err := actor.Validate()
	if err != nil {
		return Evidence{}, a.fail(op, err)
	}
	if err := v.Validate("verification"); err != nil {
		return Evidence{}, a.fail(op, err)
	}
	if err := checkVerifier(actor.Kind, v); err != nil {
		return Evidence{}, a.fail(op, err)
	}
	var out Evidence
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		now := a.timestamp()
		t, cur, err := a.getEvidence(ctx, tx, invRef, evidenceRef, false)
		if err != nil {
			return err
		}
		if cur.Verification == v {
			out = cur
			return nil
		}
		ev, err := a.writeEvidenceChange(ctx, tx, actor, t, cur.ID,
			[]store.Assignment{{Column: "verification", Value: string(v)}}, now,
			fmt.Sprintf("Set verification of %s to %s", cur.Ref, v))
		if err != nil {
			return err
		}
		out = ev
		return nil
	})
	if err != nil {
		return Evidence{}, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", out.InvestigationRef, "ref", out.Ref)
	return out, nil
}

// writeEvidenceChange applies set to evidence id (named by t), records one
// evidence.update row with summary, touches the investigation and reads the
// evidence back without raw_detail.
func (a *App) writeEvidenceChange(ctx context.Context, tx *sql.Tx, actor domain.Actor, t recordTarget, id int64,
	set []store.Assignment, now, summary string) (domain.Evidence, error) {
	if err := store.UpdateEvidence(ctx, tx, id, set, now); err != nil {
		return domain.Evidence{}, err
	}
	if err := a.recordActivity(ctx, tx, actor, t.Inv.ID, "evidence.update", t.Ref, summary, "", now); err != nil {
		return domain.Evidence{}, err
	}
	if err := store.TouchInvestigation(ctx, tx, t.Inv.ID, now); err != nil {
		return domain.Evidence{}, err
	}
	return store.GetEvidence(ctx, tx, t.Inv.ID, t.Num, false)
}

// changedField names one changed evidence field for the update activity
// summary: enum fields carry their new value ("verification source_verified",
// "source_type log"), free-text fields only their name.
func changedField(name, value string) string {
	switch name {
	case "verification", "source_type":
		return name + " " + value
	}
	return name
}
