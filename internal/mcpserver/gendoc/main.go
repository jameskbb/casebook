// Command gendoc writes docs/mcp-tools.md from the live MCP tool list. Run it
// from the module root:
//
//	go run ./internal/mcpserver/gendoc
//
// go test ./internal/mcpserver -run TestToolDocUpToDate fails when the
// committed file is stale.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/jameskbb/casebook/internal/mcpserver"
)

func main() {
	out := flag.String("o", "docs/mcp-tools.md", "output file")
	flag.Parse()
	doc, err := mcpserver.RenderToolDoc(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "gendoc:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, doc, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gendoc:", err)
		os.Exit(1)
	}
}
