package main

import (
	"encoding/json"
	"errors"

	"github.com/jameskbb/casebook/internal/app"
)

// uiError is the D-05 error shape that crosses to JavaScript. Wails rejects
// the JS promise with the error's text, so the text is this struct as JSON:
// {"kind":"validation","field":"title","message":"is required"}.
type uiError struct {
	Kind    string `json:"kind"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// Error kinds understood by the frontend (src/api/errors.ts).
const (
	kindValidation = "validation"
	kindNotFound   = "not_found"
	kindConflict   = "conflict"
	kindNoActive   = "no_active"
	kindBusy       = "busy"
	kindInternal   = "internal"
	kindOther      = "other"
)

// errNotOpen is returned by every binding when the database failed to open,
// so a failed startup never panics (the UI shows StartupError instead).
var errNotOpen = uiErr(kindInternal, "", "Casebook could not open its data")

// uiErr builds the JSON error value for one kind.
func uiErr(kind, field, message string) error {
	b, err := json.Marshal(uiError{Kind: kind, Field: field, Message: message})
	if err != nil {
		// Marshalling three strings cannot fail; keep a valid shape anyway.
		return errors.New(`{"kind":"internal","message":"Casebook could not report an error"}`)
	}
	return errors.New(string(b))
}

// mapErr turns an internal/app error into the D-05 JSON error. nil stays nil.
// Validation is checked first (docs/api.md section 6), then each sentinel;
// anything else (filesystem problems from Backup and Restore) is "other"
// with its message. Engine messages are already safe to show.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if ve, ok := app.IsValidation(err); ok {
		return uiErr(kindValidation, ve.Field, ve.Message)
	}
	switch {
	case errors.Is(err, app.ErrNotFound):
		return uiErr(kindNotFound, "", err.Error())
	case errors.Is(err, app.ErrConflict):
		return uiErr(kindConflict, "", err.Error())
	case errors.Is(err, app.ErrNoActiveInvestigation):
		return uiErr(kindNoActive, "", err.Error())
	case errors.Is(err, app.ErrBusy):
		return uiErr(kindBusy, "", err.Error())
	case errors.Is(err, app.ErrInternal):
		return uiErr(kindInternal, "", err.Error())
	}
	return uiErr(kindOther, "", err.Error())
}

// nativeErr reports a failure of a native facility (field "clipboard" or
// "dialog") as kind "other", so the UI can pick the matching copy.
func nativeErr(field string, err error) error {
	if err == nil {
		return nil
	}
	return uiErr(kindOther, field, err.Error())
}

// StartupInfo tells the UI whether the database opened (D-05). Kind is
// "schema_too_new" when the file comes from a newer Casebook, else "other".
type StartupInfo struct {
	OK      bool   `json:"ok"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
	DBPath  string `json:"db_path"`
	// Recoverable is true when restoring a backup can fix the failure: a
	// file that cannot be opened as a Casebook database, or a newer schema
	// (going back to this version restores the pre-upgrade backup). It
	// describes the startup error only; whether another program holds the
	// file is checked again when recovery runs.
	Recoverable bool `json:"recoverable"`
}

// startupInfo describes the result of opening the database at dbPath.
func startupInfo(err error, dbPath string) StartupInfo {
	if err == nil {
		return StartupInfo{OK: true, DBPath: dbPath}
	}
	if errors.Is(err, app.ErrSchemaTooNew) {
		return StartupInfo{Kind: "schema_too_new", Message: err.Error(), DBPath: dbPath, Recoverable: recoverable(err)}
	}
	return StartupInfo{Kind: kindOther, Message: err.Error(), DBPath: dbPath, Recoverable: recoverable(err)}
}
