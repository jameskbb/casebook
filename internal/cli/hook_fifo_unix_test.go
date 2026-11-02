//go:build unix

package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestHookFIFO proves the hooks never open the transcript path: the payload
// names a FIFO with no writer, which would block forever if opened, and
// each hook must still finish within 3 seconds (D-10, T-04-09).
func TestHookFIFO(t *testing.T) {
	_, db, _ := seededApp(t)
	fifo := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	for _, tc := range []struct {
		sub, event string
	}{
		{"checkpoint", "Stop"},
		{"session-start", "SessionStart"},
	} {
		payload, _ := json.Marshal(map[string]any{
			"session_id":      "fifo-1",
			"hook_event_name": tc.event,
			"transcript_path": fifo,
		})
		done := make(chan cliResult, 1)
		go func() {
			done <- runHook(t, strings.NewReader(string(payload)), "--db", db, "hook", tc.sub)
		}()
		select {
		case r := <-done:
			wantCode(t, r, 0)
		case <-time.After(3 * time.Second):
			t.Fatalf("hook %s did not finish within 3s with a FIFO transcript path (did it open the transcript?)", tc.sub)
		}
	}
}
