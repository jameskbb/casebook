package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/integrations"
)

// setupMCPGOOS is the system setup mcp spells paths for (tests replace it).
var setupMCPGOOS = runtime.GOOS

// newSetupMCPCommand prints the configuration that registers Casebook with
// any MCP client (ADR-0017). It writes nothing and never opens the database.
func newSetupMCPCommand(env *Env) *cobra.Command {
	var (
		client    string
		list, wsl bool
	)
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Print the MCP configuration for any MCP client (Cursor, VS Code, Claude Desktop, ...)",
		Long: `Print the configuration that registers Casebook as the stdio MCP server
"casebook" with an MCP client. The server command is this casebook binary by
its absolute path with "mcp" (plus "--db <path>" when --db is given).

It writes nothing: stdout is only the snippet (JSON, TOML or a command), so it
can be pasted or redirected as is, and stderr says where it goes. Merge it
into the client's file by hand; for Claude Code and Codex, "casebook setup
claude" and "casebook setup codex" can write it for you.

--client picks the client (default generic, the common "mcpServers" JSON);
--list shows them all. On Windows, --wsl prints the configuration for a
client running inside WSL that starts this casebook.exe through its
/mnt/<drive> path.

The server uses the active case of the folder the client starts it in
(casebook case activate <ref> --here), else the global one.`,
		Example: `  casebook setup mcp
  casebook setup mcp --client cursor
  casebook setup mcp --client vscode > .vscode/mcp.json
  casebook setup mcp --list`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if list {
				if cmd.Flags().Changed("client") || wsl {
					return newUsageError("--list cannot be combined with --client or --wsl")
				}
				return writeMCPClients(env)
			}
			cli, err := setupCLIPath(env)
			if err != nil {
				return err
			}
			db, err := setupDBPath(env)
			if err != nil {
				return err
			}
			home, err := env.UserHomeDir()
			if err != nil {
				home = ""
			}
			s, err := integrations.MCPSnippetFor(integrations.MCPSnippetOptions{
				Client:  client,
				CLIPath: cli,
				DBPath:  db,
				GOOS:    setupMCPGOOS,
				Home:    home,
				Getenv:  env.Getenv,
				WSL:     wsl,
			})
			if err != nil {
				return newUsageError("%s", err.Error())
			}
			noteCasebookEnv(env)
			if env.JSON {
				return writeJSON(env.Stdout, s)
			}
			verb := "merge into"
			if s.Format == integrations.FormatShell {
				verb = ""
			}
			fmt.Fprintf(env.Stderr, "casebook: %s: %s\n", s.Title, joinWords(verb, s.Location))
			if s.Note != "" {
				fmt.Fprintf(env.Stderr, "casebook: note: %s\n", s.Note)
			}
			_, err = fmt.Fprint(env.Stdout, s.Snippet)
			return err
		},
	}
	cmd.Flags().StringVar(&client, "client", "generic", "the client to print the configuration for (see --list)")
	cmd.Flags().BoolVar(&list, "list", false, "list the clients --client accepts")
	cmd.Flags().BoolVar(&wsl, "wsl", false, "Windows only: for a client running inside WSL")
	return cmd
}

// writeMCPClients prints the clients setup mcp knows.
func writeMCPClients(env *Env) error {
	clients := integrations.MCPClients()
	if env.JSON {
		return writeJSON(env.Stdout, clients)
	}
	tw := newTabWriter(env.Stdout)
	fmt.Fprintln(tw, "CLIENT\tNAME")
	for _, c := range clients {
		fmt.Fprintf(tw, "%s\t%s\n", c.Name, c.Title)
	}
	return tw.Flush()
}

// joinWords joins a and b with a space, skipping an empty a.
func joinWords(a, b string) string {
	if a == "" {
		return b
	}
	return a + " " + b
}
