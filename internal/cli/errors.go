package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/app"
)

// usageError marks a bad command line (unknown command or flag, wrong
// arguments, missing or conflicting flags). It maps to exit code 2.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// newUsageError returns a usage error with a formatted message. Commands use
// it for required and mutually exclusive flags, which they check themselves
// (cobra's helpers would bypass the flag error func and exit 1).
func newUsageError(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...)}
}

// usageArgs wraps a positional argument validator so its errors are usage
// errors.
func usageArgs(v cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := v(cmd, args); err != nil {
			return &usageError{err: err}
		}
		return nil
	}
}

// exitCode maps an error to the process exit code (D-08): 0 success; 1
// runtime failure; 2 usage or invalid input; 3 not found or no active
// investigation.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ue *usageError
	if errors.As(err, &ue) {
		return 2
	}
	if _, ok := app.IsValidation(err); ok {
		return 2
	}
	if errors.Is(err, app.ErrNotFound) || errors.Is(err, app.ErrNoActiveInvestigation) {
		return 3
	}
	return 1
}

// formatError renders err as exactly one line starting "casebook: ".
// Validation errors read "invalid <field>: <message>"; a missing active
// investigation adds a hint. App errors are already safe to show (raw
// storage text never leaves internal/app).
func formatError(err error) string {
	var msg string
	if ve, ok := app.IsValidation(err); ok {
		msg = "invalid " + ve.Field + ": " + ve.Message
	} else if errors.Is(err, app.ErrNoActiveInvestigation) {
		msg = err.Error() + `; run "casebook case activate INV-0001" or pass a ref`
	} else {
		msg = strings.TrimPrefix(err.Error(), "casebook: ")
	}
	return "casebook: " + oneLine(msg)
}

// oneLine collapses every run of whitespace (including newlines and tabs)
// into one space.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
