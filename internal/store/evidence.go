package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// NewEvidence is the data needed to insert an evidence row.
type NewEvidence struct {
	InvestigationID int64
	RefNum          int64
	Title           string
	Summary         string
	RawDetail       string
	SourceType      domain.SourceType
	SourceRef       string
	SourceLabel     string
	CapturedAt      string
	CreatedAt       string
	CreatedBy       domain.Actor
	Verification    domain.Verification
	Metadata        string
}

// InsertEvidence inserts an evidence row and returns its id.
func InsertEvidence(ctx context.Context, q Querier, e NewEvidence) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `INSERT INTO evidence
    (investigation_id, ref_num, title, summary, raw_detail, source_type, source_ref, source_label,
     captured_at, created_at, updated_at, created_by_kind, created_by_client, verification, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id`,
		e.InvestigationID, e.RefNum, e.Title, e.Summary, e.RawDetail, string(e.SourceType), e.SourceRef,
		e.SourceLabel, e.CapturedAt, e.CreatedAt, e.CreatedAt, string(e.CreatedBy.Kind), e.CreatedBy.Client,
		string(e.Verification), e.Metadata).Scan(&id)
	return id, err
}

// Assignment is one column = value pair for an update.
type Assignment struct {
	Column string
	Value  any
}

// evidenceUpdatable whitelists the evidence columns UpdateEvidence may set.
var evidenceUpdatable = map[string]bool{
	"title": true, "summary": true, "raw_detail": true, "source_type": true, "source_ref": true,
	"source_label": true, "captured_at": true, "verification": true, "metadata": true,
}

