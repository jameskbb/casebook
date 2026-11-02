package cli

import (
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/integrations"
	"github.com/jameskbb/casebook/internal/mcpserver"
)

const mcpLong = `Serve the Casebook MCP server over stdio for AI clients such as Claude Code and Codex.

Stdout carries only MCP protocol messages; logs go to stderr. The server exits
when its stdin closes.

Environment:
  CASEBOOK_LOG_LEVEL  stderr log level: debug, info, warn (default) or error
  CASEBOOK_CLIENT     client name recorded on writes, overriding the name the
                      MCP client reports (for example claude-code or codex)
  CLAUDE_CODE_SESSION_ID
                      set by Claude Code: each casebook_checkpoint call is
                      noted for that session in hook-state.json next to the
                      database (when the hooks created it), so the Stop
                      hook counts only the session's own checkpoints`

func newMCPCommand(env *Env) *cobra.Command {
	var selfTest bool
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve the Casebook MCP server over stdio",
		Long:  mcpLong,
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if selfTest {
				return runSelfTest(cmd, env)
			}
			logger := slog.New(slog.NewTextHandler(env.Stderr, &slog.HandlerOptions{
				Level: logLevel(env.Getenv("CASEBOOK_LOG_LEVEL")),
			}))
			a, err := env.App(app.WithLogger(logger))
			if err != nil {
				return err
			}
			return mcpserver.Run(cmd.Context(), a, mcpserver.Options{
				ClientOverride: env.Getenv("CASEBOOK_CLIENT"),
				Logger:         logger,
				Version:        Version,
				OnCheckpoint:   sessionCheckpointRecorder(env, logger),
			})
		},
	}
	cmd.Flags().BoolVar(&selfTest, "self-test", false,
		"start this binary as an MCP server in a child process, complete the initialize handshake and tools/list, report, and exit")
	return cmd
}

// claudeSessionEnv names the variable Claude Code (2.1.154 and later) sets
// for the stdio MCP servers it starts: the id of the session they serve,
// the same session_id its hooks receive.
const claudeSessionEnv = "CLAUDE_CODE_SESSION_ID"

// sessionCheckpointRecorder returns the MCP server's OnCheckpoint callback:
// when Claude Code passed a usable session id, it notes each checkpoint for
// that session in hook-state.json next to the database
// (integrations.RecordSessionCheckpoint), so the Stop hook's 10-minute gate
// counts only that session's checkpoints. It is nil without a session id
// (another client, or an older Claude Code). Failures are logged to stderr
// and never fail the tool call.
func sessionCheckpointRecorder(env *Env, logger *slog.Logger) func(string) {
	sessionID := env.Getenv(claudeSessionEnv)
	if !integrations.ValidSessionID(sessionID) {
		return nil
	}
	return func(invRef string) {
		dbPath, _, err := hookDBPath(env)
		if err == nil {
			err = integrations.RecordSessionCheckpoint(hookStatePath(dbPath), sessionID, invRef, hookNow())
		}
		if err != nil {
			logger.Warn("record session checkpoint for the hooks", "error", err)
		}
	}
}

// logLevel parses CASEBOOK_LOG_LEVEL (trimmed, case-insensitive). Anything
// unknown, including empty, is warn.
func logLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}
