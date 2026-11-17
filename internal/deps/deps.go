//go:build tools

// Package deps pins every third-party module Casebook depends on so that
// `go mod tidy` keeps them in go.mod even before the packages that use them
// exist. The surfaces (desktop, CLI, MCP, integrations) import these modules
// as pinned here.
package deps

import (
	_ "github.com/modelcontextprotocol/go-sdk/mcp"
	_ "github.com/pelletier/go-toml/v2"
	_ "github.com/spf13/cobra"
	_ "github.com/wailsapp/wails/v2"
	_ "modernc.org/sqlite"
)
