package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestActorValidate(t *testing.T) {
	got, err := Actor{Kind: ActorAI, Client: " claude-code\n"}.Validate()
	if err != nil || got != (Actor{Kind: ActorAI, Client: "claude-code"}) {
		t.Fatalf("Validate = %+v, %v", got, err)
	}
	cases := []struct {
		actor Actor
		field string
	}{
		{Actor{Kind: "robot", Client: "x"}, "actor.kind"},
		{Actor{Kind: "", Client: "x"}, "actor.kind"},
		{Actor{Kind: ActorHuman, Client: ""}, "actor.client"},
		{Actor{Kind: ActorHuman, Client: "  "}, "actor.client"},
		{Actor{Kind: ActorHuman, Client: strings.Repeat("c", 65)}, "actor.client"},
	}
	for _, c := range cases {
		_, err := c.actor.Validate()
		ve, ok := IsValidation(err)
		if !ok || ve.Field != c.field {
			t.Errorf("%+v.Validate() = %v, want ValidationError on %s", c.actor, err, c.field)
		}
	}
	if _, err := (Actor{Kind: ActorHuman, Client: strings.Repeat("c", 64)}).Validate(); err != nil {
		t.Errorf("64-byte client rejected: %v", err)
	}
}

func TestActorConstructors(t *testing.T) {
	if a := ActorDesktop(); a != (Actor{Kind: ActorHuman, Client: "casebook-ui"}) {
		t.Errorf("ActorDesktop = %+v", a)
	}
	if a := ActorCLI(); a != (Actor{Kind: ActorHuman, Client: "casebook-cli"}) {
		t.Errorf("ActorCLI = %+v", a)
	}
	if a := ActorMCP("codex"); a != (Actor{Kind: ActorAI, Client: "codex"}) {
		t.Errorf("ActorMCP = %+v", a)
	}
}

func TestTimestamps(t *testing.T) {
	loc := time.FixedZone("x", 2*3600)
	if got := FormatTime(time.Date(2026, 10, 1, 12, 0, 0, 5e6, loc)); got != "2026-10-01T10:00:00.005Z" {
		t.Fatalf("FormatTime = %q", got)
	}
	cases := map[string]string{
		"2026-10-01T12:00:00+02:00":      "2026-10-01T10:00:00.000Z",
		"2026-10-01T10:00:00.123456789Z": "2026-10-01T10:00:00.123Z",
		" 2026-10-01T10:00:00Z ":         "2026-10-01T10:00:00.000Z",
		"2026-10-01T05:30:00.5-04:30":    "2026-10-01T10:00:00.500Z",
	}
	for in, want := range cases {
		got, err := ParseTimestamp("captured_at", in)
		if err != nil || got != want {
			t.Errorf("ParseTimestamp(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "yesterday", "2026-10-01", "2026-10-01 10:00:00"} {
		_, err := ParseTimestamp("captured_at", bad)
		ve, ok := IsValidation(err)
		if !ok || ve.Field != "captured_at" {
			t.Errorf("ParseTimestamp(%q) = %v, want ValidationError on captured_at", bad, err)
		}
	}
}

func TestDTOJSONTags(t *testing.T) {
	b, err := json.Marshal(Investigation{})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"id"`, `"ref"`, `"problem_statement"`, `"conclusion_confidence"`, `"conclusion_rationale"`,
		`"created_at"`, `"updated_at"`, `"archived_at"`, `"archived"`, `"is_active"`, `"created_by":{"kind":"","client":""}`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("Investigation JSON %s lacks %s", b, key)
		}
	}
	b, _ = json.Marshal(Activity{})
	for _, key := range []string{`"investigation_ref"`, `"actor"`, `"action"`, `"target_ref"`, `"summary"`, `"note"`, `"created_at"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("Activity JSON %s lacks %s", b, key)
		}
	}
}

func TestActorMCPNormalizesClientName(t *testing.T) {
	long := strings.Repeat("a", 61) + "ééé" // 67 bytes; byte 64 is inside a rune
	cases := []struct{ in, want string }{
		{"claude-code", "claude-code"},
		{"  codex \n", "codex"},
		{"", DefaultMCPClient},
		{" \t\n ", DefaultMCPClient},
		{"\x00\x01\x7f", DefaultMCPClient},
		{"bad\xffutf8", "badutf8"},
		{"two\nlines\tand\x07bell", "two lines andbell"},
		{strings.Repeat("x", 75), strings.Repeat("x", MaxClientBytes)},
		{long, strings.Repeat("a", 61) + "é"},
		// S4: bidi overrides, isolates, zero-width space and C1 controls
		// are dropped; a name made only of them falls back.
		{"claude\u202e-edoc\u202c", "claude-edoc"},
		{"co\u200bdex\u0085", "codex"},
		{"\u2066\u2069\u200b\ufeff\u0085", DefaultMCPClient},
		{"tag\U000E0041\U000E0042", "tag"},
		// A cut inside a flag tag sequence leaves no stray tag character.
		{strings.Repeat("a", 55) + englandFlag, strings.Repeat("a", 55) + "\U0001F3F4"},
	}
	for _, tc := range cases {
		a := ActorMCP(tc.in)
		if a.Kind != ActorAI || a.Client != tc.want {
			t.Errorf("ActorMCP(%q) = %+v, want {ai %q}", tc.in, a, tc.want)
		}
		if v, err := a.Validate(); err != nil || v != a {
			t.Errorf("ActorMCP(%q) does not validate unchanged: %+v, %v", tc.in, v, err)
		}
		if !utf8.ValidString(a.Client) || len(a.Client) > MaxClientBytes {
			t.Errorf("ActorMCP(%q).Client = %q is not a valid client name", tc.in, a.Client)
		}
	}
}

func TestParseTimestampKeepsFixedWidth(t *testing.T) {
	for _, in := range []string{"9999-12-31T23:30:00-01:00", "0000-01-01T00:30:00+01:00", "0000-06-01T00:00:00Z"} {
		if got, err := ParseTimestamp("at", in); err == nil {
			t.Errorf("ParseTimestamp(%q) = %q, want a ValidationError", in, got)
		} else if ve, ok := IsValidation(err); !ok || ve.Field != "at" {
			t.Errorf("ParseTimestamp(%q) error = %v, want a ValidationError on at", in, err)
		}
	}
	for in, want := range map[string]string{
		"9999-12-31T23:30:00+01:00": "9999-12-31T22:30:00.000Z",
		"0001-01-01T00:30:00Z":      "0001-01-01T00:30:00.000Z",
	} {
		if got, err := ParseTimestamp("at", in); err != nil || got != want || len(got) != len(TimeLayout) {
			t.Errorf("ParseTimestamp(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}
