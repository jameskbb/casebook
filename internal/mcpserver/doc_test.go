package mcpserver

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateDoc = flag.Bool("update", false, "rewrite docs/mcp-tools.md")

var toolDocPath = filepath.Join("..", "..", "docs", "mcp-tools.md")

func TestToolDocUpToDate(t *testing.T) {
	ctx := context.Background()
	got, err := RenderToolDoc(ctx)
	if err != nil {
		t.Fatal(err)
	}
	again, err := RenderToolDoc(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, again) {
		t.Fatal("RenderToolDoc is not deterministic")
	}
	doc := string(got)
	for name := range toolSpecs {
		if !strings.Contains(doc, "### "+name+"\n") {
			t.Errorf("doc lacks a section for %s", name)
		}
	}
	for _, want := range []string{"64 KiB", activeContextURI, investigationContextTemplate, "## Tools (21)",
		"go run ./internal/mcpserver/gendoc", "`links[].relation`", "`claims[].links[].evidence_ref`", "`conclusion.confidence`"} {
		if !strings.Contains(doc, want) {
			t.Errorf("doc lacks %q", want)
		}
	}
	if strings.ContainsRune(doc, '\u2014') {
		t.Error("doc contains an em dash")
	}
	if !strings.HasSuffix(doc, "\n") || strings.HasSuffix(doc, "\n\n") {
		t.Error("doc must end with exactly one newline")
	}

	if *updateDoc {
		if err := os.WriteFile(toolDocPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	committed, err := os.ReadFile(toolDocPath)
	if err != nil {
		t.Fatalf("read %s: %v; run: go run ./internal/mcpserver/gendoc", toolDocPath, err)
	}
	if !bytes.Equal(committed, got) {
		t.Fatal("docs/mcp-tools.md is stale; run: go run ./internal/mcpserver/gendoc")
	}
}
