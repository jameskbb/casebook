package integrations

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// HookInput is the part of a Claude Code hook payload Casebook reads. Only
// these five fields are decoded: the payload's transcript path and last
// assistant message are never declared, decoded, opened or stored (D-10).
// Cwd is the session's working directory, used only to apply the hook
// scope (see HookScope); it is never stored.
type HookInput struct {
	SessionID      string `json:"session_id"`
	HookEventName  string `json:"hook_event_name"`
	Source         string `json:"source"`
	StopHookActive bool   `json:"stop_hook_active"`
	Cwd            string `json:"cwd"`
}

// InputStatus is the outcome of reading a hook payload.
type InputStatus int

const (
	// InputEmpty: stdin is a terminal, holds no bytes (or only whitespace),
	// or nothing arrived within the timeout. A manual run.
	InputEmpty InputStatus = iota
	// InputOK: one JSON object within the size cap.
	InputOK
	// InputBad: oversize, invalid JSON, not an object, or partial data when
	// the timeout hit.
	InputBad
)

// MaxHookInputBytes caps how much of a hook payload is read.
const MaxHookInputBytes = 1 << 20

// MaxAdditionalContextBytes caps the context text a hook injects.
const MaxAdditionalContextBytes = 8000

// Texts the hooks emit (D-11, D-12).
const (
	sessionPreamble = "Casebook active investigation (shared memory across AI sessions).\n" +
		"If this session continues it, build on the pack below instead of starting over (do not re-test disproven hypotheses), " +
		"record new findings with the casebook_* MCP tools and call casebook_checkpoint before your final reply; if the task is unrelated, ignore it."
	noActiveHint = "Casebook: no active investigation. If this session turns into working out why something happened " +
		"(a number that moved, a failure being root-caused, work likely to continue later), offer to open a Casebook case; " +
		"to continue an earlier one, find it with casebook_search_investigations or casebook_list_investigations."
	truncatedSuffix = "\n(Casebook context truncated)"
)

// countingReader counts bytes read and whether any was not JSON whitespace.
type countingReader struct {
	r        io.Reader
	mu       sync.Mutex
	n        int64
	nonSpace bool
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.mu.Lock()
		c.n += int64(n)
		if !c.nonSpace && len(bytes.Trim(p[:n], jsonSpace)) > 0 {
			c.nonSpace = true
		}
		c.mu.Unlock()
	}
	return n, err
}

// state returns the byte count and whether real data was seen.
func (c *countingReader) state() (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n, c.nonSpace
}

// ReadHookInput reads one hook payload from r without ever blocking a
// session: a terminal is not read at all, at most MaxHookInputBytes+1 bytes
// are read, and the wait ends after timeout (a reader that never delivers
// is abandoned; the process exits soon after). Whitespace-only input counts
// as empty (a manual `echo | casebook hook ...`).
func ReadHookInput(r io.Reader, isTerminal bool, timeout time.Duration) (HookInput, InputStatus) {
	if isTerminal || r == nil {
		return HookInput{}, InputEmpty
	}
	cr := &countingReader{r: io.LimitReader(r, MaxHookInputBytes+1)}
	type result struct {
		in     HookInput
		status InputStatus
	}
	done := make(chan result, 1)
	go func() {
		var raw json.RawMessage
		err := json.NewDecoder(cr).Decode(&raw)
		n, nonSpace := cr.state()
		switch {
		case n > MaxHookInputBytes:
			done <- result{status: InputBad}
		case err != nil:
			if errors.Is(err, io.EOF) && !nonSpace {
				done <- result{status: InputEmpty}
				return
			}
			done <- result{status: InputBad}
		case !isJSONObject(raw):
			done <- result{status: InputBad}
		default:
			var in HookInput
			if err := json.Unmarshal(raw, &in); err != nil {
				done <- result{status: InputBad}
				return
			}
			done <- result{in: in, status: InputOK}
		}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case res := <-done:
		return res.in, res.status
	case <-timer.C:
		if _, nonSpace := cr.state(); nonSpace {
			return HookInput{}, InputBad
		}
		return HookInput{}, InputEmpty
	}
}

// capContext returns s when it fits in max bytes; otherwise s cut at a
// UTF-8 rune start plus the truncation suffix, at most max bytes in all.
func capContext(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max - len(truncatedSuffix)
	if cut < 0 {
		cut = 0
	}
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + truncatedSuffix
}

// SessionStartContext is the session-start text: the two-line preamble, a
// blank line, then the compact context pack wrapped by frame, at most
// MaxAdditionalContextBytes in all (D-11). The caller passes
// app.FrameContext, which says the block is stored data rather than
// instructions and fences it, and app.FrameContextOverhead() as overhead.
// Only the pack is ever cut, before it is framed, so the preamble and both
// frame markers always survive.
func SessionStartContext(pack string, frame func(string) string, overhead int) string {
	head := sessionPreamble + "\n\n"
	return head + frame(capContext(pack, MaxAdditionalContextBytes-len(head)-overhead))
}

// HookScopeEnv names the environment variable that sets which Claude Code
// sessions the hooks speak in (see HookScope).
const HookScopeEnv = "CASEBOOK_HOOK_SCOPE"

// ScopeMarker is the file (or directory) name that opts a project into the
// hooks under the "marked" scope.
const ScopeMarker = ".casebook"

// Hook scopes.
const (
	// ScopeAll: the hooks speak in every Claude Code session (the default).
	ScopeAll = "all"
	// ScopeMarked: the hooks speak only in sessions whose working directory
	// is, or is inside, a directory holding a ScopeMarker.
	ScopeMarked = "marked"
)

