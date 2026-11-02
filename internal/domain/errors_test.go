package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorsSentinelsWrap(t *testing.T) {
	err := fmt.Errorf("%w: claim C-009 in INV-0001", ErrNotFound)
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("wrapped ErrNotFound not matched by errors.Is")
	}
	if errors.Is(err, ErrConflict) {
		t.Fatal("ErrNotFound matched ErrConflict")
	}
	if err.Error() != "not found: claim C-009 in INV-0001" {
		t.Fatalf("message = %q", err.Error())
	}
}

func TestErrorsMessages(t *testing.T) {
	cases := map[error]string{
		ErrBusy:                  "Casebook database is busy; try again",
		ErrNotFound:              "not found",
		ErrConflict:              "conflict",
		ErrNoActiveInvestigation: "no active investigation is set; pass an investigation ref or set one active",
		ErrInternal:              "casebook: internal storage error",
		ErrSchemaTooNew:          "this database was saved by a newer version of Casebook; update Casebook to open it",
	}
	for err, want := range cases {
		if err.Error() != want {
			t.Errorf("Error() = %q, want %q", err.Error(), want)
		}
	}
	if errors.Is(ErrNoActiveInvestigation, ErrNotFound) {
		t.Fatal("ErrNoActiveInvestigation must be distinct from ErrNotFound")
	}
}

func TestErrorsValidation(t *testing.T) {
	ve := NewValidationError("title", "must be at most %d bytes", 200)
	if ve.Error() != "title: must be at most 200 bytes" {
		t.Fatalf("Error() = %q", ve.Error())
	}
	wrapped := fmt.Errorf("create investigation: %w", ve)
	got, ok := IsValidation(wrapped)
	if !ok || got.Field != "title" || got.Message != "must be at most 200 bytes" {
		t.Fatalf("IsValidation(wrapped) = %+v, %v", got, ok)
	}
	if _, ok := IsValidation(ErrNotFound); ok {
		t.Fatal("IsValidation(ErrNotFound) = true")
	}
	if _, ok := IsValidation(nil); ok {
		t.Fatal("IsValidation(nil) = true")
	}
}
