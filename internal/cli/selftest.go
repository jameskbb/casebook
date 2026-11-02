package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/doctor"
)

// selfTestOutput is `casebook mcp --self-test --json`.
type selfTestOutput struct {
	OK            bool   `json:"ok"`
	Server        string `json:"server"`
	ServerVersion string `json:"server_version"`
	Tools         int    `json:"tools"`
	TempDB        bool   `json:"temp_db"`
}

// runSelfTest is `casebook mcp --self-test` (D-15): it starts this binary
// as an MCP server in a child process against the database hooks would
// read, completes the initialize handshake and tools/list, and reports. It
// never opens the App, so it never creates the database: a missing one is
// replaced by a temporary database for the check.
func runSelfTest(cmd *cobra.Command, env *Env) error {
	dbPath, _, err := hookDBPath(env)
	if err != nil {
		return err
	}
	cliPath, err := setupCLIPath(env)
	if err != nil {
		return fmt.Errorf("mcp self-test failed: %w", err)
	}
	check := doctorMCPCheck
	if check == nil {
		check = func(ctx context.Context, cliPath, dbPath string) (doctor.MCPResult, error) {
			return doctor.MCPStartup(ctx, cliPath, dbPath, doctor.MCPTimeout)
		}
	}
	res, err := check(cmd.Context(), cliPath, dbPath)
	if err != nil {
		return fmt.Errorf("mcp self-test failed: %w", err)
	}
	if env.JSON {
		return writeJSON(env.Stdout, selfTestOutput{
			OK:            true,
			Server:        res.ServerName,
			ServerVersion: res.ServerVersion,
			Tools:         res.Tools,
			TempDB:        res.TempDB,
		})
	}
	line := fmt.Sprintf("casebook mcp self-test: ok (%s %s, %d tools)", res.ServerName, res.ServerVersion, res.Tools)
	if res.TempDB {
		line += "; ran against a temporary database because " + dbPath + " does not exist yet"
	}
	_, err = fmt.Fprintln(env.Stdout, line)
	return err
}
