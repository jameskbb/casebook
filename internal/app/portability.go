package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// Portability (D-23, DATA-01, DATA-02): one investigation moves between
// machines as versioned JSON (format casebook.investigation, version 1).
// Export is a read and records no activity. Import treats its input as fully
// untrusted: size cap before decoding, strict decoding, every field sanitized
// and validated with the same helpers and limits the app methods use, refs
// and links checked, and everything inserted in one transaction.

// Import limits for activity rows.
const (
	maxImportActionBytes    = 64
	maxImportTargetRefBytes = 64
)

// importAction is the shape of an activity action: a verb or a dotted
// noun.verb such as evidence.add.
var importAction = regexp.MustCompile(`^[a-z]+(\.[a-z]+)?$`)

// ExportInvestigation returns one investigation (empty invRef means the
// active one) as indented JSON in the casebook.investigation version 1
// format: the investigation, every evidence row including raw detail and
// metadata, claims, hypotheses, questions, claim and hypothesis links and
// the investigation's activity rows, all addressed by refs. Records are
// ordered by ref number and activity in the order it was written. It writes
// no activity row. The output ends with a newline and does not HTML-escape.
//
// The output is stable so it can be committed to a repository (ADR-0016):
// exporting an unchanged investigation twice gives identical bytes.
// exported_at is the newest timestamp in the document (the latest change it
// contains), not the wall clock, and the machine-local active.* activity
// rows (which investigation was active where) are left out.
func (a *App) ExportInvestigation(ctx context.Context, invRef string) ([]byte, error) {
	const op = "export"
	var doc domain.ExportDocument
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		inv, err := a.resolveInvestigation(ctx, tx, invRef)
		if err != nil {
			return err
		}
		d, err := loadExport(ctx, tx, inv.ID)
		if err != nil {
			return err
		}
		d.Investigation.ConclusionNeedsReview = inv.ConclusionNeedsReview
		doc = d
		return nil
	})
	if err != nil {
		return nil, a.fail(op, err)
	}
	doc.Format = domain.ExportFormat
	doc.Version = domain.ExportVersion
	doc.ExportedAt = newestTimestamp(doc)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", doc.Investigation.Ref, "bytes", buf.Len())
	return buf.Bytes(), nil
}

// loadExport reads every exported row of investigation invID.
func loadExport(ctx context.Context, q store.Querier, invID int64) (domain.ExportDocument, error) {
	var (
		d   domain.ExportDocument
		err error
	)
	if d.Investigation, err = store.ExportInvestigationRow(ctx, q, invID); err != nil {
		return d, err
	}
	if d.Evidence, err = store.ExportEvidenceRows(ctx, q, invID); err != nil {
		return d, err
	}
	claims, err := store.ExportStatementRows(ctx, q, store.LinkClaim, invID)
	if err != nil {
		return d, err
	}
	d.Claims = make([]domain.ExportClaim, 0, len(claims))
	for _, c := range claims {
		d.Claims = append(d.Claims, domain.ExportClaim{
			Ref: c.Ref, Statement: c.Statement, Confidence: c.Confidence, ConfidenceRationale: c.Rationale,
			Status: domain.ClaimStatus(c.Status), Verification: c.Verification, SupersededBy: c.SupersededBy,
			CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, CreatedBy: c.CreatedBy,
		})
	}
	hyps, err := store.ExportStatementRows(ctx, q, store.LinkHypothesis, invID)
	if err != nil {
		return d, err
	}
	d.Hypotheses = make([]domain.ExportHypothesis, 0, len(hyps))
	for _, h := range hyps {
		d.Hypotheses = append(d.Hypotheses, domain.ExportHypothesis{
			Ref: h.Ref, Statement: h.Statement, Confidence: h.Confidence, Rationale: h.Rationale,
			Status: domain.HypothesisStatus(h.Status), Verification: h.Verification, SupersededBy: h.SupersededBy,
			CreatedAt: h.CreatedAt, UpdatedAt: h.UpdatedAt, CreatedBy: h.CreatedBy,
		})
	}
	if d.Questions, err = store.ExportQuestionRows(ctx, q, invID); err != nil {
		return d, err
	}
	cl, err := store.ExportLinkRows(ctx, q, store.LinkClaim, invID)
	if err != nil {
		return d, err
	}
	d.ClaimLinks = make([]domain.ExportClaimLink, 0, len(cl))
	for _, l := range cl {
		d.ClaimLinks = append(d.ClaimLinks, domain.ExportClaimLink{
			ClaimRef: l.RecordRef, EvidenceRef: l.EvidenceRef, Relation: l.Relation,
			CreatedAt: l.CreatedAt, CreatedBy: l.CreatedBy,
		})
	}
	hl, err := store.ExportLinkRows(ctx, q, store.LinkHypothesis, invID)
	if err != nil {
		return d, err
	}
	d.HypothesisLinks = make([]domain.ExportHypothesisLink, 0, len(hl))
	for _, l := range hl {
		d.HypothesisLinks = append(d.HypothesisLinks, domain.ExportHypothesisLink{
			HypothesisRef: l.RecordRef, EvidenceRef: l.EvidenceRef, Relation: l.Relation,
			CreatedAt: l.CreatedAt, CreatedBy: l.CreatedBy,
		})
	}
	rows, err := store.ExportActivityRows(ctx, q, invID)
	if err != nil {
		return d, err
	}
	d.Activity = make([]domain.ExportActivity, 0, len(rows))
	for _, r := range rows {
		if !strings.HasPrefix(r.Action, "active.") {
			d.Activity = append(d.Activity, r)
		}
	}
	return d, nil
}