// UpdateEvidence sets the given columns and updated_at on evidence id. Only
// whitelisted columns are accepted, so the search index is rewritten only
// when title or summary is among them.
func UpdateEvidence(ctx context.Context, q Querier, id int64, set []Assignment, now string) error {
	parts := make([]string, 0, len(set)+1)
	args := make([]any, 0, len(set)+2)
	for _, a := range set {
		if !evidenceUpdatable[a.Column] {
			return fmt.Errorf("update evidence: column %q is not updatable", a.Column)
		}
		parts = append(parts, a.Column+" = ?")
		args = append(args, a.Value)
	}
	parts = append(parts, "updated_at = ?")
	args = append(args, now, id)
	res, err := q.ExecContext(ctx, `UPDATE evidence SET `+strings.Join(parts, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// evidenceColumns selects an evidence row. The first placeholder chooses
// whether raw_detail text is loaded; has_raw_detail never loads it.
const evidenceColumns = `e.id, e.ref_num, i.ref_num, e.title, e.summary,
    CASE WHEN ? THEN e.raw_detail ELSE '' END, length(e.raw_detail) > 0,
    e.source_type, e.source_ref, e.source_label, e.captured_at, e.created_at, e.updated_at,
    e.created_by_kind, e.created_by_client, e.verification, e.metadata, e.imported`

func scanEvidence(row interface{ Scan(...any) error }) (domain.Evidence, error) {
	var (
		e                  domain.Evidence
		refNum, invRefNum  int64
		srcType, kind, ver string
	)
	if err := row.Scan(&e.ID, &refNum, &invRefNum, &e.Title, &e.Summary, &e.RawDetail, &e.HasRawDetail,
		&srcType, &e.SourceRef, &e.SourceLabel, &e.CapturedAt, &e.CreatedAt, &e.UpdatedAt,
		&kind, &e.CreatedBy.Client, &ver, &e.Metadata, &e.Imported); err != nil {
		return domain.Evidence{}, err
	}
	e.Ref = domain.FormatRecordRef(domain.RecordEvidence, refNum)
	e.InvestigationRef = domain.FormatInvestigationRef(invRefNum)
	e.SourceType = domain.SourceType(srcType)
	e.CreatedBy.Kind = domain.ActorKind(kind)
	e.Verification = domain.Verification(ver)
	e.ClaimLinks = []domain.RecordLink{}
	e.HypothesisLinks = []domain.RecordLink{}
	return e, nil
}

// GetEvidence returns evidence refNum of investigation invID with its links,
// or an error matching sql.ErrNoRows. raw_detail is loaded only when withRaw.
func GetEvidence(ctx context.Context, q Querier, invID, refNum int64, withRaw bool) (domain.Evidence, error) {
	e, err := scanEvidence(q.QueryRowContext(ctx, `SELECT `+evidenceColumns+`
FROM evidence e JOIN investigations i ON i.id = e.investigation_id
WHERE e.investigation_id = ? AND e.ref_num = ?`, withRaw, invID, refNum))
	if err != nil {
		return domain.Evidence{}, err
	}
	claims, hyps, err := evidenceLinks(ctx, q, "e.id = ?", e.ID)
	if err != nil {
		return domain.Evidence{}, err
	}
	if l := claims[e.ID]; l != nil {
		e.ClaimLinks = l
	}
	if l := hyps[e.ID]; l != nil {
		e.HypothesisLinks = l
	}
	return e, nil
}

// ListEvidence returns every evidence row of investigation invID ordered by
// ref number, with links and without raw_detail text.
func ListEvidence(ctx context.Context, q Querier, invID int64) ([]domain.Evidence, error) {
	return ListEvidenceRaw(ctx, q, invID, false)
}

// ListEvidenceRaw is ListEvidence that also loads raw_detail text when
// withRaw is true.
func ListEvidenceRaw(ctx context.Context, q Querier, invID int64, withRaw bool) ([]domain.Evidence, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+evidenceColumns+`
FROM evidence e JOIN investigations i ON i.id = e.investigation_id
WHERE e.investigation_id = ? ORDER BY e.ref_num`, withRaw, invID)
	if err != nil {
		return nil, err
	}
	out := []domain.Evidence{}
	for rows.Next() {
		e, err := scanEvidence(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	claims, hyps, err := evidenceLinks(ctx, q, "e.investigation_id = ?", invID)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if l := claims[out[i].ID]; l != nil {
			out[i].ClaimLinks = l
		}
		if l := hyps[out[i].ID]; l != nil {
			out[i].HypothesisLinks = l
		}
	}
	return out, nil
}

// evidenceLinks returns the claim and hypothesis links of the evidence rows
// matching where (a fixed condition on alias e with one argument), keyed by
// evidence id and ordered by record ref number.
func evidenceLinks(ctx context.Context, q Querier, where string, arg int64) (claims, hyps map[int64][]domain.RecordLink, err error) {
	claims, err = linkQuery(ctx, q, domain.RecordClaim, `SELECT ce.evidence_id, c.ref_num, ce.relation
FROM claim_evidence ce
JOIN claims c ON c.id = ce.claim_id
JOIN evidence e ON e.id = ce.evidence_id
WHERE `+where+` ORDER BY ce.evidence_id, c.ref_num`, arg)
	if err != nil {
		return nil, nil, err
	}
	hyps, err = linkQuery(ctx, q, domain.RecordHypothesis, `SELECT he.evidence_id, h.ref_num, he.relation
FROM hypothesis_evidence he
JOIN hypotheses h ON h.id = he.hypothesis_id
JOIN evidence e ON e.id = he.evidence_id
WHERE `+where+` ORDER BY he.evidence_id, h.ref_num`, arg)
	if err != nil {
		return nil, nil, err
	}
	return claims, hyps, nil
}

// linkQuery runs a (key id, ref_num, relation) query and groups the rows by
// key id as RecordLinks of type t.
func linkQuery(ctx context.Context, q Querier, t domain.RecordType, query string, args ...any) (map[int64][]domain.RecordLink, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]domain.RecordLink{}
	for rows.Next() {
		var (
			key, num int64
			rel      string
		)
		if err := rows.Scan(&key, &num, &rel); err != nil {
			return nil, err
		}
		out[key] = append(out[key], domain.RecordLink{
			Ref:      domain.FormatRecordRef(t, num),
			Relation: domain.Relation(rel),
		})
	}
	return out, rows.Err()
}
