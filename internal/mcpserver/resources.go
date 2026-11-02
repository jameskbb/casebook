package mcpserver

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
)

// Context resources (D-20): the compact context pack as Markdown, for the
// active investigation and for a given ref, framed by app.FrameContext as
// stored data rather than instructions. No prompts are registered.
const (
	activeContextURI             = "casebook://active/context"
	investigationContextTemplate = "casebook://investigation/{ref}/context"

	investigationContextPrefix = "casebook://investigation/"
	investigationContextSuffix = "/context"

	noActiveContextText = "No active investigation is set. Call casebook_list_investigations and casebook_set_active_investigation."
)

// registerResources adds the context resources to s.
func registerResources(s *mcp.Server, h *handlers) {
	s.AddResource(&mcp.Resource{
		URI:         activeContextURI,
		Name:        "active-context",
		Title:       "Active investigation context",
		Description: "Compact Markdown context pack of the active investigation (header, conclusion, claims, hypotheses, questions, evidence index); says so when no investigation is active.",
		MIMEType:    "text/markdown",
	}, h.readActiveContext)
	s.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: investigationContextTemplate,
		Name:        "investigation-context",
		Title:       "Investigation context",
		Description: "Compact Markdown context pack of one investigation; ref is an investigation ref such as INV-0001 (case-insensitive).",
		MIMEType:    "text/markdown",
	}, h.readInvestigationContext)
}

func markdownResource(uri, text string) *mcp.ReadResourceResult {
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "text/markdown", Text: text}}}
}

func (h *handlers) readActiveContext(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	text, err := h.app.RenderContext(ctx, "", "markdown", app.ContextOptions{Compact: true})
	switch {
	case errors.Is(err, app.ErrNoActiveInvestigation):
		return markdownResource(uri, noActiveContextText), nil
	case err != nil:
		return nil, errors.New(errorText(err))
	}
	return markdownResource(uri, app.FrameContext(text)), nil
}

func (h *handlers) readInvestigationContext(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	ref, ok := strings.CutPrefix(uri, investigationContextPrefix)
	if ok {
		ref, ok = strings.CutSuffix(ref, investigationContextSuffix)
	}
	if ok {
		var err error
		if ref, err = url.PathUnescape(ref); err != nil {
			ok = false
		}
	}
	// An empty ref would mean the active investigation to the app; the
	// template always names one.
	if !ok || strings.TrimSpace(ref) == "" {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	text, err := h.app.RenderContext(ctx, ref, "markdown", app.ContextOptions{Compact: true})
	if err != nil {
		if _, invalid := app.IsValidation(err); invalid || errors.Is(err, app.ErrNotFound) {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		return nil, errors.New(errorText(err))
	}
	return markdownResource(uri, app.FrameContext(text)), nil
}
