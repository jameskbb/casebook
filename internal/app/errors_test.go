package app_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"

	_ "modernc.org/sqlite" // raw lock holder in the busy test
)

func openTestApp(t *testing.T, opts ...app.Option) (*app.App, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "casebook.db")
	a, err := app.Open(path, opts...)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	return a, path
}

func assertNoRawSQLite(t *testing.T, err error) {
	t.Helper()
	msg := err.Error()
	for _, bad := range []string{"locked", "SQLITE", "sql:", "constraint failed", "database is closed"} {
		if strings.Contains(msg, bad) {
			t.Fatalf("error leaks storage text %q: %q", bad, msg)
		}
	}
}

func TestErrorMappingNotFound(t *testing.T) {
	a, _ := openTestApp(t)
	_, err := a.GetInvestigation(context.Background(), "INV-0042")
	if !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("GetInvestigation(INV-0042) = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "INV-0042") {
		t.Fatalf("not found message %q does not name the ref", err.Error())
	}
	_, err = a.ListActivity(context.Background(), app.ActivityFilter{InvestigationRef: "inv-42"})
	if !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("ListActivity(inv-42) = %v, want ErrNotFound", err)
	}
}

func TestErrorMappingNoActiveInvestigation(t *testing.T) {
	a, _ := openTestApp(t)
	_, err := a.GetInvestigation(context.Background(), "")
	if !errors.Is(err, app.ErrNoActiveInvestigation) || errors.Is(err, app.ErrNotFound) {
		t.Fatalf("GetInvestigation(\"\") = %v, want ErrNoActiveInvestigation only", err)
	}
}

func TestErrorMappingValidation(t *testing.T) {
	a, _ := openTestApp(t)
	ctx := context.Background()
	cases := []struct {
		actor app.Actor
		in    app.CreateInvestigationInput
		field string
	}{
		{app.ActorCLI(), app.CreateInvestigationInput{Title: "  "}, "title"},
		{app.ActorCLI(), app.CreateInvestigationInput{Title: strings.Repeat("t", 201)}, "title"},
		{app.ActorCLI(), app.CreateInvestigationInput{Title: "ok", ProblemStatement: strings.Repeat("p", 4001)}, "problem_statement"},
		{app.ActorCLI(), app.CreateInvestigationInput{Title: "ok", Status: app.InvestigationStatusArchived}, "status"},
		{app.ActorCLI(), app.CreateInvestigationInput{Title: "ok", Status: "bogus"}, "status"},
		{app.Actor{Kind: "robot", Client: "x"}, app.CreateInvestigationInput{Title: "ok"}, "actor.kind"},
		{app.Actor{Kind: app.ActorAI, Client: ""}, app.CreateInvestigationInput{Title: "ok"}, "actor.client"},
	}
	for _, c := range cases {
		_, err := a.CreateInvestigation(ctx, c.actor, c.in)
		ve, ok := app.IsValidation(err)
		if !ok || ve.Field != c.field {
			t.Errorf("CreateInvestigation(%+v) = %v, want ValidationError on %s", c.in, err, c.field)
		}
	}
	_, err := a.GetInvestigation(ctx, "E-001")
	if ve, ok := app.IsValidation(err); !ok || ve.Field != "investigation_ref" {
		t.Fatalf("GetInvestigation(E-001) = %v, want ValidationError on investigation_ref", err)
	}
	acts, err := a.ListActivity(ctx, app.ActivityFilter{})
	if err != nil || len(acts) != 0 {
		t.Fatalf("rejected input wrote activity: %v, %v", acts, err)
	}
}

func TestErrorMappingBusy(t *testing.T) {
	a, path := openTestApp(t, app.WithBusyTimeout(20*time.Millisecond), app.WithRetryBudget(300*time.Millisecond))
	ctx := context.Background()

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	conn, err := raw.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = a.CreateInvestigation(ctx, app.ActorCLI(), app.CreateInvestigationInput{Title: "blocked"})
	if !errors.Is(err, app.ErrBusy) {
		t.Fatalf("CreateInvestigation under lock = %v, want ErrBusy", err)
	}
	if err.Error() != "Casebook database is busy; try again" {
		t.Fatalf("busy message = %q", err.Error())
	}
	assertNoRawSQLite(t, err)
	if time.Since(start) > 2*time.Second {
		t.Fatalf("busy took %v", time.Since(start))
	}
	// Reads are not blocked by a writer in WAL mode.
	if _, err := a.ListActivity(ctx, app.ActivityFilter{}); err != nil {
		t.Fatalf("read during lock = %v", err)
	}
	if _, err := conn.ExecContext(ctx, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	inv, err := a.CreateInvestigation(ctx, app.ActorCLI(), app.CreateInvestigationInput{Title: "unblocked"})
	if err != nil || inv.Ref != "INV-0001" {
		t.Fatalf("after release = %+v, %v", inv, err)
	}
}

func TestErrorMappingInternalAfterClose(t *testing.T) {
	a, _ := openTestApp(t)
	ctx := context.Background()
	if _, err := a.CreateInvestigation(ctx, app.ActorCLI(), app.CreateInvestigationInput{Title: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	checks := map[string]error{}
	_, checks["GetInvestigation"] = a.GetInvestigation(ctx, "INV-0001")
	_, checks["CreateInvestigation"] = a.CreateInvestigation(ctx, app.ActorCLI(), app.CreateInvestigationInput{Title: "y"})
	_, checks["ListActivity"] = a.ListActivity(ctx, app.ActivityFilter{})
	for op, err := range checks {
		if !errors.Is(err, app.ErrInternal) {
			t.Errorf("%s after Close = %v, want ErrInternal", op, err)
			continue
		}
		assertNoRawSQLite(t, err)
	}
}

func TestErrorMappingContextCanceled(t *testing.T) {
	a, _ := openTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := a.GetInvestigation(ctx, "INV-0001")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetInvestigation with canceled ctx = %v, want context.Canceled", err)
	}
}

func TestErrorMappingOpenSchemaTooNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "casebook.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
INSERT INTO schema_migrations VALUES (99, '2030-01-01T00:00:00.000Z');`); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	_, err = app.Open(path)
	if !errors.Is(err, app.ErrSchemaTooNew) {
		t.Fatalf("Open newer DB = %v, want ErrSchemaTooNew", err)
	}
}

func TestErrorMappingOpenNotADatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "casebook.db")
	if err := os.WriteFile(path, []byte(strings.Repeat("not a sqlite file ", 400)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := app.Open(path)
	if !errors.Is(err, app.ErrInternal) {
		t.Fatalf("Open(garbage) = %v, want ErrInternal", err)
	}
	assertNoRawSQLite(t, err)
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("Open error %q does not name the path", err.Error())
	}
}
