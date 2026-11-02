package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// readResource reads uri and returns its single text content.
func readResource(t *testing.T, cs *mcp.ClientSession, uri string) *mcp.ResourceContents {
	t.Helper()
	res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatalf("read %s: %v", uri, err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("read %s: %d contents, want 1", uri, len(res.Contents))
	}
	c := res.Contents[0]
	if c.MIMEType != "text/markdown" || c.URI != uri {
		t.Fatalf("read %s: content uri %q mime %q", uri, c.URI, c.MIMEType)
	}
	return c
}

func TestResourceActiveContext(t *testing.T) {
	ctx := context.Background()
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	want, err := a.RenderContext(ctx, "", "markdown", app.ContextOptions{Compact: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := readResource(t, cs, activeContextURI).Text; got != app.FrameContext(want) {
		t.Fatalf("active context is not the framed compact pack:\n%s\n---\n%s", got, want)
	}

	empty := apptest.Open(t)
	cs2 := newTestSession(t, empty, sessionOpts{})
	if got := readResource(t, cs2, activeContextURI).Text; !strings.HasPrefix(got, "No active investigation is set") {
		t.Fatalf("no-active text = %q", got)
	}
}

func TestResourceInvestigationContext(t *testing.T) {
	ctx := context.Background()
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	want, err := a.RenderContext(ctx, "INV-0001", "markdown", app.ContextOptions{Compact: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"INV-0001", "inv-1"} {
		uri := "casebook://investigation/" + ref + "/context"
		if got := readResource(t, cs, uri).Text; got != app.FrameContext(want) {
			t.Fatalf("%s is not the framed compact pack", uri)
		}
	}
	for _, uri := range []string{
		"casebook://investigation/INV-0099/context",
		"casebook://investigation/not%20a%20ref/context",
	} {
		if _, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri}); err == nil ||
			!strings.Contains(err.Error(), "not found") {
			t.Fatalf("read %s: err = %v, want resource not found", uri, err)
		}
	}
}

func TestResourceListing(t *testing.T) {
	ctx := context.Background()
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	var uris []string
	for r, err := range cs.Resources(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		uris = append(uris, r.URI)
		if r.Name != "active-context" || r.MIMEType != "text/markdown" || r.Description == "" {
			t.Errorf("resource %+v", r)
		}
	}
	if strings.Join(uris, ",") != activeContextURI {
		t.Fatalf("resources = %v", uris)
	}
	var templates []string
	for r, err := range cs.ResourceTemplates(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		templates = append(templates, r.URITemplate)
		if r.Name != "investigation-context" || r.MIMEType != "text/markdown" || r.Description == "" {
			t.Errorf("template %+v", r)
		}
	}
	if strings.Join(templates, ",") != investigationContextTemplate {
		t.Fatalf("templates = %v", templates)
	}
	// No prompts are registered (D-20).
	if cs.InitializeResult().Capabilities.Prompts != nil {
		t.Fatal("prompts capability advertised")
	}
}
