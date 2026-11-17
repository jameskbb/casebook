package store

import (
	"context"
	"fmt"

	"github.com/jameskbb/casebook/internal/domain"
)

// LinkTarget names the kind of record evidence links attach to. Table and
// column names are chosen from it by the switch in spec, never from input.
type LinkTarget int

// Link targets.
const (
	LinkClaim LinkTarget = iota + 1
	LinkHypothesis
)

// targetSpec holds the fixed SQL identifiers for one LinkTarget.
type targetSpec struct {
	table        string // record table
	linkTable    string // evidence link table
	idColumn     string // record id column in linkTable
	rationaleCol string // rationale column in table
	recordType   domain.RecordType
}

// spec returns the SQL identifiers for t. The names come only from this
// switch.
func (t LinkTarget) spec() (targetSpec, error) {
	switch t {
	case LinkClaim:
		return targetSpec{
			table: "claims", linkTable: "claim_evidence", idColumn: "claim_id",
			rationaleCol: "confidence_rationale", recordType: domain.RecordClaim,
		}, nil
	case LinkHypothesis:
		return targetSpec{
			table: "hypotheses", linkTable: "hypothesis_evidence", idColumn: "hypothesis_id",
			rationaleCol: "rationale", recordType: domain.RecordHypothesis,
		}, nil
	}
	return targetSpec{}, fmt.Errorf("store: unsupported link target %d", int(t))
}

// RecordType returns the record type of t (claim or hypothesis), or "" for
// an unknown target.
func (t LinkTarget) RecordType() domain.RecordType {
	s, err := t.spec()
	if err != nil {
		return ""
	}
	return s.recordType
}

// EvidenceID returns the id of evidence refNum in investigation invID, or an
// error matching sql.ErrNoRows.
func EvidenceID(ctx context.Context, q Querier, invID, refNum int64) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM evidence WHERE investigation_id = ? AND ref_num = ?`,
		invID, refNum).Scan(&id)
	return id, err
}

// GetLinkRelation returns the relation of the link between record recordID
// and evidence evidenceID, or an error matching sql.ErrNoRows.
func GetLinkRelation(ctx context.Context, q Querier, t LinkTarget, recordID, evidenceID int64) (domain.Relation, error) {
	s, err := t.spec()
	if err != nil {
		return "", err
	}
	var rel string
	err = q.QueryRowContext(ctx, `SELECT relation FROM `+s.linkTable+` WHERE `+s.idColumn+` = ? AND evidence_id = ?`,
		recordID, evidenceID).Scan(&rel)
	return domain.Relation(rel), err
}

// InsertLink links evidence evidenceID to record recordID.
func InsertLink(ctx context.Context, q Querier, t LinkTarget, recordID, evidenceID int64, rel domain.Relation,
	now string, by domain.Actor) error {
	s, err := t.spec()
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `INSERT INTO `+s.linkTable+`
    (`+s.idColumn+`, evidence_id, relation, created_at, created_by_kind, created_by_client)
VALUES (?, ?, ?, ?, ?, ?)`, recordID, evidenceID, string(rel), now, string(by.Kind), by.Client)
	return err
}

// UpdateLinkRelation changes the relation of an existing link.
func UpdateLinkRelation(ctx context.Context, q Querier, t LinkTarget, recordID, evidenceID int64, rel domain.Relation) error {
	s, err := t.spec()
	if err != nil {
		return err
	}
	res, err := q.ExecContext(ctx, `UPDATE `+s.linkTable+` SET relation = ? WHERE `+s.idColumn+` = ? AND evidence_id = ?`,
		string(rel), recordID, evidenceID)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// DeleteLink removes the link between record recordID and evidence
// evidenceID and reports whether one existed.
func DeleteLink(ctx context.Context, q Querier, t LinkTarget, recordID, evidenceID int64) (bool, error) {
	s, err := t.spec()
	if err != nil {
		return false, err
	}
	res, err := q.ExecContext(ctx, `DELETE FROM `+s.linkTable+` WHERE `+s.idColumn+` = ? AND evidence_id = ?`,
		recordID, evidenceID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// ListLinksFor returns the evidence links of record recordID ordered by
// evidence ref number. The slice is never nil.
func ListLinksFor(ctx context.Context, q Querier, t LinkTarget, recordID int64) ([]domain.EvidenceLink, error) {
	s, err := t.spec()
	if err != nil {
		return nil, err
	}
	m, err := evidenceLinkQuery(ctx, q, `SELECT l.`+s.idColumn+`, e.ref_num, l.relation, e.title, l.created_at,
    l.created_by_kind, l.created_by_client
FROM `+s.linkTable+` l JOIN evidence e ON e.id = l.evidence_id
WHERE l.`+s.idColumn+` = ? ORDER BY e.ref_num`, recordID)
	if err != nil {
		return nil, err
	}
	if l := m[recordID]; l != nil {
		return l, nil
	}
	return []domain.EvidenceLink{}, nil
}

// listLinksForInvestigation returns the evidence links of every record of
// kind t in investigation invID, keyed by record id and ordered by evidence
// ref number.
func listLinksForInvestigation(ctx context.Context, q Querier, s targetSpec, invID int64) (map[int64][]domain.EvidenceLink, error) {
	return evidenceLinkQuery(ctx, q, `SELECT l.`+s.idColumn+`, e.ref_num, l.relation, e.title, l.created_at,
    l.created_by_kind, l.created_by_client
FROM `+s.linkTable+` l
JOIN evidence e ON e.id = l.evidence_id
JOIN `+s.table+` r ON r.id = l.`+s.idColumn+`
WHERE r.investigation_id = ? ORDER BY l.`+s.idColumn+`, e.ref_num`, invID)
}

// evidenceLinkQuery runs a (record id, evidence ref_num, relation, title,
// created_at, kind, client) query and groups the rows by record id.
func evidenceLinkQuery(ctx context.Context, q Querier, query string, arg int64) (map[int64][]domain.EvidenceLink, error) {
	rows, err := q.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]domain.EvidenceLink{}
	for rows.Next() {
		var (
			key, num  int64
			rel, kind string
			l         domain.EvidenceLink
		)
		if err := rows.Scan(&key, &num, &rel, &l.EvidenceTitle, &l.CreatedAt, &kind, &l.CreatedBy.Client); err != nil {
			return nil, err
		}
		l.EvidenceRef = domain.FormatRecordRef(domain.RecordEvidence, num)
		l.Relation = domain.Relation(rel)
		l.CreatedBy.Kind = domain.ActorKind(kind)
		out[key] = append(out[key], l)
	}
	return out, rows.Err()
}
