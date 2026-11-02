package integrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jameskbb/casebook/internal/paths"
)

// HookStateFile is the checkpoint hook's state file in the data directory.
const HookStateFile = "hook-state.json"

// maxHookSessions bounds the sessions the state file remembers.
const maxHookSessions = 200

// MaxSessionIDBytes caps the session ids the hooks accept, so the state
// file stays small (a few hundred entries of at most this size each).
const MaxSessionIDBytes = 128

// staleTempAge is how old a leftover state temp file must be before a
// later write removes it.
const staleTempAge = time.Minute

// errSessionID is returned for a session id the hooks do not accept.
var errSessionID = errors.New("hook-state: invalid session id")

// ValidSessionID reports whether id is a session id the hooks use: not
// empty and at most MaxSessionIDBytes bytes. Anything else is treated as no
// session id at all.
func ValidSessionID(id string) bool {
	return id != "" && len(id) <= MaxSessionIDBytes
}

// hookState is the state file: when each session was first seen by a hook,
// which investigation it was last reminded about, and when, and when it
// last checkpointed which investigation. It never holds payload or record
// text (D-12).
type hookState struct {
	Version  int                    `json:"version"`
	Sessions map[string]hookSession `json:"sessions"`
}

// hookSession is one remembered session. FirstSeen is when a hook first ran
// for the session (RFC3339 with nanoseconds, UTC); InvestigationRef and
// RemindedAt are empty until the session gets the Stop reminder, and
// WarnedRef and WarnedAt until it gets the PreCompact warning. The two are
// claimed separately so a user-facing PreCompact warning never uses up the
// Stop reminder that reaches the model (AR-03). CheckpointRef and
// CheckpointAt (RFC3339 with nanoseconds, UTC) are the investigation and
// time of the session's own latest casebook_checkpoint call, recorded by
// the MCP server that session runs (RecordSessionCheckpoint), so the
// checkpoint hook's 10-minute gate counts only this session's checkpoints.
type hookSession struct {
	InvestigationRef string `json:"investigation_ref,omitempty"`
	RemindedAt       string `json:"reminded_at,omitempty"`
	WarnedRef        string `json:"warned_ref,omitempty"`
	WarnedAt         string `json:"warned_at,omitempty"`
	FirstSeen        string `json:"first_seen,omitempty"`
	CheckpointRef    string `json:"checkpoint_ref,omitempty"`
	CheckpointAt     string `json:"checkpoint_at,omitempty"`
}

// lastTouched is the newest of the session's reminder, checkpoint and
// first-seen times; unreadable times count as the zero time (oldest).
func (h hookSession) lastTouched() time.Time {
	var newest time.Time
	for _, v := range []string{h.RemindedAt, h.WarnedAt, h.FirstSeen, h.CheckpointAt} {
		if at, err := time.Parse(time.RFC3339Nano, v); err == nil && at.After(newest) {
			newest = at
		}
	}
	return newest
}

