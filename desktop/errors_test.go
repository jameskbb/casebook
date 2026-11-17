package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

func TestMapErr(t *testing.T) {
	if mapErr(nil) != nil {
		t.Fatal("mapErr(nil) must be nil")
	}
	cases := []struct {
		name      string
		in        error
		kind      string
		field     string
		wantField bool
	}{
		{"not found", app.ErrNotFound, kindNotFound, "", false},
		{"conflict", app.ErrConflict, kindConflict, "", false},
		{"no active", app.ErrNoActiveInvestigation, kindNoActive, "", false},
		{"busy", app.ErrBusy, kindBusy, "", false},
		{"internal", app.ErrInternal, kindInternal, "", false},
		{"validation", app.NewValidationError("title", "is required"), kindValidation, "title", true},
		{"unknown", errors.New("open /x: permission denied"), kindOther, "", false},
		{"schema too new", app.ErrSchemaTooNew, kindOther, "", false},
	}
	for _, c := range cases {
		for _, wrapped := range []bool{false, true} {
			in := c.in
			if wrapped {
				in = fmt.Errorf("outer: %w", c.in)
			}
			name := c.name
			if wrapped {
				name += " wrapped"
			}
			t.Run(name, func(t *testing.T) {
				got := mapErr(in)
				var ue uiError
				if err := json.Unmarshal([]byte(got.Error()), &ue); err != nil {
					t.Fatalf("not JSON: %q", got.Error())
				}
				if ue.Kind != c.kind {
					t.Fatalf("kind = %q, want %q", ue.Kind, c.kind)
				}
				if ue.Field != c.field {
					t.Fatalf("field = %q, want %q", ue.Field, c.field)
				}
				if ue.Message == "" {
					t.Fatal("empty message")
				}
				var raw map[string]any
				_ = json.Unmarshal([]byte(got.Error()), &raw)
				if _, has := raw["field"]; has != c.wantField {
					t.Fatalf("field present = %v, want %v (%s)", has, c.wantField, got.Error())
				}
			})
		}
	}
	if ue := decodeUIError(t, mapErr(app.NewValidationError("links[1].relation", "must be supports or contradicts"))); ue.Message != "must be supports or contradicts" {
		t.Fatalf("validation message = %q, want the engine message alone", ue.Message)
	}
}

func TestNativeErr(t *testing.T) {
	if nativeErr("clipboard", nil) != nil {
		t.Fatal("nativeErr(nil) must be nil")
	}
	ue := decodeUIError(t, nativeErr("dialog", errors.New("no window")))
	if ue.Kind != kindOther || ue.Field != "dialog" || ue.Message != "no window" {
		t.Fatalf("nativeErr = %+v", ue)
	}
}

func TestStartupInfo(t *testing.T) {
	if s := startupInfo(nil, "/x/casebook.db"); !s.OK || s.DBPath != "/x/casebook.db" {
		t.Fatalf("ok case = %+v", s)
	}
	if s := startupInfo(fmt.Errorf("open: %w", app.ErrSchemaTooNew), "/x"); s.OK || s.Kind != "schema_too_new" || !s.Recoverable {
		t.Fatalf("schema case = %+v", s)
	}
	if s := startupInfo(errors.New("disk full"), "/x"); s.OK || s.Kind != kindOther || s.Message != "disk full" || s.Recoverable {
		t.Fatalf("other case = %+v", s)
	}
}

// TestRecoverable: only a file that is not a usable Casebook database, or
// one from a newer Casebook, is something a backup fixes.
func TestRecoverable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"not a database", fmt.Errorf("%w: cannot open /x as a Casebook database", app.ErrInternal), true},
		{"newer schema", fmt.Errorf("open: %w", app.ErrSchemaTooNew), true},
		{"busy", app.ErrBusy, false},
		{"permission", &fs.PathError{Op: "open", Path: "/x", Err: fs.ErrPermission}, false},
		{"data path", errors.New("resolve data directory: no home"), false},
		{"conflict", fmt.Errorf("%w: too many copies", app.ErrConflict), false},
	}
	for _, c := range cases {
		if got := recoverable(c.err); got != c.want {
			t.Errorf("recoverable(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}
