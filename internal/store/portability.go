package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/jameskbb/casebook/internal/domain"
)

// This file holds the SQL behind ExportInvestigation (row loaders that
// address records by ref) and ImportInvestigation (full-row inserts that keep
// imported timestamps and authors). Table and column names come from fixed
// strings or LinkTarget.spec, never from input.

// ExportInvestigationRow loads investigation id as an export row.
func ExportInvestigationRow(ctx context.Context, q Querier, id int64) (domain.ExportInvestigation, error) {
	var (
		out          domain.ExportInvestigation
		refNum       int64
		status, conf string
		pre          sql.NullString
		kind         string
	)
	err := q.QueryRowContext(ctx, `SELECT ref_num, title, problem_statement, status, pre_archive_status,
    conclusion, conclusion_confidence, conclusion_rationale, created_at, updated_at,
    COALESCE(archived_at, ''), created_by_kind, created_by_client
FROM investigations WHERE id = ?`, id).Scan(&refNum, &out.Title, &out.ProblemStatement, &status, &pre,
		&out.Conclusion, &conf, &out.ConclusionRationale, &out.CreatedAt, &out.UpdatedAt,
		&out.ArchivedAt, &kind, &out.CreatedBy.Client)
	if err != nil {
		return domain.ExportInvestigation{}, err
	}
	out.Ref = domain.FormatInvestigationRef(refNum)
	out.Status = domain.InvestigationStatus(status)
	out.PreArchiveStatus = domain.InvestigationStatus(pre.String)
	out.ConclusionConfidence = domain.Confidence(conf)
	out.CreatedBy.Kind = domain.ActorKind(kind)
	inv, err := GetInvestigationByID(ctx, q, id)
	if err != nil {
		return out, err
	}
	out.ConclusionVerification = inv.ConclusionVerification
	out.ConclusionSupportingClaims = inv.ConclusionSupportingClaims
	out.ConclusionContradictingClaims = inv.ConclusionContradictingClaims
	out.ConclusionReviewedAt = inv.ConclusionReviewedAt
	out.ConclusionReviewBasis = inv.ConclusionReviewBasis
	return out, nil
}