// loadHookState reads the state file; a missing or unparsable file is an
// empty state.
func loadHookState(path string) (hookState, error) {
	s := hookState{Version: 1, Sessions: map[string]hookSession{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("read %s: %w", path, err)
	}
	var loaded hookState
	if json.Unmarshal(data, &loaded) != nil || loaded.Sessions == nil {
		return s, nil
	}
	s.Sessions = loaded.Sessions
	return s, nil
}

// SessionFirstSeen returns when a hook first ran for sessionID, recording
// now as that time when the state has none (or an unreadable one). The
// checkpoint hook reminds only about activity after this time, so a session
// is never asked to checkpoint work done before it started (WR-02). The
// file is written only when the time is recorded.
func SessionFirstSeen(path, sessionID string, now time.Time) (time.Time, error) {
	if !ValidSessionID(sessionID) {
		return time.Time{}, errSessionID
	}
	s, err := loadHookState(path)
	if err != nil {
		return time.Time{}, err
	}
	cur := s.Sessions[sessionID]
	if at, err := time.Parse(time.RFC3339Nano, cur.FirstSeen); err == nil {
		return at, nil
	}
	now = now.UTC()
	cur.FirstSeen = now.Format(time.RFC3339Nano)
	s.Sessions[sessionID] = cur
	if err := saveHookState(path, s); err != nil {
		return time.Time{}, err
	}
	return now, nil
}

// RecordSessionCheckpoint records that sessionID (a Claude Code session id)
// checkpointed invRef at now. The MCP server calls it after a successful
// casebook_checkpoint when Claude Code passes it the session id
// (CLAUDE_CODE_SESSION_ID), so the checkpoint hook can tell this session's
// checkpoints from those of other sessions and clients (SessionCheckpoint).
// It writes only when the state file already exists: the hooks create it,
// so nothing is stored for people who do not use them. The session's other
// fields are kept.
func RecordSessionCheckpoint(path, sessionID, invRef string, now time.Time) error {
	if !ValidSessionID(sessionID) {
		return errSessionID
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	s, err := loadHookState(path)
	if err != nil {
		return err
	}
	cur := s.Sessions[sessionID]
	cur.CheckpointRef = invRef
	cur.CheckpointAt = now.UTC().Format(time.RFC3339Nano)
	s.Sessions[sessionID] = cur
	return saveHookState(path, s)
}

// SessionCheckpoint returns when sessionID last checkpointed invRef, as
// recorded by RecordSessionCheckpoint, and whether it has. A checkpoint of
// another investigation, or an unreadable time, counts as none.
func SessionCheckpoint(path, sessionID, invRef string) (time.Time, bool, error) {
	if !ValidSessionID(sessionID) {
		return time.Time{}, false, errSessionID
	}
	s, err := loadHookState(path)
	if err != nil {
		return time.Time{}, false, err
	}
	cur := s.Sessions[sessionID]
	if cur.CheckpointRef != invRef {
		return time.Time{}, false, nil
	}
	at, err := time.Parse(time.RFC3339Nano, cur.CheckpointAt)
	if err != nil {
		return time.Time{}, false, nil
	}
	return at, true, nil
}

// ClaimReminder records that sessionID is being given the Stop reminder
// about invRef and reports whether it had not been yet: false when the
// session's last reminder was for invRef. The session's first-seen time is
// kept. The state keeps the newest maxHookSessions sessions by their latest
// time (ties broken by session id) and is written atomically with mode
// 0600. A write failure returns false and the error, so a hook that cannot
// remember stays quiet. Two hooks racing on the file can at worst repeat
// one reminder.
func ClaimReminder(path, sessionID, invRef string, now time.Time) (bool, error) {
	return claimOnce(path, sessionID, invRef, now, func(h *hookSession) (*string, *string) {
		return &h.InvestigationRef, &h.RemindedAt
	})
}

// ClaimPreCompactWarning is ClaimReminder for the PreCompact warning: it is
// claimed separately from the Stop reminder, with the same rules.
func ClaimPreCompactWarning(path, sessionID, invRef string, now time.Time) (bool, error) {
	return claimOnce(path, sessionID, invRef, now, func(h *hookSession) (*string, *string) {
		return &h.WarnedRef, &h.WarnedAt
	})
}

// claimOnce records invRef and now in the ref and time fields fields picks
// and reports whether they did not already name invRef.
func claimOnce(path, sessionID, invRef string, now time.Time, fields func(*hookSession) (*string, *string)) (bool, error) {
	if !ValidSessionID(sessionID) {
		return false, errSessionID
	}
	s, err := loadHookState(path)
	if err != nil {
		return false, err
	}
	cur := s.Sessions[sessionID]
	ref, at := fields(&cur)
	if *ref == invRef && *at != "" {
		return false, nil
	}
	*ref = invRef
	*at = now.UTC().Format(time.RFC3339)
	s.Sessions[sessionID] = cur
	if err := saveHookState(path, s); err != nil {
		return false, err
	}
	return true, nil
}

// saveHookState prunes s and writes it to path atomically with mode 0600,
// creating the directory when missing. A failed write removes its temp file
// (writeFileAtomic); temp files left by a hook killed mid-write (Claude
// Code's hook timeout) are removed here once they are staleTempAge old.
func saveHookState(path string, s hookState) error {
	pruneHookSessions(s.Sessions)
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	if err := paths.EnsureDir(dir); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	sweepStaleTemps(path, time.Now())
	return writeFileAtomic(path, data, 0o600)
}

// sweepStaleTemps removes regular files named like writeTemp's temp files
// for path (".<name>.casebook-*" in the same directory) whose modification
// time is more than staleTempAge before now. Errors are ignored: this is
// cleanup only.
func sweepStaleTemps(path string, now time.Time) {
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := "." + filepath.Base(path) + ".casebook-"
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		if fi, err := e.Info(); err == nil && now.Sub(fi.ModTime()) > staleTempAge {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// pruneHookSessions keeps the newest maxHookSessions entries.
func pruneHookSessions(sessions map[string]hookSession) {
	if len(sessions) <= maxHookSessions {
		return
	}
	type entry struct {
		id string
		at time.Time
	}
	all := make([]entry, 0, len(sessions))
	for id, s := range sessions {
		all = append(all, entry{id, s.lastTouched()})
	}
	sort.Slice(all, func(i, j int) bool {
		if !all[i].at.Equal(all[j].at) {
			return all[i].at.After(all[j].at)
		}
		return all[i].id > all[j].id
	})
	for _, e := range all[maxHookSessions:] {
		delete(sessions, e.id)
	}
}
