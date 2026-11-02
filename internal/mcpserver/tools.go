package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
)

// handlers is the state every tool handler shares.
type handlers struct {
	app  *app.App
	opts Options
	log  *slog.Logger
}

// call describes one tool invocation: the tool name and the actor every write
// records.
type call struct {
	Tool  string
	Actor app.Actor
}

// toolDef is one registered tool.
type toolDef struct {
	name string
	add  func(s *mcp.Server, h *handlers)
}

// toolDefs is the tool registry. Each tools_*.go file appends to it from
// init() through registerTool, so later plans add tools without editing
// shared files. New registers them in name order.
var toolDefs []toolDef

// registerTool adds a tool to the registry. Registering a name twice is a
// programmer error and panics at startup.
func registerTool(name string, add func(s *mcp.Server, h *handlers)) {
	for _, d := range toolDefs {
		if d.name == name {
			panic("mcpserver: tool registered twice: " + name)
		}
	}
	toolDefs = append(toolDefs, toolDef{name: name, add: add})
}

// addTool registers a typed tool. The input schema is inferred from In with
// enum values (inputSchema) unless the tool sets one. The output schema is
// only {"type": "object"}: results still carry structuredContent, but the
// full inferred output schemas (about a third of tools/list) cost every
// session context for nothing the model needs before calling (AR-05); the
// field meanings are in the descriptions and docs/mcp-tools.md. The adapter derives
// the actor from the client name, recovers panics, maps errors through
// toolError and enforces the response budget.
//
// Failures go through the handler's error return, never a result with
// IsError set: the SDK would otherwise attach the zero Out value as
// structuredContent.
func addTool[In, Out any](s *mcp.Server, h *handlers, t *mcp.Tool,
	fn func(ctx context.Context, c call, in In) (Out, string, error)) {
	tool := *t
	if tool.InputSchema == nil {
		tool.InputSchema = inputSchema[In]()
	}
	tool.OutputSchema = &jsonschema.Schema{Type: "object"}
	if tool.Annotations != nil {
		ann := *tool.Annotations
		if tool.Title == "" {
			tool.Title = ann.Title
		}
		if ann.Title == "" {
			ann.Title = tool.Title
		}
		tool.Annotations = &ann
	}
	name := tool.Name
	mcp.AddTool(s, &tool, func(ctx context.Context, req *mcp.CallToolRequest, in In) (res *mcp.CallToolResult, out Out, err error) {
		defer func() {
			if p := recover(); p != nil {
				h.log.Error("mcp.tool_panic", "tool", name, "panic", fmt.Sprint(p), "stack", string(debug.Stack()))
				var zero Out
				res, out = nil, zero
				err = toolError(fmt.Errorf("casebook: internal error while running %s; see the server log", name)).GetError()
			}
		}()
		c := call{Tool: name, Actor: app.ActorMCP(ClientName(clientInfoName(req), h.opts.ClientOverride))}
		if x, ok := any(&in).(exactJSONInput); ok && req != nil && req.Params != nil && len(req.Params.Arguments) > 0 {
			if err := x.setExactJSON(req.Params.Arguments); err != nil {
				var zero Out
				return nil, zero, toolError(app.NewValidationError("arguments", "must be a JSON object")).GetError()
			}
		}
		result, text, ferr := fn(ctx, c, in)
		if ferr != nil {
			h.log.Info("mcp.tool_error", "tool", name, "client", c.Actor.Client, "class", errorClass(ferr))
			var zero Out
			return nil, zero, toolError(ferr).GetError()
		}
		if n := resultBytes(text, result); n > MaxResponseBytes {
			h.log.Error("mcp.tool_over_budget", "tool", name, "client", c.Actor.Client, "bytes", n)
			var zero Out
			return nil, zero, errors.New("casebook: result exceeded the 64 KiB response budget; narrow the request (limit, sections, max_bytes)")
		}
		h.log.Debug("mcp.tool", "tool", name, "client", c.Actor.Client)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, result, nil
	})
}

// errorClass names the kind of a tool failure for logs (never its text).
func errorClass(err error) string {
	if _, ok := app.IsValidation(err); ok {
		return "validation"
	}
	switch {
	case errors.Is(err, app.ErrNotFound):
		return "not_found"
	case errors.Is(err, app.ErrNoActiveInvestigation):
		return "no_active"
	case errors.Is(err, app.ErrConflict):
		return "conflict"
	case errors.Is(err, app.ErrBusy):
		return "busy"
	case errors.Is(err, app.ErrInternal):
		return "internal"
	default:
		return "other"
	}
}

func ptr[T any](v T) *T { return &v }

// readOnlyTool annotates a tool that only reads. Set the tool's Title on the
// Tool itself.
func readOnlyTool() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    true,
		IdempotentHint:  true,
		DestructiveHint: ptr(false),
		OpenWorldHint:   ptr(false),
	}
}

// mutatingTool annotates a tool that writes; idempotent says whether
// repeating the same call changes nothing more.
func mutatingTool(idempotent bool) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    false,
		IdempotentHint:  idempotent,
		DestructiveHint: ptr(false),
		OpenWorldHint:   ptr(false),
	}
}
