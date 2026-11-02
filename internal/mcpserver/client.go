package mcpserver

import (
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// knownClients maps the exact (trimmed, case-insensitive) names known
// clients report in clientInfo.name to the name Casebook records.
var knownClients = map[string]string{
	"claude-code":      "claude-code",
	"claude code":      "claude-code",
	"codex":            "codex",
	"codex-mcp-client": "codex",
}

// ClientName returns the client name recorded on writes (D-18). A non-empty
// (after trimming) override wins as given. Otherwise the exact names of
// known clients are mapped (claude-code, codex); any other name passes
// through unchanged for app.ActorMCP to sanitize (empty becomes mcp-client
// there), so an unknown client that merely contains "claude" or "codex" in
// its name is recorded as itself (AR-15).
func ClientName(raw, override string) string {
	if o := strings.TrimSpace(override); o != "" {
		return o
	}
	if known, ok := knownClients[strings.ToLower(strings.TrimSpace(raw))]; ok {
		return known
	}
	return raw
}

// clientInfoName returns the calling client's self-reported name. It works
// for both the initialize handshake and the per-request _meta protocol.
func clientInfoName(req *mcp.CallToolRequest) string {
	if req == nil {
		return ""
	}
	info := req.ClientInfo()
	if info == nil {
		return ""
	}
	return info.Name
}