// newestTimestamp is the latest timestamp anywhere in d: the export's "as
// of" time, which only moves when the investigation does. Timestamps share
// TimeLayout, so the string order is the time order.
func newestTimestamp(d domain.ExportDocument) string {
	newest := ""
	see := func(ts ...string) {
		for _, t := range ts {
			if t > newest {
				newest = t
			}
		}
	}
	inv := d.Investigation
	see(inv.CreatedAt, inv.UpdatedAt, inv.ArchivedAt)
	for _, e := range d.Evidence {
		see(e.CreatedAt, e.UpdatedAt)
	}
	for _, c := range d.Claims {
		see(c.CreatedAt, c.UpdatedAt)
	}
	for _, h := range d.Hypotheses {
		see(h.CreatedAt, h.UpdatedAt)
	}
	for _, q := range d.Questions {
		see(q.CreatedAt, q.UpdatedAt, q.ResolvedAt)
	}
	for _, l := range d.ClaimLinks {
		see(l.CreatedAt)
	}
	for _, l := range d.HypothesisLinks {
		see(l.CreatedAt)
	}
	for _, act := range d.Activity {
		see(act.CreatedAt)
	}
	return newest
}

// ImportInvestigation imports one investigation from casebook.investigation
// version 1 JSON (D-23). The input is untrusted:
//
//   - more than MaxImportBytes is a ValidationError on data before decoding;
//   - malformed JSON, unknown fields and trailing data are a ValidationError
//     on data naming the byte offset (never echoing input text);
//   - a format other than casebook.investigation is a ValidationError on
//     format, any version other than 1 a ValidationError on version;
//   - every field is sanitized and validated with the app-layer rules, and a
//     failure is a ValidationError on an indexed path such as
//     evidence[2].summary or claim_links[3].evidence_ref;
//   - refs must have the right prefix and be unique per kind; links and
//     superseded_by must name refs present in the document; ref numbers are
//     at most MaxImportInvestigationRefNum (investigation) and
//     MaxImportRecordRefNum (records), so an import never exhausts refs.
//
// States the app cannot produce are refused like the app methods refuse
// them: a superseded claim or hypothesis needs superseded_by, a resolution
// needs status resolved and a defer note status deferred.
//
// Everything is inserted in one transaction or nothing is. Record refs are
// kept; the original INV ref is kept when free, otherwise the next INV ref is
// allocated (RefChanged). superseded_by and links are remapped; original
// authors and timestamps are preserved, so a re-export round trips. One
// import activity row is added with actor {import, <actor client>} and a
// summary naming the original ref. The active investigation is not changed.
//
// Importing a case that is already in the database (SameCaseRefs) still
// adds a separate copy: import never merges into or replaces an existing
// investigation (ADR-0016).
func (a *App) ImportInvestigation(ctx context.Context, actor Actor, data []byte) (ImportResult, error) {
	const op = "import"
	actor, err := actor.Validate()
	if err != nil {
		return ImportResult{}, a.fail(op, err)
	}
	if len(data) > domain.MaxImportBytes {
		return ImportResult{}, domain.NewValidationError("data", "must be at most %d bytes (20 MiB)", domain.MaxImportBytes)
	}
	doc, err := decodeExport(data)
	if err != nil {
		return ImportResult{}, a.fail(op, err)
	}
	p, err := cleanImport(doc)
	if err != nil {
		return ImportResult{}, a.fail(op, err)
	}

	var res ImportResult
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		r, err := a.insertImport(ctx, tx, actor, p)
		if err != nil {
			return err
		}
		res = r
		return nil
	})
	if err != nil {
		return ImportResult{}, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", res.InvestigationRef, "original_ref", res.OriginalRef,
		"ref_changed", res.RefChanged)
	return res, nil
}

// ImportInvestigationFrom is ImportInvestigation reading the document from
// r. It reads at most MaxImportBytes plus one byte, so an oversized or
// endless input is refused (ValidationError on data) without being held in
// memory; surfaces importing from a file or stdin use it instead of reading
// the whole input themselves. A read error is returned as is.
func (a *App) ImportInvestigationFrom(ctx context.Context, actor Actor, r io.Reader) (ImportResult, error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(domain.MaxImportBytes)+1))
	if err != nil {
		return ImportResult{}, fmt.Errorf("read import: %w", err)
	}
	return a.ImportInvestigation(ctx, actor, data)
}

// decodeExport decodes data strictly. The format and version are checked on
// a lenient first pass so a file from a newer Casebook gets a version error
// rather than an unknown-field error.
func decodeExport(data []byte) (domain.ExportDocument, error) {
	var head struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
	}
	hd := json.NewDecoder(bytes.NewReader(data))
	if err := hd.Decode(&head); err != nil {
		return domain.ExportDocument{}, jsonError(err, hd.InputOffset())
	}
	if head.Format != domain.ExportFormat {
		return domain.ExportDocument{}, domain.NewValidationError("format", "must be %q", domain.ExportFormat)
	}
	if head.Version != 1 && head.Version != domain.ExportVersion {
		return domain.ExportDocument{}, domain.NewValidationError("version",
			"unsupported export version %d; this Casebook reads versions 1 and %d", head.Version, domain.ExportVersion)
	}

	var doc domain.ExportDocument
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return domain.ExportDocument{}, jsonError(err, dec.InputOffset())
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return domain.ExportDocument{}, domain.NewValidationError("data",
			"has trailing data after the JSON document (byte offset %d)", dec.InputOffset())
	}
	return doc, nil
}

