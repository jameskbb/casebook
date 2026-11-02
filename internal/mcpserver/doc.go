package mcpserver

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RenderToolDoc renders docs/mcp-tools.md from the live server: every tool
// with its annotations, description and parameters, the server instructions
// and the resources. It builds the server without an App (no handler runs)
// and lists it through an in-memory client, so the document shows exactly
// what clients see. The output is deterministic.
func RenderToolDoc(ctx context.Context) ([]byte, error) {
	s := New(nil, Options{Version: "doc"})
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, serverT, nil)
	if err != nil {
		return nil, fmt.Errorf("connect server: %w", err)
	}
	defer func() { _ = ss.Close() }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "gendoc", Version: "doc"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		return nil, fmt.Errorf("connect client: %w", err)
	}
	defer func() { _ = cs.Close() }()

	var tools []*mcp.Tool
	for t, err := range cs.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("list tools: %w", err)
		}
		tools = append(tools, t)
	}
	slices.SortFunc(tools, func(x, y *mcp.Tool) int { return strings.Compare(x.Name, y.Name) })
	var resources []*mcp.Resource
	for r, err := range cs.Resources(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("list resources: %w", err)
		}
		resources = append(resources, r)
	}
	slices.SortFunc(resources, func(x, y *mcp.Resource) int { return strings.Compare(x.URI, y.URI) })
	var templates []*mcp.ResourceTemplate
	for r, err := range cs.ResourceTemplates(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("list resource templates: %w", err)
		}
		templates = append(templates, r)
	}
	slices.SortFunc(templates, func(x, y *mcp.ResourceTemplate) int { return strings.Compare(x.URITemplate, y.URITemplate) })

	var b bytes.Buffer
	b.WriteString(`# Casebook MCP tools

The app and the guides call an investigation a case ([ADR 0021](decisions/0021-case-is-the-word.md)); the API keeps the name investigation.

This file is generated from the live tool list; do not edit it by hand.
Regenerate it with ` + "`go run ./internal/mcpserver/gendoc`" + ` from the module root;
` + "`go test ./internal/mcpserver -run TestToolDocUpToDate`" + ` fails when it is stale.

## Connecting

- ` + "`casebook mcp`" + ` serves the tools over stdio. Stdout carries protocol traffic only; logs go to stderr.
- The database is ` + "`--db <file>`" + `, else ` + "`CASEBOOK_DB`" + `, else ` + "`casebook.db`" + ` in the per-user data directory (` + "`CASEBOOK_HOME`" + ` when set).
- ` + "`CASEBOOK_CLIENT`" + ` overrides the client name recorded on writes (by default the name the MCP client reports: Claude Code's and Codex's own names record ` + "`claude-code`" + ` and ` + "`codex`" + `; any other name is recorded as reported).
- ` + "`CASEBOOK_LOG_LEVEL`" + ` sets the stderr log level: debug, info, warn (default) or error.

## Setup and verification

- ` + "`" + `casebook setup claude` + "`" + ` registers casebook in Claude Code's user config (` + "`" + `~/.claude.json` + "`" + `, or ` + "`" + `$CLAUDE_CONFIG_DIR/.claude.json` + "`" + `).
- ` + "`" + `casebook setup claude --hooks` + "`" + ` also adds SessionStart, Stop and PreCompact hooks to ` + "`" + `~/.claude/settings.json` + "`" + ` (or ` + "`" + `$CLAUDE_CONFIG_DIR/settings.json` + "`" + `): session start injects the active investigation's compact context (and reloads it after compaction); Stop reminds the model once per session to call ` + "`" + `casebook_checkpoint` + "`" + `, and PreCompact warns the user (Claude Code shows PreCompact output to the user only).
- ` + "`" + `casebook setup codex` + "`" + ` adds a managed ` + "`" + `[mcp_servers.casebook]` + "`" + ` block to ` + "`" + `~/.codex/config.toml` + "`" + ` (or ` + "`" + `$CODEX_HOME/config.toml` + "`" + `). When it cannot merge safely (for example a hand-written ` + "`" + `[mcp_servers.casebook]` + "`" + `), it changes nothing and prints the config to add by hand.
- ` + "`" + `--print` + "`" + ` shows exactly what would be written without changing anything; ` + "`" + `--remove` + "`" + ` removes only Casebook-owned entries; ` + "`" + `--db <file>` + "`" + ` wires a non-default database into the server command.
- Setup writes only what it owns inside Casebook's entries: ` + "`" + `env` + "`" + ` and other keys you add to ` + "`" + `mcpServers.casebook` + "`" + `, the matcher and timeout of a Casebook hook group, and lines you add inside the Codex block are kept on a rerun.
- Every change first writes a timestamped ` + "`" + `<file>.casebook-backup-<time>` + "`" + ` copy, and rerunning setup is safe: a config that already matches is left untouched.
- Verify Claude Code with ` + "`" + `claude mcp list` + "`" + ` and ` + "`" + `claude mcp get casebook` + "`" + `, and Codex with ` + "`" + `codex mcp list` + "`" + ` and ` + "`" + `codex mcp get casebook --json` + "`" + `.
- ` + "`" + `casebook mcp --self-test` + "`" + ` starts the server in a child process, completes the initialize handshake and tools/list, and prints ok with the tool count (exit 1 on failure).
- ` + "`" + `casebook doctor` + "`" + ` checks the data directory, database integrity, migrations, write access, the MCP handshake, the CLI path, whether the desktop app next to the CLI is the same version, and the Claude Code and Codex config. It never creates the database; opening an existing one applies pending schema migrations (after a pre-upgrade backup), as every casebook command does. It prints only paths, counts and statuses (never record text or secrets), supports ` + "`" + `--json` + "`" + `, and exits 1 when a check fails.

## Conventions

- Refs: investigations ` + "`INV-0001`" + `; evidence ` + "`E-001`" + `, claims ` + "`C-001`" + `, hypotheses ` + "`H-001`" + `, questions ` + "`Q-001`" + `; qualified ` + "`INV-0002/E-001`" + `. Refs are case-insensitive.
- ` + "`investigation_ref`" + ` omitted or empty means the active investigation on every tool that reads or writes one investigation; when none is active the call fails and says how to set one. ` + "`casebook_search_investigations`" + ` without it searches all investigations, and ` + "`casebook_set_active_investigation`" + ` requires it.
- Every write records the actor ` + "`{ai, <client>}`" + ` with the client name from the initialize handshake.
- Errors are tool results with ` + "`isError: true`" + ` and a short text (for example ` + "`invalid source_type: must be one of ...`" + ` or ` + "`not found: ...`" + `), never protocol errors.
- Response budget: every tool result, text plus structured content, stays under 64 KiB. Lists default to 20 items (max 100) and report ` + "`truncated`" + ` and ` + "`total`" + `; ` + "`casebook_get_investigation`" + ` lists default to 50 items (max 200) and shrink to fit; ` + "`casebook_get_context`" + ` and the raw detail window of ` + "`casebook_get_evidence`" + ` shrink to fit. When that is not enough, long text fields are cut and named in ` + "`truncated_fields`" + ` (` + "`casebook_get_evidence`" + ` also cuts its link lists and keeps ` + "`claim_links_total`" + ` and ` + "`hypothesis_links_total`" + `). Search counts at most 100 hits and the investigation list 1000; at that cap ` + "`total_is_lower_bound`" + ` is true.

## Server instructions

`)
	if init := cs.InitializeResult(); init != nil {
		for i, line := range strings.Split(strings.TrimSpace(init.Instructions), "\n") {
			if i > 0 {
				b.WriteString(">\n")
			}
			fmt.Fprintf(&b, "> %s\n", line)
		}
	}

	fmt.Fprintf(&b, "\n## Tools (%d)\n", len(tools))
	for _, t := range tools {
		writeToolDoc(&b, t)
	}

	b.WriteString("\n## Resources\n\nBoth return the compact Markdown context pack (`text/markdown`); reading them writes nothing.\n\n")
	b.WriteString("| URI | Name | Description |\n|---|---|---|\n")
	for _, r := range resources {
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", r.URI, cell(r.Name), cell(r.Description))
	}
	for _, r := range templates {
		fmt.Fprintf(&b, "| `%s` (template) | %s | %s |\n", r.URITemplate, cell(r.Name), cell(r.Description))
	}
	return b.Bytes(), nil
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func writeToolDoc(b *bytes.Buffer, t *mcp.Tool) {
	fmt.Fprintf(b, "\n### %s\n\n", t.Name)
	if t.Title != "" {
		fmt.Fprintf(b, "**%s**\n\n", t.Title)
	}
	if a := t.Annotations; a != nil {
		destructive, openWorld := true, true // spec defaults when unset
		if a.DestructiveHint != nil {
			destructive = *a.DestructiveHint
		}
		if a.OpenWorldHint != nil {
			openWorld = *a.OpenWorldHint
		}
		fmt.Fprintf(b, "Read-only: %s. Idempotent: %s. Destructive: %s. Open world: %s.\n\n",
			yesNo(a.ReadOnlyHint), yesNo(a.IdempotentHint), yesNo(destructive), yesNo(openWorld))
	}
	fmt.Fprintf(b, "%s\n\n", strings.TrimSpace(t.Description))
	schema, _ := t.InputSchema.(map[string]any)
	var rows []string
	paramRows(&rows, schema, "")
	if len(rows) == 0 {
		b.WriteString("No parameters.\n")
		return
	}
	b.WriteString("| Parameter | Type | Required | Values | Description |\n|---|---|---|---|---|\n")
	for _, r := range rows {
		b.WriteString(r)
	}
}

