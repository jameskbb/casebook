package integrations

import (
	"bytes"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// Codex (0.156 and later) appends tables such as [projects."<path>"] with
// trust_level at the end of config.toml, after the last table but before
// trailing comments. When the Casebook block ends the file, the new table
// lands between [mcp_servers.casebook] and the "# END casebook" line. Setup
// and remove therefore move every table inside the block that is not
// Casebook's out of it, right after the block, instead of rewriting or
// deleting it with the block. Once moved, the block no longer ends the
// file, so later appends land outside it.

// splitForeign separates the tables inside a managed block that are not
// Casebook's. own is the block without them; foreign holds each of them
// (its header and the lines up to the next table header or the END line),
// in order, with their line endings. A header counts as Casebook's when it
// is [mcp_servers.casebook] or a table below it.
func splitForeign(block []byte) (own, foreign []byte) {
	inForeign := false
	for off := 0; off < len(block); {
		line, next := block[off:], len(block)
		if i := bytes.IndexByte(line, '\n'); i >= 0 {
			line, next = line[:i+1], off+i+1
		}
		trimmed := strings.TrimSpace(string(line))
		switch {
		case trimmed == codexBlockEnd:
			inForeign = false
		case strings.HasPrefix(trimmed, "["):
			inForeign = !casebookHeader(trimmed)
		}
		if inForeign {
			foreign = append(foreign, line...)
		} else {
			own = append(own, line...)
		}
		off = next
	}
	return own, foreign
}

// casebookHeader reports whether a table header line names
// mcp_servers.casebook or a table below it, however its keys are quoted.
func casebookHeader(header string) bool {
	if strings.HasPrefix(header, "[[") {
		return false
	}
	var doc map[string]any
	if err := toml.Unmarshal([]byte(header+"\n"), &doc); err != nil {
		return false
	}
	servers, ok := doc[codexServersKey].(map[string]any)
	if !ok || len(doc) != 1 {
		return false
	}
	_, ok = servers[codexServerName]
	return ok && len(servers) == 1
}

// withEOL returns b ending in a line ending (eol added when it has none).
func withEOL(b []byte, eol string) []byte {
	if len(b) == 0 || bytes.HasSuffix(b, []byte("\n")) {
		return b
	}
	return append(append([]byte(nil), b...), eol...)
}

// foreignInBlock reports whether the managed block in data holds tables
// that are not Casebook's.
func foreignInBlock(data []byte) bool {
	span, err := findManagedBlock(data)
	if err != nil || !span.Found {
		return false
	}
	_, foreign := splitForeign(data[span.Start:span.End])
	return len(foreign) > 0
}