// jsonError turns a decoder error into a ValidationError on data that names
// a position but never echoes input text.
func jsonError(err error, offset int64) error {
	var (
		syn *json.SyntaxError
		typ *json.UnmarshalTypeError
	)
	switch {
	case errors.As(err, &syn):
		return domain.NewValidationError("data", "is not valid JSON (byte offset %d)", syn.Offset)
	case errors.As(err, &typ):
		field := typ.Field
		if field == "" {
			field = "the document"
		}
		return domain.NewValidationError("data", "has a value of the wrong JSON type for %s (byte offset %d)",
			field, typ.Offset)
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return domain.NewValidationError("data", "is not valid JSON: unexpected end of input")
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		return domain.NewValidationError("data", "has an unknown field (byte offset %d)", offset)
	}
	return domain.NewValidationError("data", "is not a valid Casebook export (byte offset %d)", offset)
}

// importStmt is a validated claim or hypothesis.
type importStmt struct {
	row               store.ImportedStatement
	supersededBy      int64 // replacement ref number, 0 when none
	resetVerification bool  // untrustedVerification lowered it
}

// importLink is a validated claim or hypothesis link.
type importLink struct {
	recordNum, evidenceNum int64
	relation               domain.Relation
	createdAt              string
	createdBy              domain.Actor
}

// importPlan is a fully validated import, ready to insert. Investigation ids
// are filled in by insertImport.
type importPlan struct {
	origNum    int64
	inv        store.ImportedInvestigation
	evidence   []store.ImportedEvidence
	claims     []importStmt
	hypotheses []importStmt
	questions  []store.ImportedQuestion
	claimLinks []importLink
	hypLinks   []importLink
	activity   []store.NewActivity
	// reset counts human_verified and source_verified states the import
	// lowered to ai_generated (see untrustedVerification).
	reset int
}

// insertImport writes a validated plan inside the caller's transaction. It
// is safe to re-run: it reads p and never modifies it.
func (a *App) insertImport(ctx context.Context, tx *sql.Tx, actor domain.Actor, p importPlan) (ImportResult, error) {
	now := a.timestamp()
	num := p.origNum
	taken, err := store.InvestigationNumTaken(ctx, tx, num)
	if err != nil {
		return ImportResult{}, err
	}
	if taken {
		if err := store.RaiseInvestigationCounter(ctx, tx, 0); err != nil {
			return ImportResult{}, err
		}
		if num, err = store.AllocateInvestigationNum(ctx, tx); err != nil {
			return ImportResult{}, err
		}
	} else if err := store.RaiseInvestigationCounter(ctx, tx, num); err != nil {
		return ImportResult{}, err
	}

	inv := p.inv
	inv.RefNum = num
	invID, err := store.InsertImportedInvestigation(ctx, tx, inv)
	if err != nil {
		return ImportResult{}, err
	}
	same, err := store.SameCaseInvestigations(ctx, tx, inv.CreatedAt, inv.CreatedBy, invID)
	if err != nil {
		return ImportResult{}, err
	}
	var sameRefs []string
	for _, n := range same {
		sameRefs = append(sameRefs, domain.FormatInvestigationRef(n))
	}

	evIDs := make(map[int64]int64, len(p.evidence))
	for _, ev := range p.evidence {
		e := ev
		e.InvestigationID = invID
		id, err := store.InsertImportedEvidence(ctx, tx, e)
		if err != nil {
			return ImportResult{}, err
		}
		evIDs[e.RefNum] = id
	}
	claimIDs, err := insertImportedStatements(ctx, tx, store.LinkClaim, invID, p.claims)
	if err != nil {
		return ImportResult{}, err
	}
	hypIDs, err := insertImportedStatements(ctx, tx, store.LinkHypothesis, invID, p.hypotheses)
	if err != nil {
		return ImportResult{}, err
	}
	for _, qu := range p.questions {
		q := qu
		q.InvestigationID = invID
		if _, err := store.InsertImportedQuestion(ctx, tx, q); err != nil {
			return ImportResult{}, err
		}
	}
	if err := setImportedSupersede(ctx, tx, store.LinkClaim, p.claims, claimIDs); err != nil {
		return ImportResult{}, err
	}
	if err := setImportedSupersede(ctx, tx, store.LinkHypothesis, p.hypotheses, hypIDs); err != nil {
		return ImportResult{}, err
	}
	for _, l := range p.claimLinks {
		if err := store.InsertImportedLink(ctx, tx, store.LinkClaim, claimIDs[l.recordNum], evIDs[l.evidenceNum],
			l.relation, l.createdAt, l.createdBy); err != nil {
			return ImportResult{}, err
		}
	}
	for _, l := range p.hypLinks {
		if err := store.InsertImportedLink(ctx, tx, store.LinkHypothesis, hypIDs[l.recordNum], evIDs[l.evidenceNum],
			l.relation, l.createdAt, l.createdBy); err != nil {
			return ImportResult{}, err
		}
	}
	for _, act := range p.activity {
		row := act
		row.InvestigationID = invID
		if _, err := store.InsertActivity(ctx, tx, row); err != nil {
			return ImportResult{}, err
		}
	}

	res := ImportResult{
		InvestigationRef: domain.FormatInvestigationRef(num),
		Title:            p.inv.Title,
		OriginalRef:      domain.FormatInvestigationRef(p.origNum),
		RefChanged:       num != p.origNum,
		SameCaseRefs:     sameRefs,
		Counts: domain.Counts{
			Investigations:  1,
			Evidence:        len(p.evidence),
			Claims:          len(p.claims),
			Hypotheses:      len(p.hypotheses),
			Questions:       len(p.questions),
			ClaimLinks:      len(p.claimLinks),
			HypothesisLinks: len(p.hypLinks),
			Activity:        len(p.activity) + 1,
		},
	}
	as := res.OriginalRef
	if res.RefChanged {
		as += " as " + res.InvestigationRef
	}
	summary := fmt.Sprintf("Imported %s (%d evidence, %d claims, %d hypotheses, %d questions)",
		as, len(p.evidence), len(p.claims), len(p.hypotheses), len(p.questions))
	if p.reset > 0 {
		summary += fmt.Sprintf("; %s reset to ai_generated", plural(p.reset, "verification"))
	}
	importer := domain.Actor{Kind: domain.ActorImport, Client: actor.Client}
	if err := a.recordActivity(ctx, tx, importer, invID, "import", res.InvestigationRef, summary, "", now); err != nil {
		return ImportResult{}, err
	}
	return res, nil
}