// paramRows appends one table row per property of the object schema, the
// required ones first and then the optional ones, each in name order, with
// the properties of nested objects and array items after their parent
// (links[].relation, conclusion.confidence).
func paramRows(rows *[]string, schema map[string]any, prefix string) {
	props, _ := schema["properties"].(map[string]any)
	required := map[string]bool{}
	for _, r := range asList(schema["required"]) {
		if s, ok := r.(string); ok {
			required[s] = true
		}
	}
	names := make([]string, 0, len(props))
	for n := range props {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if required[names[i]] != required[names[j]] {
			return required[names[i]]
		}
		return names[i] < names[j]
	})
	for _, n := range names {
		p, _ := props[n].(map[string]any)
		name := prefix + n
		desc, _ := p["description"].(string)
		*rows = append(*rows, fmt.Sprintf("| `%s` | %s | %s | %s | %s |\n",
			name, cell(schemaType(p)), yesNo(required[n]), cell(enumText(p)), cell(desc)))
		if hasType(p, "object") && p["properties"] != nil {
			paramRows(rows, p, name+".")
		}
		if items, ok := p["items"].(map[string]any); ok {
			if hasType(items, "object") && items["properties"] != nil {
				paramRows(rows, items, name+"[].")
			}
		}
	}
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

// types lists the JSON types of a schema without null.
func types(p map[string]any) []string {
	var out []string
	if s, ok := p["type"].(string); ok {
		out = append(out, s)
	}
	for _, t := range asList(p["types"]) {
		if s, ok := t.(string); ok {
			out = append(out, s)
		}
	}
	for _, t := range asList(p["type"]) {
		if s, ok := t.(string); ok {
			out = append(out, s)
		}
	}
	return slices.DeleteFunc(out, func(s string) bool { return s == "null" })
}

func hasType(p map[string]any, t string) bool {
	return slices.Contains(types(p), t)
}

// schemaType is the Type cell: string, integer, boolean, object, or
// "array of <item type>".
func schemaType(p map[string]any) string {
	ts := types(p)
	if len(ts) == 0 {
		return "any"
	}
	if slices.Contains(ts, "array") {
		if items, ok := p["items"].(map[string]any); ok {
			return "array of " + schemaType(items)
		}
	}
	return strings.Join(ts, " or ")
}

// enumText is the Values cell: the enum values (null omitted), also of
// array items.
func enumText(p map[string]any) string {
	enum := asList(p["enum"])
	if items, ok := p["items"].(map[string]any); ok && len(enum) == 0 {
		enum = asList(items["enum"])
	}
	var vals []string
	for _, v := range enum {
		if v != nil {
			vals = append(vals, fmt.Sprint(v))
		}
	}
	return strings.Join(vals, ", ")
}

// cell makes text safe for one Markdown table cell.
func cell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.ReplaceAll(s, "|", `\|`)
}
