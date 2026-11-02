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

// resolveInvestigation resolves invRef inside a transaction. An empty (or
// whitespace-only) ref means the active investigation as this App resolves
// it (a directory binding of its working directory, else the global setting;
// ErrNoActiveInvestigation when none is set or it no longer exists);
// otherwise the ref is parsed leniently and looked up (ErrNotFound naming
// the ref when missing). IsActive follows the same resolution.
func (a *App) resolveInvestigation(ctx context.Context, q store.Querier, invRef string) (domain.Investigation, error) {
	invRef = strings.TrimSpace(invRef)
	if invRef == "" {
		id, ok, _, err := a.activeID(ctx, q)
		if err != nil {
			return domain.Investigation{}, err
		}
		if !ok {
			return domain.Investigation{}, domain.ErrNoActiveInvestigation
		}
		inv, err := a.getInvestigationByID(ctx, q, id)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Investigation{}, domain.ErrNoActiveInvestigation
		}
		return inv, err
	}
	n, err := domain.ParseInvestigationRef(invRef)
	if err != nil {
		return domain.Investigation{}, err
	}
	inv, err := store.GetInvestigationByNum(ctx, q, n)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Investigation{}, fmt.Errorf("%w: investigation %s", domain.ErrNotFound, domain.FormatInvestigationRef(n))
	}
	if err != nil {
		return inv, err
	}
	if err := a.markConclusionAttention(ctx, q, &inv); err != nil {
		return inv, err
	}
	return inv, a.markActive(ctx, q, &inv)
}

// recordTarget is a record ref resolved to its investigation.
type recordTarget struct {
	Inv  domain.Investigation
	Type domain.RecordType
	Num  int64
	Ref  string // canonical record ref, e.g. E-001
}

// notFound is the ErrNotFound error naming the record and its investigation,
// e.g. "not found: evidence E-009 in INV-0001".
func (r recordTarget) notFound() error {
	return fmt.Errorf("%w: %s %s in %s", domain.ErrNotFound, r.Type, r.Ref, r.Inv.Ref)
}

// resolveRecord resolves a record ref of type t inside a transaction. The ref
// is parsed leniently (wrong prefix is a ValidationError on ref). A qualified
// ref such as INV-0002/E-001 names its investigation: with an empty invRef it
// is used as is, and with a non-empty invRef naming another investigation it
// is a ValidationError on ref. An unqualified ref uses invRef (empty means the
// active investigation). Whether the record itself exists is left to the
// caller, which reports recordTarget.notFound.
func (a *App) resolveRecord(ctx context.Context, q store.Querier, invRef, recordRef string, t domain.RecordType) (recordTarget, error) {
	rr, err := domain.ParseRecordRef(recordRef, t)
	if err != nil {
		return recordTarget{}, err
	}
	ref := domain.FormatRecordRef(t, rr.Num)
	invRef = strings.TrimSpace(invRef)
	if rr.InvestigationNum != 0 {
		qualified := domain.FormatInvestigationRef(rr.InvestigationNum)
		if invRef != "" {
			n, err := domain.ParseInvestigationRef(invRef)
			if err != nil {
				return recordTarget{}, err
			}
			if n != rr.InvestigationNum {
				return recordTarget{}, domain.NewValidationError("ref", "%s/%s belongs to %s, not %s",
					qualified, ref, qualified, domain.FormatInvestigationRef(n))
			}
		}
		invRef = qualified
	}
	inv, err := a.resolveInvestigation(ctx, q, invRef)
	if err != nil {
		return recordTarget{}, err
	}
	return recordTarget{Inv: inv, Type: t, Num: rr.Num, Ref: ref}, nil
}