// insertImportedStatements inserts claims or hypotheses (superseded_by left
// NULL) and returns ref number to id.
func insertImportedStatements(ctx context.Context, tx *sql.Tx, t store.LinkTarget, invID int64, rows []importStmt) (map[int64]int64, error) {
	ids := make(map[int64]int64, len(rows))
	for _, s := range rows {
		r := s.row
		r.InvestigationID = invID
		id, err := store.InsertImportedStatement(ctx, tx, t, r)
		if err != nil {
			return nil, err
		}
		ids[r.RefNum] = id
	}
	return ids, nil
}

// setImportedSupersede sets superseded_by_id once every row has an id.
func setImportedSupersede(ctx context.Context, tx *sql.Tx, t store.LinkTarget, rows []importStmt, ids map[int64]int64) error {
	for _, s := range rows {
		if s.supersededBy == 0 {
			continue
		}
		if err := store.SetImportedSupersededBy(ctx, tx, t, ids[s.row.RefNum], ids[s.supersededBy]); err != nil {
			return err
		}
	}
	return nil
}

// untrustedVerification lowers a verification an import cannot vouch for:
// human_verified and source_verified in a file are claims by whoever wrote
// the file, so an imported record starts as ai_generated (unverified here)
// until someone verifies it in this database (AR-09). Other states pass.
func untrustedVerification(v domain.Verification) (domain.Verification, bool) {
	switch v {
	case domain.VerificationHumanVerified, domain.VerificationSourceVerified:
		return domain.VerificationAIGenerated, true
	}
	return v, false
}

