package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
)

// openApp opens an App on a fresh database file and closes it when the test
// ends.
func openApp(t *testing.T) (*app.App, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "casebook.db")
	a, err := app.Open(dbPath)
	if err != nil {
		t.Fatalf("open app: %v", err)
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Errorf("close app: %v", err)
		}
	})
	return a, dbPath
}

// createActive creates an investigation as the desktop actor and makes it
// active.
func createActive(t *testing.T, a *app.App) app.Investigation {
	t.Helper()
	inv, err := a.CreateInvestigation(context.Background(), app.ActorDesktop(), app.CreateInvestigationInput{
		Title:      "September cloud bill up 38%",
		MakeActive: true,
	})
	if err != nil {
		t.Fatalf("create investigation: %v", err)
	}
	return inv
}

// mutationJSON mirrors the structuredContent of a mutating tool.
type mutationJSON struct {
	Ref              string          `json:"ref"`
	InvestigationRef string          `json:"investigation_ref"`
	Action           string          `json:"action"`
	Changed          []string        `json:"changed"`
	Record           json.RawMessage `json:"record"`
}

// callTool calls a tool with a timeout and fails the test on a protocol
// error.
func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return res
}

// decodeStructured round-trips a result's structuredContent into v.
func decodeStructured(t *testing.T, res *mcp.CallToolResult, v any) {
	t.Helper()
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("decode structured content %s: %v", b, err)
	}
}

// resultText joins the text content of a result.
func resultText(res *mcp.CallToolResult) string {
	var s string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			s += tc.Text
		}
	}
	return s
}

// TestMCPSubprocessAddEvidence is the phase tracer: a real `casebook mcp`
// child adds evidence for an MCP client named claude-code, and internal/app
// reads it back from the same file with the AI actor.
func TestMCPSubprocessAddEvidence(t *testing.T) {
	a, dbPath := openApp(t)
	inv := createActive(t, a)

	cs, stderr := connectMCP(t, dbPath, "claude-code", "2025-06-18")
	res := callTool(t, cs, "casebook_add_evidence", map[string]any{
		"title":       "Service chart default log level set to debug",
		"summary":     "The shared service chart now defaults every service to debug logging.",
		"source_type": "code",
	})
	if res.IsError {
		t.Fatalf("add evidence returned a tool error: %s\nstderr:\n%s", resultText(res), stderr.String())
	}
	var got mutationJSON
	decodeStructured(t, res, &got)
	if got.Ref != "E-001" || got.InvestigationRef != inv.Ref || got.Action != "created" {
		t.Fatalf("structured content = %+v, want ref E-001 in %s, action created", got, inv.Ref)
	}

	ev, err := a.GetEvidence(context.Background(), "", "E-001", app.GetEvidenceOptions{})
	if err != nil {
		t.Fatalf("get evidence through app: %v", err)
	}
	if ev.Title != "Service chart default log level set to debug" {
		t.Errorf("title = %q", ev.Title)
	}
	want := app.Actor{Kind: app.ActorAI, Client: "claude-code"}
	if ev.CreatedBy != want {
		t.Errorf("created_by = %+v, want %+v", ev.CreatedBy, want)
	}

	if err := cs.Close(); err != nil {
		t.Fatalf("close session (child exit): %v\nstderr:\n%s", err, stderr.String())
	}
}

