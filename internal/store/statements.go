package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// Claims and hypotheses share one row shape (statement, confidence,
// rationale, status, verification, superseded_by_id), so their SQL lives here
// parameterized by LinkTarget. claims.go and hypotheses.go convert rows to
// the domain DTOs.

// StatementRow is a claim or hypothesis row with its evidence links.
type StatementRow struct {
	ID               int64
	InvestigationID  int64
	InvestigationRef string
	RefNum           int64
	Ref              string
	Statement        string
	Confidence       domain.Confidence
	Rationale        string
	Status           string
	Verification     domain.Verification
	SupersededByID   int64  // 0 when none
	SupersededByRef  string // "" when none
	CreatedAt        string
	UpdatedAt        string
	CreatedBy        domain.Actor
	Imported         bool
	Links            []domain.EvidenceLink // never nil
}

// NewStatement is the data needed to insert a claim or hypothesis row.
type NewStatement struct {
	InvestigationID int64
	RefNum          int64
	Statement       string
	Confidence      domain.Confidence
	Rationale       string
	Status          string
	Verification    domain.Verification
	CreatedAt       string
	CreatedBy       domain.Actor
}

// InsertStatement inserts a claim or hypothesis row and returns its id.
func InsertStatement(ctx context.Context, q Querier, t LinkTarget, n NewStatement) (int64, error) {
	s, err := t.spec()
	if err != nil {
		return 0, err
	}
	var id int64
	err = q.QueryRowContext(ctx, `INSERT INTO `+s.table+`
    (investigation_id, ref_num, statement, confidence, `+s.rationaleCol+`, status, verification,
     created_at, updated_at, created_by_kind, created_by_client)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id`,
		n.InvestigationID, n.RefNum, n.Statement, string(n.Confidence), n.Rationale, n.Status,
		string(n.Verification), n.CreatedAt, n.CreatedAt, string(n.CreatedBy.Kind), n.CreatedBy.Client).Scan(&id)
	return id, err
}

// statementSelect returns the SELECT ... FROM clause for t. The replacement
// row is joined for its ref number.
func statementSelect(s targetSpec) string {
	return `SELECT r.id, r.investigation_id, i.ref_num, r.ref_num, r.statement, r.confidence, r.` + s.rationaleCol + `,
    r.status, r.verification, COALESCE(r.superseded_by_id, 0), COALESCE(sup.ref_num, 0),
    r.created_at, r.updated_at, r.created_by_kind, r.created_by_client, r.imported
FROM ` + s.table + ` r
JOIN investigations i ON i.id = r.investigation_id
LEFT JOIN ` + s.table + ` sup ON sup.id = r.superseded_by_id`
}

func scanStatement(s targetSpec, row interface{ Scan(...any) error }) (StatementRow, error) {
	var (
		r                       StatementRow
		invNum, supNum          int64
		conf, ver, kind, status string
	)
	if err := row.Scan(&r.ID, &r.InvestigationID, &invNum, &r.RefNum, &r.Statement, &conf, &r.Rationale,
		&status, &ver, &r.SupersededByID, &supNum, &r.CreatedAt, &r.UpdatedAt, &kind, &r.CreatedBy.Client, &r.Imported); err != nil {
		return StatementRow{}, err
	}
	r.InvestigationRef = domain.FormatInvestigationRef(invNum)
	r.Ref = domain.FormatRecordRef(s.recordType, r.RefNum)
	r.Confidence = domain.Confidence(conf)
	r.Status = status
	r.Verification = domain.Verification(ver)
	if supNum != 0 {
		r.SupersededByRef = domain.FormatRecordRef(s.recordType, supNum)
	}
	r.CreatedBy.Kind = domain.ActorKind(kind)
	r.Links = []domain.EvidenceLink{}
	return r, nil
}

// GetStatement returns record refNum of kind t in investigation invID with
// its links, or an error matching sql.ErrNoRows.
func GetStatement(ctx context.Context, q Querier, t LinkTarget, invID, refNum int64) (StatementRow, error) {
	s, err := t.spec()
	if err != nil {
		return StatementRow{}, err
	}
	r, err := scanStatement(s, q.QueryRowContext(ctx, statementSelect(s)+`
WHERE r.investigation_id = ? AND r.ref_num = ?`, invID, refNum))
	if err != nil {
		return StatementRow{}, err
	}
	if r.Links, err = ListLinksFor(ctx, q, t, r.ID); err != nil {
		return StatementRow{}, err
	}
	return r, nil
}

// GetStatementByID returns the record of kind t with primary key id and its
// links, or an error matching sql.ErrNoRows.
func GetStatementByID(ctx context.Context, q Querier, t LinkTarget, id int64) (StatementRow, error) {
	s, err := t.spec()
	if err != nil {
		return StatementRow{}, err
	}
	r, err := scanStatement(s, q.QueryRowContext(ctx, statementSelect(s)+` WHERE r.id = ?`, id))
	if err != nil {
		return StatementRow{}, err
	}
	if r.Links, err = ListLinksFor(ctx, q, t, r.ID); err != nil {
		return StatementRow{}, err
	}
	return r, nil
}

// ListStatements returns every record of kind t in investigation invID
// ordered by ref number, each with its links. The slice is never nil.
func ListStatements(ctx context.Context, q Querier, t LinkTarget, invID int64) ([]StatementRow, error) {
	s, err := t.spec()
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, statementSelect(s)+`
WHERE r.investigation_id = ? ORDER BY r.ref_num`, invID)
	if err != nil {
		return nil, err
	}
	out := []StatementRow{}
	for rows.Next() {
		r, err := scanStatement(s, rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	links, err := listLinksForInvestigation(ctx, q, s, invID)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if l := links[out[i].ID]; l != nil {
			out[i].Links = l
		}
	}
	return out, nil
}

// Statement columns UpdateStatement accepts. "rationale" maps to the
// target's rationale column; superseded_by_id takes an int64 or nil.
var statementUpdatable = map[string]bool{
	"statement": true, "confidence": true, "rationale": true, "status": true, "verification": true,
	"superseded_by_id": true,
}

// UpdateStatement sets the given columns and updated_at on record id of kind
// t. Only whitelisted columns are accepted, so the search index is rewritten
// only when statement is among them. An empty set only touches updated_at.
func UpdateStatement(ctx context.Context, q Querier, t LinkTarget, id int64, set []Assignment, now string) error {
	s, err := t.spec()
	if err != nil {
		return err
	}
	parts := make([]string, 0, len(set)+1)
	args := make([]any, 0, len(set)+2)
	for _, a := range set {
		if !statementUpdatable[a.Column] {
			return fmt.Errorf("update %s: column %q is not updatable", s.table, a.Column)
		}
		col := a.Column
		if col == "rationale" {
			col = s.rationaleCol
		}
		parts = append(parts, col+" = ?")
		args = append(args, a.Value)
	}
	parts = append(parts, "updated_at = ?")
	args = append(args, now, id)
	res, err := q.ExecContext(ctx, `UPDATE `+s.table+` SET `+strings.Join(parts, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return err
	}
	return expectOne(res)
}
