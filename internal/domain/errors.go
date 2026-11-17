package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors. Wrap them with fmt.Errorf("%w: ...") to say what failed;
// callers test with errors.Is.
var (
	// ErrNotFound means the addressed investigation or record does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict means the request contradicts existing data.
	ErrConflict = errors.New("conflict")
	// ErrNoActiveInvestigation means an empty investigation ref was given and
	// no active investigation is set. It is distinct from ErrNotFound.
	ErrNoActiveInvestigation = errors.New("no active investigation is set; pass an investigation ref or set one active")
	// ErrBusy means the database stayed locked past the retry budget.
	ErrBusy = errors.New("Casebook database is busy; try again")
	// ErrInternal replaces any unexpected storage failure so that raw SQLite
	// text never escapes the app layer.
	ErrInternal = errors.New("casebook: internal storage error")
	// ErrSchemaTooNew means the database was written by a newer Casebook.
	ErrSchemaTooNew = errors.New("this database was saved by a newer version of Casebook; update Casebook to open it")
)

// IntegrityError is returned by CheckIntegrity when the database has
// problems. It wraps ErrInternal, so errors.Is(err, ErrInternal) is true for
// surfaces that only show a generic failure, while diagnostics can list the
// problems with errors.As. Problems are sanitized one-line descriptions
// naming only the check, tables, indexes and row ids (never raw SQLite text
// or column values); Total counts every problem found, including those
// beyond the listed ones.
type IntegrityError struct {
	Problems []string `json:"problems"`
	Total    int      `json:"total"`
}

// Error returns "casebook: database integrity check failed (N problems): a; b".
func (e *IntegrityError) Error() string {
	noun := "problems"
	if e.Total == 1 {
		noun = "problem"
	}
	msg := fmt.Sprintf("casebook: database integrity check failed (%d %s)", e.Total, noun)
	if len(e.Problems) > 0 {
		msg += ": " + strings.Join(e.Problems, "; ")
	}
	if more := e.Total - len(e.Problems); more > 0 && len(e.Problems) > 0 {
		msg += fmt.Sprintf("; and %d more", more)
	}
	return msg
}

// Unwrap makes errors.Is(err, ErrInternal) true.
func (e *IntegrityError) Unwrap() error { return ErrInternal }

// IsIntegrity reports whether err is or wraps an *IntegrityError and returns
// it.
func IsIntegrity(err error) (*IntegrityError, bool) {
	var ie *IntegrityError
	if errors.As(err, &ie) {
		return ie, true
	}
	return nil, false
}

// ValidationError reports invalid caller input for one field.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error returns "field: message".
func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// NewValidationError builds a ValidationError with a formatted message.
func NewValidationError(field, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// IsValidation reports whether err is or wraps a *ValidationError and returns
// it.
func IsValidation(err error) (*ValidationError, bool) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve, true
	}
	return nil, false
}
