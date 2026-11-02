package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// TimeLayout is the fixed-width UTC timestamp format used for every stored and
// returned timestamp. It is valid RFC3339 and sorts as text in time order.
const TimeLayout = "2006-01-02T15:04:05.000Z"

// FormatTime renders t in UTC with TimeLayout.
func FormatTime(t time.Time) string {
	return t.UTC().Format(TimeLayout)
}

// Actor identifies who performed a mutation.
type Actor struct {
	Kind   ActorKind `json:"kind"`
	Client string    `json:"client"`
}

// Validate returns a sanitized copy of the actor, or a *ValidationError on
// field actor.kind or actor.client.
func (a Actor) Validate() (Actor, error) {
	if err := a.Kind.Validate("actor.kind"); err != nil {
		return Actor{}, err
	}
	client, err := SanitizeLine("actor.client", a.Client, MaxClientBytes, true)
	if err != nil {
		return Actor{}, err
	}
	return Actor{Kind: a.Kind, Client: client}, nil
}

// ActorDesktop is the actor the desktop app uses: {human, casebook-ui}.
func ActorDesktop() Actor { return Actor{Kind: ActorHuman, Client: "casebook-ui"} }

// ActorCLI is the actor the CLI uses: {human, casebook-cli}.
func ActorCLI() Actor { return Actor{Kind: ActorHuman, Client: "casebook-cli"} }

// DefaultMCPClient is the client name ActorMCP uses when the MCP client sent
// no usable name.
const DefaultMCPClient = "mcp-client"

// ActorMCP is the actor an MCP session uses: {ai, <client name from the MCP
// initialize request>}. The name is untrusted, so it is normalized to always
// pass Actor.Validate: invalid UTF-8, control characters and invisible
// format characters (bidi overrides, zero-width space) are dropped,
// newlines and tabs collapse to a space, surrounding whitespace is trimmed,
// and the result is cut to MaxClientBytes at a rune boundary. An empty result
// becomes DefaultMCPClient.
func ActorMCP(client string) Actor {
	return Actor{Kind: ActorAI, Client: normalizeClient(client, DefaultMCPClient)}
}

// normalizeClient cleans a free client name like SanitizeLine, cuts it to
// MaxClientBytes at a rune boundary (no ellipsis, so the result stays a
// prefix of the name), and returns fallback when nothing is left.
func normalizeClient(name, fallback string) string {
	s := strings.ToValidUTF8(name, "")
	s = stripControls(collapseLine(s))
	s = strings.TrimSpace(s)
	if len(s) > MaxClientBytes {
		cut := MaxClientBytes
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		// The cut can split a flag tag sequence; strip again so no stray
		// tag characters survive.
		s = strings.TrimSpace(stripControls(s[:cut]))
	}
	if s == "" {
		return fallback
	}
	return s
}

// ParseTimestamp accepts an RFC3339 or RFC3339Nano timestamp with any offset
// and returns it in UTC TimeLayout (millisecond precision). The UTC year must
// be 0001 to 9999, so the result always has the fixed width that makes
// timestamps sort as text (an offset can push 9999-12-31 or 0000-01-01 past
// that range). Failures are a *ValidationError on field.
func ParseTimestamp(field, s string) (string, error) {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(s))
	if err != nil {
		return "", NewValidationError(field, "must be an RFC3339 timestamp such as 2026-10-01T12:00:00Z")
	}
	if y := t.UTC().Year(); y < 1 || y > 9999 {
		return "", NewValidationError(field, "must fall between years 0001 and 9999 in UTC")
	}
	return FormatTime(t), nil
}

// Activity is one row of the activity log.
type Activity struct {
	ID               int64  `json:"id"`
	InvestigationRef string `json:"investigation_ref"` // "" for global rows
	Actor            Actor  `json:"actor"`
	Action           string `json:"action"`
	TargetRef        string `json:"target_ref"`
	Summary          string `json:"summary"`
	Note             string `json:"note"`
	CreatedAt        string `json:"created_at"`
	// Imported is true for historical rows copied from an imported file:
	// the actor and summary are what the file claimed (AR-09).
	Imported bool `json:"imported"`
}

// ActivityFilter selects activity rows. An empty InvestigationRef means all
// investigations (not the active one). AfterID > 0 keeps rows with a larger
// id and returns them oldest first (a forward cursor); AfterID 0 returns the
// newest rows newest first. Limit 0 means 50; a negative value or one above
// 500 is a ValidationError on limit.
type ActivityFilter struct {
	InvestigationRef string `json:"investigation_ref"`
	AfterID          int64  `json:"after_id"`
	Limit            int    `json:"limit"`
}
