// Command casebook is the Casebook CLI. `casebook mcp` serves the MCP server
// over stdio. All logic lives in internal/cli.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jameskbb/casebook/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// The first signal cancels ctx; stopping then restores the default
	// action, so a second Ctrl-C kills a command that is slow to wind down.
	go func() {
		<-ctx.Done()
		stop()
	}()
	code := cli.Execute(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
