package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// sessionOpts configure newTestSession.
type sessionOpts struct {
	// ClientName is the MCP client name (default claude-code).
	ClientName string
	// Protocol forces a protocol version; empty means the SDK default.
	Protocol string
	// Options are the server options (Version "test" when empty).
	Options Options
}

// newTestSession builds a server with New and connects an in-memory client
// to it (server first, D-22).
func newTestSession(t *testing.T, a *app.App, opt sessionOpts) *mcp.ClientSession {
	t.Helper()
	if opt.Options.Version == "" {
		opt.Options.Version = "test"
	}
	return connectServer(t, New(a, opt.Options), opt)
}

// connectServer connects an in-memory client to s.
func connectServer(t *testing.T, s *mcp.Server, opt sessionOpts) *mcp.ClientSession {
	t.Helper()
	if opt.ClientName == "" {
		opt.ClientName = "claude-code"
	}
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	var copts *mcp.ClientSessionOptions
	if opt.Protocol != "" {
		copts = &mcp.ClientSessionOptions{ProtocolVersion: opt.Protocol}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: opt.ClientName, Version: "test"}, nil)
	cs, err := client.Connect(ctx, clientT, copts)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		_ = ss.Wait()
	})
	return cs
}

// seeded returns an App holding the cloud spend fixture.
func seeded(t *testing.T, opts ...app.Option) (*app.App, apptest.Fixture) {
	t.Helper()
	a := apptest.Open(t, opts...)
	return a, apptest.SeedCloudSpend(t, a)
}

// callTool calls a tool and fails the test on a protocol error.
func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: protocol error: %v", name, err)
	}
	return res
}

// decode asserts res succeeded, has no JSON null in its structuredContent and
// decodes the structuredContent into T.
func decode[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(res))
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("structured content is not JSON: %v", err)
	}
	apptest.AssertNoJSONNulls(t, raw)
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("decode structured content %s: %v", b, err)
	}
	return v
}

// resultText joins the text content of a result.
func resultText(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// wantToolError asserts res is a tool error whose text contains substr.
func wantToolError(t *testing.T, res *mcp.CallToolResult, substr string) {
	t.Helper()
	if !res.IsError {
		t.Fatalf("want a tool error containing %q, got success: %s", substr, resultText(res))
	}
	if txt := resultText(res); !strings.Contains(txt, substr) {
		t.Fatalf("tool error %q does not contain %q", txt, substr)
	}
}

// assertToolMeta checks the listed metadata of one tool: a description of at
// most 900 runes without em dashes, an object input schema and an output
// schema.
func assertToolMeta(t *testing.T, cs *mcp.ClientSession, name string) {
	t.Helper()
	ctx := context.Background()
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		if tool.Name != name {
			continue
		}
		if tool.Description == "" {
			t.Errorf("%s: empty description", name)
		}
		if n := utf8.RuneCountInString(tool.Description); n > 900 {
			t.Errorf("%s: description has %d runes, want at most 900", name, n)
		}
		if strings.ContainsRune(tool.Description, '\u2014') {
			t.Errorf("%s: description contains an em dash", name)
		}
		in, ok := tool.InputSchema.(map[string]any)
		if !ok || in["type"] != "object" {
			t.Errorf("%s: input schema %v is not an object schema", name, tool.InputSchema)
		}
		if tool.OutputSchema == nil {
			t.Errorf("%s: no output schema", name)
		}
		if tool.Annotations == nil || tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
			t.Errorf("%s: annotations must set openWorldHint false", name)
		}
		return
	}
	t.Fatalf("tool %s is not listed", name)
}

// syncBuffer is a bytes.Buffer safe for concurrent writes (log handlers run
// on server goroutines).
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