// cleanImport validates and sanitizes every field of doc. All validation
// happens here, before any transaction.
func cleanImport(doc domain.ExportDocument) (importPlan, error) {
	var (
		p   importPlan
		err error
	)
	if p.origNum, p.inv, err = cleanImportInvestigation(doc.Investigation); err != nil {
		return p, err
	}

	if v, lowered := untrustedVerification(p.inv.ConclusionVerification); lowered {
		p.inv.ConclusionVerification = v
		p.reset++
	}
	evNums := map[int64]int{}
	for i, e := range doc.Evidence {
		ev, err := cleanImportEvidence(fmt.Sprintf("evidence[%d]", i), e)
		if err != nil {
			return p, err
		}
		if v, lowered := untrustedVerification(ev.Verification); lowered {
			ev.Verification = v
			p.reset++
		}
		if j, dup := evNums[ev.RefNum]; dup {
			return p, domain.NewValidationError(fmt.Sprintf("evidence[%d].ref", i), "duplicates evidence[%d].ref", j)
		}
		evNums[ev.RefNum] = i
		p.evidence = append(p.evidence, ev)
		p.inv.LastEvidenceNum = max(p.inv.LastEvidenceNum, ev.RefNum)
	}

	claims := make([]rawImportStmt, len(doc.Claims))
	for i, c := range doc.Claims {
		claims[i] = rawImportStmt{c.Ref, c.Statement, c.Confidence, c.ConfidenceRationale, string(c.Status),
			c.Verification, c.SupersededBy, c.CreatedAt, c.UpdatedAt, c.CreatedBy}
	}
	claimNums := map[int64]int{}
	if p.claims, err = cleanImportStatements("claims", claimKind, claims, claimNums); err != nil {
		return p, err
	}
	for _, c := range p.claims {
		p.inv.LastClaimNum = max(p.inv.LastClaimNum, c.row.RefNum)
		if c.resetVerification {
			p.reset++
		}
	}

	for _, refs := range [][]string{p.inv.ConclusionSupportingClaims, p.inv.ConclusionContradictingClaims} {
		for _, ref := range refs {
			rr, _ := domain.ParseRecordRef(ref, domain.RecordClaim)
			if _, ok := claimNums[rr.Num]; !ok {
				return p, domain.NewValidationError("investigation.conclusion_supporting_claims", "claim %s is not present in the import", ref)
			}
		}
	}
	hyps := make([]rawImportStmt, len(doc.Hypotheses))
	for i, h := range doc.Hypotheses {
		hyps[i] = rawImportStmt{h.Ref, h.Statement, h.Confidence, h.Rationale, string(h.Status),
			h.Verification, h.SupersededBy, h.CreatedAt, h.UpdatedAt, h.CreatedBy}
	}
	hypNums := map[int64]int{}
	if p.hypotheses, err = cleanImportStatements("hypotheses", hypothesisKind, hyps, hypNums); err != nil {
		return p, err
	}
	for _, h := range p.hypotheses {
		p.inv.LastHypothesisNum = max(p.inv.LastHypothesisNum, h.row.RefNum)
		if h.resetVerification {
			p.reset++
		}
	}

	qNums := map[int64]int{}
	for i, q := range doc.Questions {
		qu, err := cleanImportQuestion(fmt.Sprintf("questions[%d]", i), q)
		if err != nil {
			return p, err
		}
		if j, dup := qNums[qu.RefNum]; dup {
			return p, domain.NewValidationError(fmt.Sprintf("questions[%d].ref", i), "duplicates questions[%d].ref", j)
		}
		qNums[qu.RefNum] = i
		p.questions = append(p.questions, qu)
		p.inv.LastQuestionNum = max(p.inv.LastQuestionNum, qu.RefNum)
	}

	cl := make([]rawImportLink, len(doc.ClaimLinks))
	for i, l := range doc.ClaimLinks {
		cl[i] = rawImportLink{l.ClaimRef, l.EvidenceRef, l.Relation, l.CreatedAt, l.CreatedBy}
	}
	if p.claimLinks, err = cleanImportLinks("claim_links", "claim_ref", domain.RecordClaim, cl, claimNums, evNums); err != nil {
		return p, err
	}
	hl := make([]rawImportLink, len(doc.HypothesisLinks))
	for i, l := range doc.HypothesisLinks {
		hl[i] = rawImportLink{l.HypothesisRef, l.EvidenceRef, l.Relation, l.CreatedAt, l.CreatedBy}
	}
	if p.hypLinks, err = cleanImportLinks("hypothesis_links", "hypothesis_ref", domain.RecordHypothesis, hl, hypNums, evNums); err != nil {
		return p, err
	}

	for i, act := range doc.Activity {
		row, err := cleanImportActivity(fmt.Sprintf("activity[%d]", i), act)
		if err != nil {
			return p, err
		}
		row.Imported = true
		p.activity = append(p.activity, row)
	}
	return p, nil
}

// cleanImportInvestigation validates the investigation section.
func cleanImportInvestigation(in domain.ExportInvestigation) (int64, store.ImportedInvestigation, error) {
	const p = "investigation"
	var (
		out store.ImportedInvestigation
		err error
	)
	num, err := domain.ParseInvestigationRef(in.Ref)
	if err != nil {
		return 0, out, refieldValidation(err, p+".ref")
	}
	if num > domain.MaxImportInvestigationRefNum {
		return 0, out, domain.NewValidationError(p+".ref", "ref number must be at most %d (%s)",
			domain.MaxImportInvestigationRefNum, domain.FormatInvestigationRef(domain.MaxImportInvestigationRefNum))
	}
	if out.Title, err = domain.SanitizeLine(p+".title", in.Title, domain.MaxTitleBytes, true); err != nil {
		return 0, out, err
	}
	if out.ProblemStatement, err = domain.SanitizeText(p+".problem_statement", in.ProblemStatement,
		domain.MaxProblemStatementBytes, false); err != nil {
		return 0, out, err
	}
	if err = in.Status.Validate(p + ".status"); err != nil {
		return 0, out, err
	}
	out.Status = in.Status
	if in.PreArchiveStatus != "" {
		if err = in.PreArchiveStatus.Validate(p + ".pre_archive_status"); err != nil {
			return 0, out, err
		}
		if in.PreArchiveStatus == domain.InvestigationStatusArchived {
			return 0, out, domain.NewValidationError(p+".pre_archive_status", "must not be archived")
		}
		if in.Status != domain.InvestigationStatusArchived {
			return 0, out, domain.NewValidationError(p+".pre_archive_status", "is only allowed when status is archived")
		}
	}
	out.PreArchiveStatus = in.PreArchiveStatus
	if out.Conclusion, err = domain.SanitizeText(p+".conclusion", in.Conclusion, domain.MaxConclusionBytes, false); err != nil {
		return 0, out, err
	}
	if out.ConclusionConfidence, err = cleanConfidence(in.ConclusionConfidence); err != nil {
		return 0, out, refieldValidation(err, p+".conclusion_confidence")
	}
	if out.ConclusionRationale, err = domain.SanitizeText(p+".conclusion_rationale", in.ConclusionRationale,
		domain.MaxConclusionRationaleBytes, false); err != nil {
		return 0, out, err
	}
	out.ConclusionVerification = in.ConclusionVerification
	if out.ConclusionVerification == "" {
		out.ConclusionVerification = domain.VerificationAIGenerated
	}
	if err := out.ConclusionVerification.Validate(p + ".conclusion_verification"); err != nil {
		return 0, out, err
	}
	if out.ConclusionVerification == domain.VerificationSuperseded {
		return 0, out, domain.NewValidationError(p+".conclusion_verification", "superseded is not a conclusion verification")
	}
	importInv := domain.Investigation{Ref: in.Ref}
	if out.ConclusionSupportingClaims, err = canonicalConclusionRefs(importInv, p+".conclusion_supporting_claims", in.ConclusionSupportingClaims); err != nil {
		return 0, out, err
	}
	if out.ConclusionContradictingClaims, err = canonicalConclusionRefs(importInv, p+".conclusion_contradicting_claims", in.ConclusionContradictingClaims); err != nil {
		return 0, out, err
	}
	for _, s := range out.ConclusionSupportingClaims {
		for _, c := range out.ConclusionContradictingClaims {
			if s == c {
				return 0, out, domain.NewValidationError(p+".conclusion_contradicting_claims", "%s cannot both support and contradict the conclusion", s)
			}
		}
	}
	if out.Conclusion == "" && (len(out.ConclusionSupportingClaims) > 0 || len(out.ConclusionContradictingClaims) > 0) {
		return 0, out, domain.NewValidationError(p+".conclusion", "an empty conclusion cannot select claims")
	}
	// Review timestamps and basis are untrusted file assertions. No imported
	// conclusion retains a trusted review or a material-basis fingerprint.
	if out.CreatedAt, err = importTime(p+".created_at", in.CreatedAt, true); err != nil {
		return 0, out, err
	}
	if out.UpdatedAt, err = importTime(p+".updated_at", in.UpdatedAt, true); err != nil {
		return 0, out, err
	}
	if out.ArchivedAt, err = importTime(p+".archived_at", in.ArchivedAt, false); err != nil {
		return 0, out, err
	}
	if (out.Status == domain.InvestigationStatusArchived) != (out.ArchivedAt != "") {
		return 0, out, domain.NewValidationError(p+".archived_at", "must be set exactly when status is archived")
	}
	if out.CreatedBy, err = importActor(p+".created_by", in.CreatedBy); err != nil {
		return 0, out, err
	}
	return num, out, nil
}

