package integrations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// hookSessionOnDisk is one session entry of the state file.
type hookSessionOnDisk struct {
	InvestigationRef string `json:"investigation_ref"`
	RemindedAt       string `json:"reminded_at"`
	WarnedRef        string `json:"warned_ref"`
	WarnedAt         string `json:"warned_at"`
	FirstSeen        string `json:"first_seen"`
}

// hookStateOnDisk reads the state file.
func hookStateOnDisk(t *testing.T, path string) map[string]hookSessionOnDisk {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Version  int                          `json:"version"`
		Sessions map[string]hookSessionOnDisk `json:"sessions"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("state file: %v\n%s", err, data)
	}
	if s.Version != 1 {
		t.Errorf("version = %d, want 1", s.Version)
	}
	return s.Sessions
}

func TestClaimReminder(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	t.Run("once per session and ref", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "data", HookStateFile)
		claim := func(session, ref string) bool {
			t.Helper()
			ok, err := ClaimReminder(path, session, ref, now)
			if err != nil {
				t.Fatal(err)
			}
			return ok
		}
		if !claim("s1", "INV-0001") {
			t.Error("first claim = false, want true")
		}
		wantMode(t, path, 0o600)
		if claim("s1", "INV-0001") {
			t.Error("repeat claim = true, want false")
		}
		if !claim("s1", "INV-0002") {
			t.Error("same session, other ref = false, want true")
		}
		if !claim("s2", "INV-0002") {
			t.Error("other session = false, want true")
		}
		got := hookStateOnDisk(t, path)
		if got["s1"].InvestigationRef != "INV-0002" || got["s1"].RemindedAt != "2026-10-02T12:00:00Z" {
			t.Errorf("s1 = %+v", got["s1"])
		}
	})
	t.Run("prunes to the newest 200", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), HookStateFile)
		for i := 0; i < 205; i++ {
			if _, err := ClaimReminder(path, fmt.Sprintf("s%03d", i), "INV-0001", now.Add(time.Duration(i)*time.Second)); err != nil {
				t.Fatal(err)
			}
		}
		got := hookStateOnDisk(t, path)
		if len(got) != maxHookSessions {
			t.Fatalf("%d sessions, want %d", len(got), maxHookSessions)
		}
		for i := 0; i < 5; i++ {
			if _, ok := got[fmt.Sprintf("s%03d", i)]; ok {
				t.Errorf("s%03d kept, want pruned (oldest)", i)
			}
		}
		if _, ok := got["s204"]; !ok {
			t.Error("newest session pruned")
		}
	})
	t.Run("corrupt file is treated as empty", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), HookStateFile)
		if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
			t.Fatal(err)
		}
		ok, err := ClaimReminder(path, "s1", "INV-0001", now)
		if err != nil || !ok {
			t.Fatalf("claim = %v, %v; want true", ok, err)
		}
		if got := hookStateOnDisk(t, path); len(got) != 1 {
			t.Errorf("sessions = %v, want just s1", got)
		}
	})
	t.Run("write error", func(t *testing.T) {
		dir := t.TempDir()
		blocker := filepath.Join(dir, "file")
		if err := os.WriteFile(blocker, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		ok, err := ClaimReminder(filepath.Join(blocker, HookStateFile), "s1", "INV-0001", now)
		if ok || err == nil {
			t.Errorf("claim under a file = %v, %v; want false and an error", ok, err)
		}
	})
}

// TestSessionCheckpoint: RecordSessionCheckpoint writes only into an
// existing state file, keeps the session's other fields, and
// SessionCheckpoint returns the time only for the same session and
// investigation.
func TestSessionCheckpoint(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 123456789, time.UTC)
	path := filepath.Join(t.TempDir(), HookStateFile)
	if err := RecordSessionCheckpoint(path, "s1", "INV-0001", now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("recorded a checkpoint without a state file (%v)", err)
	}
	if _, ok, err := SessionCheckpoint(path, "s1", "INV-0001"); ok || err != nil {
		t.Errorf("no state file: ok %v, err %v; want none", ok, err)
	}
	if _, err := SessionFirstSeen(path, "s1", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := RecordSessionCheckpoint(path, "s1", "INV-0001", now); err != nil {
		t.Fatal(err)
	}
	wantMode(t, path, 0o600)
	got, ok, err := SessionCheckpoint(path, "s1", "INV-0001")
	if err != nil || !ok || !got.Equal(now) {
		t.Errorf("SessionCheckpoint = %v, %v, %v; want %v", got, ok, err, now)
	}
	if first, err := SessionFirstSeen(path, "s1", now); err != nil || !first.Equal(now.Add(-time.Hour)) {
		t.Errorf("first seen after a checkpoint = %v, %v; want it kept", first, err)
	}
	if _, ok, _ := SessionCheckpoint(path, "s1", "INV-0002"); ok {
		t.Error("checkpoint of INV-0001 counted for INV-0002")
	}
	if _, ok, _ := SessionCheckpoint(path, "s2", "INV-0001"); ok {
		t.Error("checkpoint of s1 counted for s2")
	}
	// A checkpoint of another investigation replaces the record.
	if err := RecordSessionCheckpoint(path, "s1", "INV-0002", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := SessionCheckpoint(path, "s1", "INV-0001"); ok {
		t.Error("INV-0001 still recorded after a checkpoint of INV-0002")
	}
	if err := RecordSessionCheckpoint(path, "", "INV-0001", now); err == nil {
		t.Error("empty session id accepted")
	}
	if _, _, err := SessionCheckpoint(path, strings.Repeat("x", MaxSessionIDBytes+1), "INV-0001"); err == nil {
		t.Error("over-long session id accepted")
	}
}

func TestSessionFirstSeen(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 123456789, time.UTC)
	path := filepath.Join(t.TempDir(), HookStateFile)
	got, err := SessionFirstSeen(path, "s1", now)
	if err != nil || !got.Equal(now) {
		t.Fatalf("first call = %v, %v; want %v", got, err, now)
	}
	wantMode(t, path, 0o600)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Later calls return the recorded time and write nothing.
	got, err = SessionFirstSeen(path, "s1", now.Add(time.Hour))
	if err != nil || !got.Equal(now) {
		t.Fatalf("second call = %v, %v; want %v", got, err, now)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Error("a repeat call rewrote the state file")
	}
	// A reminder keeps the first-seen time.
	if ok, err := ClaimReminder(path, "s1", "INV-0001", now.Add(time.Minute)); err != nil || !ok {
		t.Fatalf("claim = %v, %v; want true", ok, err)
	}
	s1 := hookStateOnDisk(t, path)["s1"]
	if s1.FirstSeen != "2026-10-02T12:00:00.123456789Z" || s1.InvestigationRef != "INV-0001" || s1.RemindedAt != "2026-10-02T12:01:00Z" {
		t.Errorf("s1 = %+v", s1)
	}
	if ok, err := ClaimReminder(path, "s1", "INV-0001", now.Add(2*time.Minute)); err != nil || ok {
		t.Errorf("repeat claim = %v, %v; want false", ok, err)
	}
	// A session first seen only at its reminder (an older state file) gets
	// a first-seen time on the next call.
	if _, err := ClaimReminder(path, "s2", "INV-0001", now); err != nil {
		t.Fatal(err)
	}
	if got, err := SessionFirstSeen(path, "s2", now.Add(time.Hour)); err != nil || !got.Equal(now.Add(time.Hour)) {
		t.Errorf("s2 first seen = %v, %v; want %v", got, err, now.Add(time.Hour))
	}
}

func TestHookStateSessionIDCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), HookStateFile)
	long := strings.Repeat("s", MaxSessionIDBytes+1)
	if ValidSessionID("") || ValidSessionID(long) || !ValidSessionID(long[1:]) {
		t.Error("ValidSessionID does not enforce 1 to MaxSessionIDBytes bytes")
	}
	if _, err := SessionFirstSeen(path, long, time.Now()); err == nil {
		t.Error("SessionFirstSeen accepted an oversize session id")
	}
	if ok, err := ClaimReminder(path, long, "INV-0001", time.Now()); ok || err == nil {
		t.Error("ClaimReminder accepted an oversize session id")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("an oversize session id wrote %s", path)
	}
}

func TestHookStateSweepsStaleTemps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, HookStateFile)
	stale := filepath.Join(dir, "."+HookStateFile+".casebook-111")
	fresh := filepath.Join(dir, "."+HookStateFile+".casebook-222")
	other := filepath.Join(dir, ".other.json.casebook-333")
	old := time.Now().Add(-2 * staleTempAge)
	for _, p := range []string{stale, fresh, other} {
		if err := os.WriteFile(p, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{stale, other} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := SessionFirstSeen(path, "s1", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("a stale hook-state temp file was kept")
	}
	for _, p := range []string{fresh, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was removed: %v", filepath.Base(p), err)
		}
	}
}

// TestClaimPreCompactWarning proves the PreCompact warning and the Stop
// reminder are claimed independently, each once per session and
// investigation (AR-03).
func TestClaimPreCompactWarning(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), HookStateFile)
	if ok, err := ClaimPreCompactWarning(path, "s1", "INV-0001", now); err != nil || !ok {
		t.Fatalf("first warning = %v, %v; want true", ok, err)
	}
	if ok, err := ClaimPreCompactWarning(path, "s1", "INV-0001", now); err != nil || ok {
		t.Errorf("repeat warning = %v, %v; want false", ok, err)
	}
	if ok, err := ClaimReminder(path, "s1", "INV-0001", now); err != nil || !ok {
		t.Errorf("Stop reminder after the warning = %v, %v; want true", ok, err)
	}
	if ok, err := ClaimPreCompactWarning(path, "s1", "INV-0002", now); err != nil || !ok {
		t.Errorf("warning for another investigation = %v, %v; want true", ok, err)
	}
	s1 := hookStateOnDisk(t, path)["s1"]
	if s1.WarnedRef != "INV-0002" || s1.WarnedAt != "2026-10-05T12:00:00Z" || s1.InvestigationRef != "INV-0001" {
		t.Errorf("s1 = %+v", s1)
	}
	if ok, err := ClaimPreCompactWarning(path, strings.Repeat("s", MaxSessionIDBytes+1), "INV-0001", now); ok || err == nil {
		t.Error("ClaimPreCompactWarning accepted an oversize session id")
	}
}
