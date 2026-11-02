package mcpserver

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func TestInstructions(t *testing.T) {
	for _, want := range []string{"INV-0001", "investigation_ref", "casebook_get_context", "casebook_checkpoint", "64 KiB",
		// When to use Casebook, not only how (scripts/agent-eval).
		"casebook_create_investigation", "only after they say yes", "disproven", "source_ref", "human_verified",
		// Handing a case to a note-taking or document app through the
		// client's own tools.
		"casebook_get_report (mode brief or report)", "tools this session actually has", "never sends it anywhere",
		"verification labels", "omission notices exactly", "read the intended note first", "truncated is true",
		"offer the text to copy or save", "reported success", "casebook_add_evidence"} {
		if !strings.Contains(instructions, want) {
			t.Errorf("instructions do not mention %q", want)
		}
	}
	if strings.ContainsRune(instructions, '\u2014') {
		t.Error("instructions contain an em dash")
	}
	// No third-party tool names or provider, and nothing that implies
	// Casebook updates anything outside its database.
	for _, banned := range []string{"Evernote", "evernote", "get_note", "edit_note", "create_note", "sync"} {
		if strings.Contains(instructions, banned) || strings.Contains(getReportDescription, banned) {
			t.Errorf("instructions or the report tool description mention %q", banned)
		}
	}
	// Every MCP session pays for these bytes. 2 KiB kept them near 500
	// tokens; the handoff to a note-taking or document app adds about 700
	// bytes, because an agent must know the rules before it touches a
	// person's note, and the tool descriptions load only on demand.
	if len(instructions) > 2700 {
		t.Errorf("instructions are %d bytes, want at most 2700", len(instructions))
	}
}

func TestServerInfo(t *testing.T) {
	cs := newTestSession(t, apptest.Open(t), sessionOpts{Options: Options{Version: "v9.9.9"}})
	res := cs.InitializeResult()
	if res == nil || res.ServerInfo == nil {
		t.Fatal("no initialize result")
	}
	if res.ServerInfo.Name != "casebook" || res.ServerInfo.Version != "v9.9.9" {
		t.Errorf("server info = %+v", res.ServerInfo)
	}
	if !strings.Contains(res.Instructions, "casebook_checkpoint") {
		t.Errorf("instructions not sent: %q", res.Instructions)
	}
}

func TestToolRegistrySorted(t *testing.T) {
	cs := newTestSession(t, apptest.Open(t), sessionOpts{})
	var names []string
	for tool, err := range cs.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, tool.Name)
	}
	for _, want := range []string{"casebook_add_evidence", "casebook_get_active_investigation", "casebook_get_context", "casebook_get_investigation"} {
		found := false
		for _, n := range names {
			found = found || n == want
		}
		if !found {
			t.Errorf("tool %s not listed in %v", want, names)
		}
	}
}

func TestPanicRecovery(t *testing.T) {
	a := apptest.Open(t)
	logs := &syncBuffer{}
	opts := Options{Version: "test", Logger: slog.New(slog.NewTextHandler(logs, nil))}
	s := New(a, opts)
	h := &handlers{app: a, opts: opts, log: opts.Logger}
	type empty struct{}
	addTool(s, h, &mcp.Tool{Name: "test_panic", Description: "panics", Annotations: readOnlyTool()},
		func(context.Context, call, empty) (empty, string, error) {
			panic("boom")
		})
	cs := connectServer(t, s, sessionOpts{})

	res := callTool(t, cs, "test_panic", nil)
	wantToolError(t, res, "internal error while running test_panic")
	if strings.Contains(resultText(res), "goroutine") || strings.Contains(resultText(res), "boom") {
		t.Errorf("tool error leaks panic detail: %q", resultText(res))
	}
	if res.StructuredContent != nil {
		t.Errorf("error result carries structured content %v", res.StructuredContent)
	}
	if !strings.Contains(logs.String(), "mcp.tool_panic") {
		t.Errorf("panic not logged: %s", logs.String())
	}
	next := callTool(t, cs, "casebook_get_active_investigation", nil)
	if next.IsError {
		t.Fatalf("server stopped serving after a panic: %s", resultText(next))
	}
}

func TestLogsCarryNoContent(t *testing.T) {
	logs := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	a, _ := seeded(t, app.WithLogger(logger))
	cs := newTestSession(t, a, sessionOpts{Options: Options{Logger: logger}})

	secrets := map[string]any{
		"title":       "Log hygiene probe",
		"summary":     "SECRET-SUMMARY-7f3a",
		"raw_detail":  "SECRET-RAW-7f3a",
		"source_type": "code",
	}
	if res := callTool(t, cs, "casebook_add_evidence", secrets); res.IsError {
		t.Fatalf("add evidence: %s", resultText(res))
	}
	failing := map[string]any{"investigation_ref": "INV-0099"}
	for k, v := range secrets {
		failing[k] = v
	}
	wantToolError(t, callTool(t, cs, "casebook_add_evidence", failing), "not found")
	bad := map[string]any{"source_type": "bogus"}
	for k, v := range secrets {
		if k != "source_type" {
			bad[k] = v
		}
	}
	wantToolError(t, callTool(t, cs, "casebook_add_evidence", bad), "source_type")

	out := logs.String()
	if out == "" {
		t.Fatal("debug logger recorded nothing")
	}
	for _, secret := range []string{"SECRET-SUMMARY-7f3a", "SECRET-RAW-7f3a"} {
		if strings.Contains(out, secret) {
			t.Errorf("log contains %q:\n%s", secret, out)
		}
	}
	if !strings.Contains(out, "casebook_add_evidence") {
		t.Errorf("log does not name the tool:\n%s", out)
	}
}

// newestActor returns the actor of the newest activity row.
func newestActor(t *testing.T, a *app.App) app.Actor {
	t.Helper()
	rows, err := a.ListActivity(context.Background(), app.ActivityFilter{Limit: 1})
	if err != nil || len(rows) != 1 {
		t.Fatalf("list activity: %v (%d rows)", err, len(rows))
	}
	return rows[0].Actor
}

func TestActor(t *testing.T) {
	cases := []struct {
		name string
		opt  sessionOpts
		want app.Actor
	}{
		{"default protocol", sessionOpts{ClientName: "claude-code"}, app.ActorMCP("claude-code")},
		{"2025-06-18 handshake", sessionOpts{ClientName: "Claude Code", Protocol: "2025-06-18"}, app.ActorMCP("claude-code")},
		{"codex", sessionOpts{ClientName: "codex-mcp-client", Protocol: "2025-06-18"}, app.ActorMCP("codex")},
		{"override", sessionOpts{ClientName: "claude-code", Options: Options{ClientOverride: "my-agent"}}, app.ActorMCP("my-agent")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, _ := seeded(t)
			cs := newTestSession(t, a, c.opt)
			if res := callTool(t, cs, "casebook_add_evidence", evidenceArgs(nil)); res.IsError {
				t.Fatalf("add evidence: %s", resultText(res))
			}
			if got := newestActor(t, a); got != c.want {
				t.Errorf("activity actor = %+v, want %+v", got, c.want)
			}
			if c.want.Kind != app.ActorAI {
				t.Errorf("want kind ai")
			}
		})
	}
}