// cleanImportEvidence validates one evidence row; p is its path prefix.
func cleanImportEvidence(p string, in domain.ExportEvidence) (store.ImportedEvidence, error) {
	var (
		out store.ImportedEvidence
		err error
	)
	if out.RefNum, err = importRefNum(p+".ref", in.Ref, domain.RecordEvidence); err != nil {
		return out, err
	}
	if out.Title, err = domain.SanitizeLine(p+".title", in.Title, domain.MaxTitleBytes, true); err != nil {
		return out, err
	}
	if out.Summary, err = domain.SanitizeText(p+".summary", in.Summary, domain.MaxEvidenceSummaryBytes, true); err != nil {
		return out, err
	}
	if out.RawDetail, err = domain.SanitizeText(p+".raw_detail", in.RawDetail, domain.MaxRawDetailBytes, false); err != nil {
		return out, err
	}
	if in.SourceType == "" {
		return out, domain.NewValidationError(p+".source_type", "is required")
	}
	if err = in.SourceType.Validate(p + ".source_type"); err != nil {
		return out, err
	}
	out.SourceType = in.SourceType
	if out.SourceRef, err = domain.SanitizeLine(p+".source_ref", in.SourceRef, domain.MaxSourceRefBytes, false); err != nil {
		return out, err
	}
	if out.SourceLabel, err = domain.SanitizeLine(p+".source_label", in.SourceLabel, domain.MaxSourceLabelBytes, false); err != nil {
		return out, err
	}
	if out.CapturedAt, err = importTime(p+".captured_at", in.CapturedAt, true); err != nil {
		return out, err
	}
	if err = in.Verification.Validate(p + ".verification"); err != nil {
		return out, err
	}
	out.Verification = in.Verification
	if out.Metadata, err = importMetadata(p+".metadata", in.Metadata); err != nil {
		return out, err
	}
	if out.CreatedAt, err = importTime(p+".created_at", in.CreatedAt, true); err != nil {
		return out, err
	}
	if out.UpdatedAt, err = importTime(p+".updated_at", in.UpdatedAt, true); err != nil {
		return out, err
	}
	if out.CreatedBy, err = importActor(p+".created_by", in.CreatedBy); err != nil {
		return out, err
	}
	return out, nil
}

// importMetadata validates embedded metadata: absent or null means none;
// anything else must be a JSON object accepted by NormalizeMetadata.
func importMetadata(field string, raw json.RawMessage) (string, error) {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || string(t) == "null" {
		return "", nil
	}
	// Compact first so indentation in the file never counts against the
	// metadata input cap; the decoder has already checked it is valid JSON.
	var buf bytes.Buffer
	if err := json.Compact(&buf, t); err != nil {
		return "", domain.NewValidationError(field, "must be a JSON object")
	}
	return domain.NormalizeMetadata(field, buf.String())
}

// rawImportStmt is the shared form of ExportClaim and ExportHypothesis.
type rawImportStmt struct {
	Ref          string
	Statement    string
	Confidence   domain.Confidence
	Rationale    string
	Status       string
	Verification domain.Verification
	SupersededBy string
	CreatedAt    string
	UpdatedAt    string
	CreatedBy    domain.Actor
}

