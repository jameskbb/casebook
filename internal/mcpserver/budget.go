package mcpserver

import (
	"encoding/json"
	"math"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
)

// MaxResponseBytes is the documented budget for one tool result: the JSON of
// the whole result, content plus structuredContent (D-16).
const MaxResponseBytes = 64 * 1024

// Size and list limits (D-15, D-16).
const (
	// responseTarget is what handlers shrink their results to, leaving room
	// under MaxResponseBytes for the JSON-RPC envelope.
	responseTarget = 61440

	// List tools: default and maximum items per call.
	defaultListLimit = 20
	maxListLimit     = 100

	// casebook_get_investigation: default and maximum items per list.
	defaultDetailLimit = 50
	maxDetailLimit     = 200

	// maxRecentActivity caps the activity rows in investigation views.
	maxRecentActivity = 10

	// casebook_get_context: default and maximum Markdown budget. The pack is
	// sent twice (content and structuredContent), so twice the maximum must
	// fit responseTarget.
	defaultContextMaxBytes = 24000
	maxContextMaxBytes     = 28000

	// textLineBytes bounds the one-line record texts in read views.
	textLineBytes = 240
)

// resultBytes is the JSON size of a tool result carrying text as content and
// out as structuredContent.
func resultBytes(text string, out any) int {
	b, err := json.Marshal(&mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: text}},
		StructuredContent: out,
	})
	if err != nil {
		return math.MaxInt
	}
	return len(b)
}

// limitParam applies a list limit parameter: 0 gives def; a negative value or
// one above max is a ValidationError on field (nothing is silently clamped).
func limitParam(field string, v, def, max int) (int, error) {
	if v == 0 {
		return def, nil
	}
	if v < 0 || v > max {
		return 0, app.NewValidationError(field, "must be between 0 and %d", max)
	}
	return v, nil
}

// oneLine renders user text as one line of at most textLineBytes bytes,
// ending in "..." when cut.
func oneLine(s string) string {
	return app.TruncateLine(s, textLineBytes)
}
