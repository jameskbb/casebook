// Package apptest provides helpers for tests that drive internal/app: a
// temp-dir App with a fixed clock, deterministic clocks, standard actors and
// JSON shape assertions. Phase 2 and 3 tests import it too.
package apptest

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
)

// FixedTime is the instant FixedClock always returns.
var FixedTime = time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)

// Standard test actors.
var (
	// Human is the desktop actor {human, casebook-ui}.
	Human = app.ActorDesktop()
	// AI is an MCP actor {ai, claude-code}.
	AI = app.ActorMCP("claude-code")
)

// FixedClock returns a clock that always reports FixedTime.
func FixedClock() func() time.Time {
	return func() time.Time { return FixedTime }
}

// StepClock returns a clock that reports start, start+step, start+2*step, ...
// on successive calls. It is safe for concurrent use.
func StepClock(start time.Time, step time.Duration) func() time.Time {
	var (
		mu   sync.Mutex
		next = start
	)
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		t := next
		next = next.Add(step)
		return t
	}
}

// Open opens an App on a fresh casebook.db in t.TempDir() with FixedClock
// (caller options are applied after it, so they win) and closes it when the
// test ends. It fails the test on error.
func Open(t testing.TB, opts ...app.Option) *app.App {
	t.Helper()
	return Reopen(t, filepath.Join(t.TempDir(), "casebook.db"), opts...)
}

// Reopen opens another App on path (for close-and-reopen and two-App tests)
// with FixedClock first and caller options after, and closes it when the test
// ends.
func Reopen(t testing.TB, path string, opts ...app.Option) *app.App {
	t.Helper()
	all := append([]app.Option{app.WithClock(FixedClock())}, opts...)
	a, err := app.Open(path, all...)
	if err != nil {
		t.Fatalf("apptest: open %s: %v", path, err)
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Errorf("apptest: close %s: %v", path, err)
		}
	})
	return a
}

// AssertNoJSONNulls fails the test when v marshals to JSON containing a null
// token (D-18: DTO slices are never nil).
func AssertNoJSONNulls(t testing.TB, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("apptest: marshal %T: %v", v, err)
	}
	if hasNullToken(b) {
		t.Fatalf("apptest: %T marshals with a null: %s", v, b)
	}
}

// hasNullToken reports whether the JSON document b contains a null value,
// ignoring the text "null" inside strings.
func hasNullToken(b []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(b))
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if tok == nil {
			return true
		}
	}
}