// cleanImportStatements validates claims or hypotheses (name is the array
// name) and fills nums with ref number to index.
func cleanImportStatements(name string, k stmtKind, in []rawImportStmt, nums map[int64]int) ([]importStmt, error) {
	out := make([]importStmt, 0, len(in))
	for i, s := range in {
		p := fmt.Sprintf("%s[%d]", name, i)
		var (
			st  importStmt
			err error
		)
		r := &st.row
		if r.RefNum, err = importRefNum(p+".ref", s.Ref, k.rec); err != nil {
			return nil, err
		}
		if j, dup := nums[r.RefNum]; dup {
			return nil, domain.NewValidationError(p+".ref", "duplicates %s[%d].ref", name, j)
		}
		nums[r.RefNum] = i
		if r.Statement, err = domain.SanitizeText(p+".statement", s.Statement, domain.MaxStatementBytes, true); err != nil {
			return nil, err
		}
		if r.Confidence, err = cleanConfidence(s.Confidence); err != nil {
			return nil, refieldValidation(err, p+".confidence")
		}
		if r.Rationale, err = domain.SanitizeText(p+"."+k.rationaleField, s.Rationale, domain.MaxRationaleBytes, false); err != nil {
			return nil, err
		}
		if err = k.validStatus(p+".status", s.Status); err != nil {
			return nil, err
		}
		r.Status = s.Status
		if err = s.Verification.Validate(p + ".verification"); err != nil {
			return nil, err
		}
		r.Status, r.Verification = k.repairPair(r.Status, s.Verification, s.CreatedBy.Kind)
		if v, lowered := untrustedVerification(r.Verification); lowered {
			r.Verification = v
			st.resetVerification = true
		}
		if strings.TrimSpace(s.SupersededBy) != "" {
			if st.supersededBy, err = importRefNum(p+".superseded_by", s.SupersededBy, k.rec); err != nil {
				return nil, err
			}
			if st.supersededBy == r.RefNum {
				return nil, domain.NewValidationError(p+".superseded_by", "must name another %s, not itself", k.rec)
			}
			if r.Status != k.superseded {
				return nil, domain.NewValidationError(p+".superseded_by", "is only allowed when status is %s", k.superseded)
			}
		} else if r.Status == k.superseded {
			// The app never produces a superseded record without its
			// replacement (UpdateClaim and UpdateHypothesis require one).
			return nil, domain.NewValidationError(p+".superseded_by", "is required when status is %s", k.superseded)
		}
		if r.CreatedAt, err = importTime(p+".created_at", s.CreatedAt, true); err != nil {
			return nil, err
		}
		if r.UpdatedAt, err = importTime(p+".updated_at", s.UpdatedAt, true); err != nil {
			return nil, err
		}
		if r.CreatedBy, err = importActor(p+".created_by", s.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, st)
	}

	// superseded_by must name a record in this document and must not form a
	// cycle.
	next := make(map[int64]int64, len(out))
	for _, st := range out {
		next[st.row.RefNum] = st.supersededBy
	}
	for i, st := range out {
		if st.supersededBy == 0 {
			continue
		}
		p := fmt.Sprintf("%s[%d].superseded_by", name, i)
		if _, ok := nums[st.supersededBy]; !ok {
			return nil, domain.NewValidationError(p, "names %s, which is not in this export",
				domain.FormatRecordRef(k.rec, st.supersededBy))
		}
		cur := st.supersededBy
		for steps := 0; cur != 0 && steps <= len(out); steps++ {
			if cur == st.row.RefNum {
				return nil, domain.NewValidationError(p, "forms a supersede cycle")
			}
			cur = next[cur]
		}
	}
	return out, nil
}

// cleanImportQuestion validates one question; p is its path prefix.
func cleanImportQuestion(p string, in domain.ExportQuestion) (store.ImportedQuestion, error) {
	var (
		out store.ImportedQuestion
		err error
	)
	if out.RefNum, err = importRefNum(p+".ref", in.Ref, domain.RecordQuestion); err != nil {
		return out, err
	}
	if out.Question, err = domain.SanitizeText(p+".question", in.Question, domain.MaxQuestionBytes, true); err != nil {
		return out, err
	}
	if err = in.Status.Validate(p + ".status"); err != nil {
		return out, err
	}
	out.Status = in.Status
	if out.Resolution, err = domain.SanitizeText(p+".resolution", in.Resolution, domain.MaxResolutionBytes, false); err != nil {
		return out, err
	}
	if out.DeferNote, err = domain.SanitizeText(p+".defer_note", in.DeferNote, domain.MaxDeferNoteBytes, false); err != nil {
		return out, err
	}
	if out.ResolvedAt, err = importTime(p+".resolved_at", in.ResolvedAt, false); err != nil {
		return out, err
	}
	resolved := out.Status == domain.QuestionStatusResolved
	if resolved != (out.ResolvedAt != "") {
		return out, domain.NewValidationError(p+".resolved_at", "must be set exactly when status is resolved")
	}
	if resolved && out.Resolution == "" {
		return out, domain.NewValidationError(p+".resolution", "is required when status is resolved")
	}
	// The app's transitions always clear these, so an imported question
	// never holds a state ReopenQuestion or DeferQuestion was not written for.
	if !resolved && out.Resolution != "" {
		return out, domain.NewValidationError(p+".resolution", "is only allowed when status is resolved")
	}
	if out.Status != domain.QuestionStatusDeferred && out.DeferNote != "" {
		return out, domain.NewValidationError(p+".defer_note", "is only allowed when status is deferred")
	}
	if out.CreatedAt, err = importTime(p+".created_at", in.CreatedAt, true); err != nil {
		return out, err
	}
	if out.UpdatedAt, err = importTime(p+".updated_at", in.UpdatedAt, true); err != nil {
		return out, err
	}
	if out.CreatedBy, err = importActor(p+".created_by", in.CreatedBy); err != nil {
		return out, err
	}
	return out, nil
}