// HookScope parses a CASEBOOK_HOOK_SCOPE value: "" or "all" is ScopeAll,
// "marked" is ScopeMarked (case-insensitive, trimmed). Any other value is an
// error; the hooks then print nothing (they fail closed).
func HookScope(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", ScopeAll:
		return ScopeAll, nil
	case ScopeMarked:
		return ScopeMarked, nil
	}
	return "", fmt.Errorf("%s must be %q or %q", HookScopeEnv, ScopeAll, ScopeMarked)
}

// EffectiveHookScope combines the scope a hook command line asks for (its
// --scope flag, which `setup claude --hooks --scope marked` writes; "" when
// absent) with the CASEBOOK_HOOK_SCOPE value env. Either one can narrow the
// hooks to ScopeMarked and neither can widen the other: the result is
// ScopeMarked when either says "marked", else ScopeAll. An invalid value in
// either is an error, so the hooks fail closed.
func EffectiveHookScope(flag, env string) (string, error) {
	fromEnv, err := HookScope(env)
	if err != nil {
		return "", err
	}
	var fromFlag string
	switch strings.ToLower(strings.TrimSpace(flag)) {
	case "", ScopeAll:
		fromFlag = ScopeAll
	case ScopeMarked:
		fromFlag = ScopeMarked
	default:
		return "", fmt.Errorf("--scope must be %q or %q", ScopeAll, ScopeMarked)
	}
	if fromEnv == ScopeMarked || fromFlag == ScopeMarked {
		return ScopeMarked, nil
	}
	return ScopeAll, nil
}

// InScope reports whether a hook may speak in a session working in cwd
// under scope. ScopeAll always may. ScopeMarked may only when cwd is an
// absolute path and it or one of its parents holds a ScopeMarker (anything
// stat finds at that name); an empty or relative cwd is out of scope.
func InScope(scope, cwd string, stat func(string) (os.FileInfo, error)) bool {
	if scope == ScopeAll {
		return true
	}
	if cwd == "" || !filepath.IsAbs(cwd) {
		return false
	}
	dir := filepath.Clean(cwd)
	for {
		if _, err := stat(filepath.Join(dir, ScopeMarker)); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// NoActiveHint is the session-start text when no investigation is active.
func NoActiveHint() string { return noActiveHint }

// CheckpointReminder is the Stop reminder for investigation invRef (D-12).
func CheckpointReminder(invRef string) string {
	return "Casebook: if this session produced material findings for " + invRef +
		" (new evidence, claims, contradicted claims, hypothesis changes, resolved or new questions, conclusion changes), " +
		"persist them now with one casebook_checkpoint call. Skip if nothing material changed."
}

// PreCompactMessage is the PreCompact warning for investigation invRef. It
// is addressed to the user: Claude Code shows a PreCompact systemMessage to
// the user only and gives PreCompact no way to add model context (AR-03).
func PreCompactMessage(invRef string) string {
	return "Casebook: this conversation is being compacted. If it found anything for " + invRef +
		" that is not saved yet, ask Claude to record it with casebook_checkpoint; " +
		"the investigation's context is reloaded after compaction."
}

// hookSpecificOutput is the event-specific part of a hook's JSON output.
type hookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// hookOutput is a hook's JSON output: one object on one line.
type hookOutput struct {
	HookSpecificOutput *hookSpecificOutput `json:"hookSpecificOutput,omitempty"`
	SystemMessage      string              `json:"systemMessage,omitempty"`
}

// encodeHookOutput renders o as one JSON line (no HTML escaping) plus a
// newline.
func encodeHookOutput(o hookOutput) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(o); err != nil {
		// Strings always encode.
		panic("integrations: encode hook output: " + err.Error())
	}
	return b.Bytes()
}

// SessionStartOutput is the SessionStart hook output that adds text to
// Claude's context:
// {"hookSpecificOutput": {"hookEventName": "SessionStart", "additionalContext": text}}.
func SessionStartOutput(text string) []byte {
	return encodeHookOutput(hookOutput{HookSpecificOutput: &hookSpecificOutput{
		HookEventName: "SessionStart", AdditionalContext: text,
	}})
}

// StopOutput is the non-blocking Stop output carrying reminder.
// Verified 2026-10-02 with claude -p (Claude Code 2.1.287): Stop
// hookSpecificOutput.additionalContext reaches the model with no hook error;
// the model answers it in one more turn. systemMessage is shown to the user
// only, and decision "block" reaches the model but raises a "Stop hook error
// occurred" notice, so neither is used.
func StopOutput(reminder string) []byte {
	return encodeHookOutput(hookOutput{HookSpecificOutput: &hookSpecificOutput{
		HookEventName: "Stop", AdditionalContext: reminder,
	}})
}

// PreCompactOutput is the PreCompact output: {"systemMessage": msg}, a
// warning shown to the user, never a decision (blocking would cancel the
// compaction, and blocking auto-compaction can leave a session with no room).
// Nothing a PreCompact hook prints reaches the model: the Claude Code hooks
// reference (checked 2026-10-05 for 2.1.289/2.1.290) supports only
// decision/reason, systemMessage and continue for PreCompact, all shown to
// the user, and no additionalContext. Verified 2026-10-02 with claude -p
// (2.1.287): PreCompact hookSpecificOutput fails Claude Code's output
// validation (a hook error). The Stop reminder is the model-facing
// mechanism; the SessionStart "compact" matcher reloads the pack afterwards.
func PreCompactOutput(msg string) []byte {
	return encodeHookOutput(hookOutput{SystemMessage: msg})
}
