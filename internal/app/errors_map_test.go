package app

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/domain"
)

// TestErrorMappingConstraint checks the mapping function directly with a real
// SQLite constraint error, which no public method can provoke yet.
func TestErrorMappingConstraint(t *testing.T) {
	raw, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec(`CREATE TABLE t (v TEXT NOT NULL CHECK (v IN ('a')))`); err != nil {
		t.Fatal(err)
	}
	_, cerr := raw.Exec(`INSERT INTO t VALUES ('b')`)
	if cerr == nil {
		t.Fatal("constraint not enforced")
	}
	var logs strings.Builder
	a := &App{log: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))}

	got := a.fail("claim.add", cerr)
	if !errors.Is(got, domain.ErrConflict) || strings.Contains(got.Error(), "CHECK") {
		t.Fatalf("constraint mapped to %v, want ErrConflict without SQLite text", got)
	}
	for _, passthrough := range []error{
		domain.ErrNotFound, domain.ErrConflict, domain.ErrNoActiveInvestigation, domain.ErrBusy, domain.ErrSchemaTooNew,
		domain.NewValidationError("title", "is required"), context.Canceled, context.DeadlineExceeded,
	} {
		wrapped := errors.Join(passthrough)
		if got := a.fail("op", wrapped); got != wrapped {
			t.Errorf("fail(%v) = %v, want it unchanged", passthrough, got)
		}
	}
	if got := a.fail("op", nil); got != nil {
		t.Errorf("fail(nil) = %v", got)
	}
	unknown := errors.New("disk I/O error (10)")
	got = a.fail("investigation.get", unknown)
	if !errors.Is(got, domain.ErrInternal) || strings.Contains(got.Error(), "disk") {
		t.Fatalf("unknown error mapped to %v, want bare ErrInternal", got)
	}
	if !strings.Contains(logs.String(), "investigation.get") || !strings.Contains(logs.String(), "disk I/O error") {
		t.Fatalf("raw error not logged at Warn with op: %q", logs.String())
	}
}
