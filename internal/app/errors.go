package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// fail is the single error mapping every App method uses on its error path,
// so raw SQLite text never reaches a surface:
//
//   - domain sentinels, *ValidationError and context errors pass unchanged;
//   - lock contention becomes ErrBusy;
//   - a constraint violation becomes ErrConflict naming the operation;
//   - anything else is logged at Warn (op and raw error) and becomes
//     ErrInternal.
func (a *App) fail(op string, err error) error {
	if err == nil {
		return nil
	}
	if passThrough(err) {
		return err
	}
	if store.IsBusy(err) {
		a.log.Debug("app.busy", "op", op)
		return domain.ErrBusy
	}
	if store.IsConstraint(err) {
		a.log.Warn("app.constraint", "op", op, "err", err)
		return fmt.Errorf("%w: %s violates a data rule", domain.ErrConflict, op)
	}
	a.log.Warn("app.internal_error", "op", op, "err", err)
	return domain.ErrInternal
}

// passThrough reports whether err is already a caller-facing error.
func passThrough(err error) bool {
	for _, sentinel := range []error{
		domain.ErrNotFound, domain.ErrConflict, domain.ErrNoActiveInvestigation,
		domain.ErrBusy, domain.ErrInternal, domain.ErrSchemaTooNew,
		context.Canceled, context.DeadlineExceeded,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	_, ok := domain.IsValidation(err)
	return ok
}
