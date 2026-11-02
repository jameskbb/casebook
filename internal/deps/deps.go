//go:build tools

// Package deps pins every third-party module Casebook depends on so that
// `go mod tidy` keeps them in go.mod even before the packages that use them
// exist. Phase 1 owns go.mod: later phases (desktop, CLI, MCP, integrations)
// must not edit go.mod or go.sum; they import these modules as pinned here.
package deps

import (
	_ "github.com/modelcontextprotocol/go-sdk/mcp"
	_ "github.com/pelletier/go-toml/v2"
	_ "github.com/spf13/cobra"
	_ "github.com/wailsapp/wails/v2"
	_ "modernc.org/sqlite"
)