// rawImportLink is the shared form of ExportClaimLink and
// ExportHypothesisLink.
type rawImportLink struct {
	RecordRef   string
	EvidenceRef string
	Relation    domain.Relation
	CreatedAt   string
	CreatedBy   domain.Actor
}

// cleanImportLinks validates claim or hypothesis links: both refs must name
// records in the document and each pair may appear once.
func cleanImportLinks(name, refField string, rec domain.RecordType, in []rawImportLink,
	recNums, evNums map[int64]int) ([]importLink, error) {
	out := make([]importLink, 0, len(in))
	seen := map[[2]int64]int{}
	for i, l := range in {
		p := fmt.Sprintf("%s[%d]", name, i)
		var (
			il  importLink
			err error
		)
		if il.recordNum, err = importRefNum(p+"."+refField, l.RecordRef, rec); err != nil {
			return nil, err
		}
		if _, ok := recNums[il.recordNum]; !ok {
			return nil, domain.NewValidationError(p+"."+refField, "names %s, which is not in this export",
				domain.FormatRecordRef(rec, il.recordNum))
		}
		if il.evidenceNum, err = importRefNum(p+".evidence_ref", l.EvidenceRef, domain.RecordEvidence); err != nil {
			return nil, err
		}
		if _, ok := evNums[il.evidenceNum]; !ok {
			return nil, domain.NewValidationError(p+".evidence_ref", "names %s, which is not in this export",
				domain.FormatRecordRef(domain.RecordEvidence, il.evidenceNum))
		}
		key := [2]int64{il.recordNum, il.evidenceNum}
		if j, dup := seen[key]; dup {
			return nil, domain.NewValidationError(p+".evidence_ref", "duplicates the link at %s[%d]", name, j)
		}
		seen[key] = i
		if err = l.Relation.Validate(p + ".relation"); err != nil {
			return nil, err
		}
		il.relation = l.Relation
		if il.createdAt, err = importTime(p+".created_at", l.CreatedAt, true); err != nil {
			return nil, err
		}
		if il.createdBy, err = importActor(p+".created_by", l.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, il)
	}
	return out, nil
}

// cleanImportActivity validates one historical activity row.
func cleanImportActivity(p string, in domain.ExportActivity) (store.NewActivity, error) {
	var (
		out store.NewActivity
		err error
	)
	if out.Actor, err = importActor(p+".actor", in.Actor); err != nil {
		return out, err
	}
	if len(in.Action) > maxImportActionBytes || !importAction.MatchString(in.Action) {
		return out, domain.NewValidationError(p+".action",
			"must be a lowercase verb or noun.verb of at most %d bytes", maxImportActionBytes)
	}
	out.Action = in.Action
	if out.TargetRef, err = domain.SanitizeLine(p+".target_ref", in.TargetRef, maxImportTargetRefBytes, false); err != nil {
		return out, err
	}
	if out.Summary, err = domain.SanitizeLine(p+".summary", in.Summary, domain.MaxActivitySummaryBytes, false); err != nil {
		return out, err
	}
	if out.Note, err = domain.SanitizeText(p+".note", in.Note, domain.MaxCheckpointNoteBytes, false); err != nil {
		return out, err
	}
	if out.CreatedAt, err = importTime(p+".created_at", in.CreatedAt, true); err != nil {
		return out, err
	}
	return out, nil
}

// importRefNum parses an unqualified record ref of type t, reporting
// failures on field.
func importRefNum(field, s string, t domain.RecordType) (int64, error) {
	rr, err := domain.ParseRecordRef(s, t)
	if err != nil {
		return 0, refieldValidation(err, field)
	}
	if rr.InvestigationNum != 0 {
		return 0, domain.NewValidationError(field, "must be an unqualified %s ref such as %s",
			t, domain.FormatRecordRef(t, 1))
	}
	if rr.Num > domain.MaxImportRecordRefNum {
		return 0, domain.NewValidationError(field, "ref number must be at most %d (%s)",
			domain.MaxImportRecordRefNum, domain.FormatRecordRef(t, domain.MaxImportRecordRefNum))
	}
	return rr.Num, nil
}

// importTime parses an RFC3339 timestamp on field; blank is "" unless
// required.
func importTime(field, s string, required bool) (string, error) {
	if strings.TrimSpace(s) == "" {
		if required {
			return "", domain.NewValidationError(field, "is required")
		}
		return "", nil
	}
	return domain.ParseTimestamp(field, s)
}

// importActor validates an actor, reporting failures on prefix.kind or
// prefix.client.
func importActor(prefix string, a domain.Actor) (domain.Actor, error) {
	v, err := a.Validate()
	if err != nil {
		if ve, ok := domain.IsValidation(err); ok {
			return domain.Actor{}, &domain.ValidationError{
				Field:   prefix + strings.TrimPrefix(ve.Field, "actor"),
				Message: ve.Message,
			}
		}
		return domain.Actor{}, err
	}
	return v, nil
}