// TestMCPStdoutIsProtocolOnly speaks raw newline-delimited JSON-RPC to the
// real binary with debug logging on and checks that every stdout line is a
// JSON-RPC 2.0 message, logs went to stderr, and the process exits 0 when
// stdin closes (D-21).
func TestMCPStdoutIsProtocolOnly(t *testing.T) {
	bin := binary(t)
	cmd := exec.Command(bin, "mcp", "--db", filepath.Join(t.TempDir(), "casebook.db"))
	cmd.Env = childEnv(t, "CASEBOOK_LOG_LEVEL=debug")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	t.Cleanup(func() {
		select {
		case <-exited:
		default:
			_ = cmd.Process.Kill()
		}
	})

	lines := make(chan string, 64)
	readDone := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
		readDone <- sc.Err()
	}()

	var all []string
	responses := map[float64]map[string]any{}
	deadline := time.After(15 * time.Second)
	waitFor := func(ids ...float64) {
		t.Helper()
		for {
			missing := false
			for _, id := range ids {
				if _, ok := responses[id]; !ok {
					missing = true
				}
			}
			if !missing {
				return
			}
			select {
			case line, ok := <-lines:
				if !ok {
					t.Fatalf("stdout closed before responses %v arrived\nstderr:\n%s", ids, stderr.String())
				}
				all = append(all, line)
				var msg map[string]any
				if err := json.Unmarshal([]byte(line), &msg); err == nil {
					if id, ok := msg["id"].(float64); ok {
						responses[id] = msg
					}
				}
			case <-deadline:
				t.Fatalf("timed out waiting for responses %v\nstdout so far:\n%s\nstderr:\n%s",
					ids, strings.Join(all, "\n"), stderr.String())
			}
		}
	}
	send := func(msg string) {
		t.Helper()
		if _, err := io.WriteString(stdin, msg+"\n"); err != nil {
			t.Fatalf("write %s: %v", msg, err)
		}
	}

	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"raw-test","version":"1"}}}`)
	waitFor(1)
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	send(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"casebook_add_evidence","arguments":{"title":"x"}}}`)
	waitFor(2, 3)

	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	for line := range lines {
		all = append(all, line)
	}
	if err := <-readDone; err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		exited <- err
		if err != nil {
			t.Fatalf("casebook mcp exit: %v\nstderr:\n%s", err, stderr.String())
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("casebook mcp did not exit after stdin closed\nstderr:\n%s", stderr.String())
	}

	// Every line must be one JSON-RPC message; a blank line is stray output
	// too (the SDK writes none).
	for i, line := range all {
		if strings.TrimSpace(line) == "" {
			t.Errorf("stdout line %d is blank", i+1)
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Errorf("stdout line %d is not JSON: %q", i+1, line)
			continue
		}
		if msg["jsonrpc"] != "2.0" {
			t.Errorf("stdout line %d is not JSON-RPC 2.0: %q", i+1, line)
		}
	}
	result, _ := responses[3]["result"].(map[string]any)
	if isErr, _ := result["isError"].(bool); !isErr {
		t.Errorf("tools/call with a missing field: response %v, want isError true", responses[3])
	}
	tools, _ := responses[2]["result"].(map[string]any)["tools"].([]any)
	if len(tools) < 4 {
		t.Errorf("tools/list returned %d tools", len(tools))
	}
	if strings.TrimSpace(stderr.String()) == "" {
		t.Error("stderr is empty with CASEBOOK_LOG_LEVEL=debug; logs must go to stderr")
	}
}

// TestMCPClientOverride checks CASEBOOK_CLIENT overrides the client name on
// writes through the real binary.
func TestMCPClientOverride(t *testing.T) {
	a, dbPath := openApp(t)
	createActive(t, a)
	cs, stderr := connectMCP(t, dbPath, "claude-code", "2025-06-18", "CASEBOOK_CLIENT=my-agent")
	res := callTool(t, cs, "casebook_add_evidence", map[string]any{
		"title": "Override probe", "summary": "Written under CASEBOOK_CLIENT.", "source_type": "manual_observation",
	})
	if res.IsError {
		t.Fatalf("add evidence: %s\nstderr:\n%s", resultText(res), stderr.String())
	}
	var got mutationJSON
	decodeStructured(t, res, &got)
	ev, err := a.GetEvidence(context.Background(), "", got.Ref, app.GetEvidenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if want := (app.Actor{Kind: app.ActorAI, Client: "my-agent"}); ev.CreatedBy != want {
		t.Errorf("created_by = %+v, want %+v", ev.CreatedBy, want)
	}
}

// TestBinaryVersion checks the ldflags version reaches `casebook version` and
// the MCP server info.
func TestBinaryVersion(t *testing.T) {
	bin := binary(t)
	cmd := exec.Command(bin, "version")
	cmd.Env = childEnv(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("casebook version: %v", err)
	}
	if string(out) != "casebook "+testVersion+"\n" {
		t.Errorf("version output = %q", out)
	}
	cs, _ := connectMCP(t, filepath.Join(t.TempDir(), "casebook.db"), "codex", "")
	info := cs.InitializeResult()
	if info == nil || info.ServerInfo == nil || info.ServerInfo.Version != testVersion || info.ServerInfo.Name != "casebook" {
		t.Errorf("server info = %+v", info)
	}
}