// ExportEvidenceRows loads every evidence row of investigation invID,
// including raw detail and metadata, ordered by ref number.
func ExportEvidenceRows(ctx context.Context, q Querier, invID int64) ([]domain.ExportEvidence, error) {
	rows, err := q.QueryContext(ctx, `SELECT ref_num, title, summary, raw_detail, source_type, source_ref,
    source_label, captured_at, verification, metadata, created_at, updated_at, created_by_kind, created_by_client
FROM evidence WHERE investigation_id = ? ORDER BY ref_num`, invID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ExportEvidence{}
	for rows.Next() {
		var (
			e                       domain.ExportEvidence
			refNum                  int64
			st, ver, metadata, kind string
		)
		if err := rows.Scan(&refNum, &e.Title, &e.Summary, &e.RawDetail, &st, &e.SourceRef, &e.SourceLabel,
			&e.CapturedAt, &ver, &metadata, &e.CreatedAt, &e.UpdatedAt, &kind, &e.CreatedBy.Client); err != nil {
			return nil, err
		}
		e.Ref = domain.FormatRecordRef(domain.RecordEvidence, refNum)
		e.SourceType = domain.SourceType(st)
		e.Verification = domain.Verification(ver)
		if metadata != "" {
			e.Metadata = json.RawMessage(metadata)
		}
		e.CreatedBy.Kind = domain.ActorKind(kind)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ExportStatement is a claim or hypothesis row addressed by refs.
type ExportStatement struct {
	Ref          string
	Statement    string
	Confidence   domain.Confidence
	Rationale    string
	Status       string
	Verification domain.Verification
	SupersededBy string // replacement ref, or ""
	CreatedAt    string
	UpdatedAt    string
	CreatedBy    domain.Actor
}

// ExportStatementRows loads every claim or hypothesis of investigation invID
// ordered by ref number, with superseded_by as the replacement's ref.
func ExportStatementRows(ctx context.Context, q Querier, t LinkTarget, invID int64) ([]ExportStatement, error) {
	s, err := t.spec()
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, `SELECT r.ref_num, r.statement, r.confidence, r.`+s.rationaleCol+`, r.status,
    r.verification, COALESCE(sup.ref_num, 0), r.created_at, r.updated_at, r.created_by_kind, r.created_by_client
FROM `+s.table+` r LEFT JOIN `+s.table+` sup ON sup.id = r.superseded_by_id
WHERE r.investigation_id = ? ORDER BY r.ref_num`, invID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExportStatement{}
	for rows.Next() {
		var (
			e              ExportStatement
			refNum, supNum int64
			conf, ver      string
			kind           string
		)
		if err := rows.Scan(&refNum, &e.Statement, &conf, &e.Rationale, &e.Status, &ver, &supNum,
			&e.CreatedAt, &e.UpdatedAt, &kind, &e.CreatedBy.Client); err != nil {
			return nil, err
		}
		e.Ref = domain.FormatRecordRef(s.recordType, refNum)
		e.Confidence = domain.Confidence(conf)
		e.Verification = domain.Verification(ver)
		if supNum != 0 {
			e.SupersededBy = domain.FormatRecordRef(s.recordType, supNum)
		}
		e.CreatedBy.Kind = domain.ActorKind(kind)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ExportQuestionRows loads every question of investigation invID ordered by
// ref number.
func ExportQuestionRows(ctx context.Context, q Querier, invID int64) ([]domain.ExportQuestion, error) {
	rows, err := q.QueryContext(ctx, `SELECT ref_num, question, status, resolution, defer_note,
    COALESCE(resolved_at, ''), created_at, updated_at, created_by_kind, created_by_client
FROM questions WHERE investigation_id = ? ORDER BY ref_num`, invID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ExportQuestion{}
	for rows.Next() {
		var (
			e            domain.ExportQuestion
			refNum       int64
			status, kind string
		)
		if err := rows.Scan(&refNum, &e.Question, &status, &e.Resolution, &e.DeferNote, &e.ResolvedAt,
			&e.CreatedAt, &e.UpdatedAt, &kind, &e.CreatedBy.Client); err != nil {
			return nil, err
		}
		e.Ref = domain.FormatRecordRef(domain.RecordQuestion, refNum)
		e.Status = domain.QuestionStatus(status)
		e.CreatedBy.Kind = domain.ActorKind(kind)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ExportLink is one claim or hypothesis evidence link addressed by refs.
type ExportLink struct {
	RecordRef   string
	EvidenceRef string
	Relation    domain.Relation
	CreatedAt   string
	CreatedBy   domain.Actor
}

// ExportLinkRows loads every claim or hypothesis evidence link of
// investigation invID ordered by record ref number, then evidence ref number.
func ExportLinkRows(ctx context.Context, q Querier, t LinkTarget, invID int64) ([]ExportLink, error) {
	s, err := t.spec()
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, `SELECT r.ref_num, e.ref_num, l.relation, l.created_at, l.created_by_kind,
    l.created_by_client
FROM `+s.linkTable+` l
JOIN `+s.table+` r ON r.id = l.`+s.idColumn+`
JOIN evidence e ON e.id = l.evidence_id
WHERE r.investigation_id = ? ORDER BY r.ref_num, e.ref_num`, invID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExportLink{}
	for rows.Next() {
		var (
			l              ExportLink
			recNum, evNum  int64
			relation, kind string
		)
		if err := rows.Scan(&recNum, &evNum, &relation, &l.CreatedAt, &kind, &l.CreatedBy.Client); err != nil {
			return nil, err
		}
		l.RecordRef = domain.FormatRecordRef(s.recordType, recNum)
		l.EvidenceRef = domain.FormatRecordRef(domain.RecordEvidence, evNum)
		l.Relation = domain.Relation(relation)
		l.CreatedBy.Kind = domain.ActorKind(kind)
		out = append(out, l)
	}
	return out, rows.Err()
}

// ExportActivityRows loads the activity rows of investigation invID in the
// order they were written (id ascending). Global rows are not included.
func ExportActivityRows(ctx context.Context, q Querier, invID int64) ([]domain.ExportActivity, error) {
	rows, err := q.QueryContext(ctx, `SELECT actor_kind, actor_client, action, target_ref, summary, note, created_at
FROM activity WHERE investigation_id = ? ORDER BY id`, invID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ExportActivity{}
	for rows.Next() {
		var (
			a    domain.ExportActivity
			kind string
		)
		if err := rows.Scan(&kind, &a.Actor.Client, &a.Action, &a.TargetRef, &a.Summary, &a.Note, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Actor.Kind = domain.ActorKind(kind)
		out = append(out, a)
	}
	return out, rows.Err()
}

// InvestigationNumTaken reports whether an investigation uses ref number n.
func InvestigationNumTaken(ctx context.Context, q Querier, n int64) (bool, error) {
	var taken bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM investigations WHERE ref_num = ?)`, n).Scan(&taken)
	return taken, err
}

// RaiseInvestigationCounter makes the investigation ref counter at least n
// and at least the highest ref number in use, so the next allocation never
// collides with an imported ref.
func RaiseInvestigationCounter(ctx context.Context, q Querier, n int64) error {
	_, err := q.ExecContext(ctx, `INSERT INTO counters (name, value)
VALUES ('investigation', max(?, (SELECT COALESCE(MAX(ref_num), 0) FROM investigations)))
ON CONFLICT (name) DO UPDATE SET value = max(value, excluded.value)`, n)
	return err
}

// ImportedInvestigation is a full investigation row from an import. The
// Last*Num fields seed the per-investigation ref counters.
type ImportedInvestigation struct {
	RefNum                        int64
	Title                         string
	ProblemStatement              string
	Status                        domain.InvestigationStatus
	PreArchiveStatus              domain.InvestigationStatus // "" stores NULL
	Conclusion                    string
	ConclusionConfidence          domain.Confidence
	ConclusionRationale           string
	ConclusionVerification        domain.Verification
	ConclusionSupportingClaims    []string
	ConclusionContradictingClaims []string
	CreatedAt                     string
	UpdatedAt                     string
	ArchivedAt                    string // "" stores NULL
	CreatedBy                     domain.Actor
	LastEvidenceNum               int64
	LastClaimNum                  int64
	LastHypothesisNum             int64
	LastQuestionNum               int64
}

// InsertImportedInvestigation inserts a full investigation row and returns
// its id.
func InsertImportedInvestigation(ctx context.Context, q Querier, in ImportedInvestigation) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `INSERT INTO investigations
    (ref_num, title, problem_statement, status, pre_archive_status, conclusion, conclusion_confidence,
     conclusion_rationale, created_at, updated_at, archived_at, created_by_kind, created_by_client,
     last_evidence_num, last_claim_num, last_hypothesis_num, last_question_num)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id`,
		in.RefNum, in.Title, in.ProblemStatement, string(in.Status), nullString(string(in.PreArchiveStatus)),
		in.Conclusion, string(in.ConclusionConfidence), in.ConclusionRationale, in.CreatedAt, in.UpdatedAt,
		nullString(in.ArchivedAt), string(in.CreatedBy.Kind), in.CreatedBy.Client,
		in.LastEvidenceNum, in.LastClaimNum, in.LastHypothesisNum, in.LastQuestionNum).Scan(&id)
	if err != nil {
		return 0, err
	}
	err = SetConclusionAccountability(ctx, q, domain.Investigation{ID: id, ConclusionVerification: in.ConclusionVerification, ConclusionSupportingClaims: in.ConclusionSupportingClaims, ConclusionContradictingClaims: in.ConclusionContradictingClaims}, "")
	return id, err
}

// ImportedEvidence is a full evidence row from an import.
type ImportedEvidence struct {
	NewEvidence
	UpdatedAt string
}

// InsertImportedEvidence inserts a full evidence row, keeping updated_at, and
// returns its id.
func InsertImportedEvidence(ctx context.Context, q Querier, e ImportedEvidence) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `INSERT INTO evidence
    (investigation_id, ref_num, title, summary, raw_detail, source_type, source_ref, source_label,
     captured_at, created_at, updated_at, created_by_kind, created_by_client, verification, metadata, imported)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
RETURNING id`,
		e.InvestigationID, e.RefNum, e.Title, e.Summary, e.RawDetail, string(e.SourceType), e.SourceRef,
		e.SourceLabel, e.CapturedAt, e.CreatedAt, e.UpdatedAt, string(e.CreatedBy.Kind), e.CreatedBy.Client,
		string(e.Verification), e.Metadata).Scan(&id)
	return id, err
}

// ImportedStatement is a full claim or hypothesis row from an import, without
// superseded_by (set afterwards with SetImportedSupersededBy once every row
// has an id).
type ImportedStatement struct {
	NewStatement
	UpdatedAt string
}

// InsertImportedStatement inserts a full claim or hypothesis row, keeping
// updated_at, and returns its id.
func InsertImportedStatement(ctx context.Context, q Querier, t LinkTarget, n ImportedStatement) (int64, error) {
	s, err := t.spec()
	if err != nil {
		return 0, err
	}
	var id int64
	err = q.QueryRowContext(ctx, `INSERT INTO `+s.table+`
    (investigation_id, ref_num, statement, confidence, `+s.rationaleCol+`, status, verification,
     created_at, updated_at, created_by_kind, created_by_client, imported)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
RETURNING id`,
		n.InvestigationID, n.RefNum, n.Statement, string(n.Confidence), n.Rationale, n.Status,
		string(n.Verification), n.CreatedAt, n.UpdatedAt, string(n.CreatedBy.Kind), n.CreatedBy.Client).Scan(&id)
	return id, err
}

// SetImportedSupersededBy sets superseded_by_id on an imported claim or
// hypothesis without touching updated_at.
func SetImportedSupersededBy(ctx context.Context, q Querier, t LinkTarget, id, supersededByID int64) error {
	s, err := t.spec()
	if err != nil {
		return err
	}
	res, err := q.ExecContext(ctx, `UPDATE `+s.table+` SET superseded_by_id = ? WHERE id = ?`, supersededByID, id)
	if err != nil {
		return err
	}
	return expectOne(res)
}

// ImportedQuestion is a full question row from an import.
type ImportedQuestion struct {
	InvestigationID int64
	RefNum          int64
	Question        string
	Status          domain.QuestionStatus
	Resolution      string
	DeferNote       string
	ResolvedAt      string // "" stores NULL
	CreatedAt       string
	UpdatedAt       string
	CreatedBy       domain.Actor
}

// InsertImportedQuestion inserts a full question row and returns its id.
func InsertImportedQuestion(ctx context.Context, q Querier, n ImportedQuestion) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `INSERT INTO questions
    (investigation_id, ref_num, question, status, resolution, defer_note, resolved_at, created_at, updated_at,
     created_by_kind, created_by_client, imported)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
RETURNING id`,
		n.InvestigationID, n.RefNum, n.Question, string(n.Status), n.Resolution, n.DeferNote,
		nullString(n.ResolvedAt), n.CreatedAt, n.UpdatedAt, string(n.CreatedBy.Kind), n.CreatedBy.Client).Scan(&id)
	return id, err
}

// InsertImportedLink inserts a claim or hypothesis evidence link keeping its
// original created_at and author.
func InsertImportedLink(ctx context.Context, q Querier, t LinkTarget, recordID, evidenceID int64,
	relation domain.Relation, createdAt string, by domain.Actor) error {
	s, err := t.spec()
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `INSERT INTO `+s.linkTable+`
    (`+s.idColumn+`, evidence_id, relation, created_at, created_by_kind, created_by_client)
VALUES (?, ?, ?, ?, ?, ?)`, recordID, evidenceID, string(relation), createdAt, string(by.Kind), by.Client)
	return err
}

// SameCaseInvestigations returns the ref numbers of investigations other
// than excludeID created at createdAt by creator, in ref order: the same
// case as an imported one (ADR-0016).
func SameCaseInvestigations(ctx context.Context, q Querier, createdAt string, creator domain.Actor, excludeID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `SELECT ref_num FROM investigations
WHERE created_at = ? AND created_by_kind = ? AND created_by_client = ? AND id != ? ORDER BY ref_num`,
		createdAt, string(creator.Kind), creator.Client, excludeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var n int64
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
