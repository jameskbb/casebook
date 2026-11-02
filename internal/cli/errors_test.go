package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"plain", errors.New("x"), 1},
		{"usage", newUsageError("bad"), 2},
		{"wrapped usage", fmt.Errorf("parse: %w", &usageError{err: errors.New("unknown flag: --nope")}), 2},
		{"validation", app.NewValidationError("limit", "must be between 0 and 100"), 2},
		{"wrapped validation", fmt.Errorf("add: %w", app.NewValidationError("title", "is required")), 2},
		{"not found", fmt.Errorf("%w: investigation INV-0009", app.ErrNotFound), 3},
		{"no active", app.ErrNoActiveInvestigation, 3},
		{"conflict", app.ErrConflict, 1},
		{"busy", app.ErrBusy, 1},
		{"internal", app.ErrInternal, 1},
	}
	for _, c := range cases {
		if got := exitCode(c.err); got != c.want {
			t.Errorf("%s: exitCode = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestFormatError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"validation", app.NewValidationError("limit", "must be between 0 and 100"),
			"casebook: invalid limit: must be between 0 and 100"},
		{"no active", app.ErrNoActiveInvestigation,
			"casebook: " + app.ErrNoActiveInvestigation.Error() + `; run "casebook case activate INV-0001" or pass a ref`},
		{"internal", app.ErrInternal, "casebook: internal storage error"},
		{"integrity keeps one prefix",
			&app.IntegrityError{Problems: []string{"integrity_check: damaged database pages"}, Total: 1},
			"casebook: database integrity check failed (1 problem): integrity_check: damaged database pages"},
		{"not found", fmt.Errorf("%w: investigation INV-0009", app.ErrNotFound), "casebook: not found: investigation INV-0009"},
		{"multi-line", errors.New("first line\nsecond\tline"), "casebook: first line second line"},
		{"usage", newUsageError(`unknown command %q for %q`, "bogus", "casebook"), `casebook: unknown command "bogus" for "casebook"`},
	}
	for _, c := range cases {
		got := formatError(c.err)
		if got != c.want {
			t.Errorf("%s: formatError = %q, want %q", c.name, got, c.want)
		}
		if strings.ContainsAny(got, "\n\t") {
			t.Errorf("%s: formatError is not one line: %q", c.name, got)
		}
	}
}

func TestLogLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"": slog.LevelWarn, "debug": slog.LevelDebug, " INFO ": slog.LevelInfo, "warn": slog.LevelWarn,
		"warning": slog.LevelWarn, "Error": slog.LevelError, "loud": slog.LevelWarn,
	}
	for in, want := range cases {
		if got := logLevel(in); got != want {
			t.Errorf("logLevel(%q) = %v, want %v", in, got, want)
		}
	}
}
