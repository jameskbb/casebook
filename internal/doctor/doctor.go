// Package doctor diagnoses a Casebook installation: the data directory, the
// database (integrity, migrations, write access), a real MCP handshake
// against a spawned `casebook mcp`, the CLI location, whether the desktop app
// next to the CLI is the same version, the Claude Code and Codex config
// (D-14).
//
// Doctor never creates the database, never changes its records and never
// runs an AI client. Opening an existing database applies any pending schema
// migrations (after writing a pre-upgrade backup to the backups folder next
// to it) and tightens its file mode, as every casebook command does;
// the migrations check reports the version that open leaves. Its output is meant to be pasted into bug reports, so every
// detail holds only paths, booleans, counts, refs, versions and statuses:
// never record text, environment values or config file contents (DIAG-02).
package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/integrations"
)

// Status is the outcome of one check.
type Status string

// Check statuses: a warn never fails doctor; a fail does.
const (
	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
	StatusSkip Status = "skip"
)

// Check is one diagnostic result.
type Check struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
}

// Report is a whole doctor run.
type Report struct {
	Version string `json:"version"`
	// OK is true when no check failed (warnings and skips are fine).
	OK bool `json:"ok"`
	// Checks are in a fixed order and never nil.
	Checks []Check `json:"checks"`
}

// Failed counts the failed checks.
func (r Report) Failed() int {
	n := 0
	for _, c := range r.Checks {
		if c.Status == StatusFail {
			n++
		}
	}
	return n
}

// Options configures Run.
type Options struct {
	// DBPath is the absolute database path to diagnose.
	DBPath string
	// CLIPath is the casebook CLI ("" when it could not be determined).
	CLIPath string
	// Paths locates the AI client config files.
	Paths integrations.Paths
	// Version is the CLI version (cli.Version).
	Version string
	// GOOS names the desktop app binary; "" means runtime.GOOS.
	GOOS string
	// LookPath finds client CLIs on PATH; nil means exec.LookPath. Doctor
	// never runs what it finds.
	LookPath func(string) (string, error)
	// MCPCheck runs the MCP startup check; nil means MCPStartup with
	// MCPTimeout.
	MCPCheck func(ctx context.Context, cliPath, dbPath string) (MCPResult, error)
	// WorkDir is the directory the active investigation is reported for
	// (a per-directory one wins there, ADR-0015); "" reports the global one.
	WorkDir string
}

// Check ids and names, in report order.
const (
	idDataDir       = "data_dir"
	idDatabase      = "database"
	idIntegrity     = "integrity"
	idMigrations    = "migrations"
	idWriteAccess   = "write_access"
	idMCPStartup    = "mcp_startup"
	idCLIPath       = "cli_path"
	idBundleVersion = "bundle_version"
	idClaudeConfig  = "claude_config"
	idCodexConfig   = "codex_config"
	idClaudeCLI     = "claude_cli"
	idCodexCLI      = "codex_cli"
)

var checkNames = map[string]string{
	idDataDir:       "Data directory",
	idDatabase:      "Database",
	idIntegrity:     "Integrity",
	idMigrations:    "Migrations",
	idWriteAccess:   "Write access",
	idMCPStartup:    "MCP startup",
	idCLIPath:       "CLI path",
	idBundleVersion: "Bundle version",
	idClaudeConfig:  "Claude Code config",
	idCodexConfig:   "Codex config",
	idClaudeCLI:     "Claude Code CLI",
	idCodexCLI:      "Codex CLI",
}

// check builds a Check with its fixed name.
func check(id string, s Status, format string, args ...any) Check {
	return Check{ID: id, Name: checkNames[id], Status: s, Detail: fmt.Sprintf(format, args...)}
}

// openBudget bounds how long doctor waits on a locked database.
const openBudget = 2 * time.Second

// dbState is what the database check found, for the checks that depend on
// it.
type dbState int

const (
	dbMissing dbState = iota
	dbOpenFailed
	// dbTooNew is an open failure because a newer Casebook wrote the file.
	dbTooNew
	dbOpened
)

// Run performs every check in order and returns the report. It closes any
// App it opened.
func Run(ctx context.Context, o Options) Report {
	if o.LookPath == nil {
		o.LookPath = exec.LookPath
	}
	if o.MCPCheck == nil {
		o.MCPCheck = func(ctx context.Context, cliPath, dbPath string) (MCPResult, error) {
			return MCPStartup(ctx, cliPath, dbPath, MCPTimeout)
		}
	}
	checks := []Check{dataDirCheck(o.DBPath)}

	dbc, a, state := databaseCheck(ctx, o.DBPath, o.WorkDir)
	if a != nil {
		defer a.Close()
	}
	checks = append(checks,
		dbc,
		integrityCheck(ctx, a, state),
		migrationsCheck(ctx, a, state),
		writeAccessCheck(o.DBPath, state),
		mcpStartupCheck(ctx, o),
		cliPathCheck(o),
		bundleVersionCheck(o),
	)
	claudePath, claudeFound := lookup(o, "claude")
	codexPath, codexFound := lookup(o, "codex")
	checks = append(checks,
		claudeConfigCheck(o, claudeFound),
		codexConfigCheck(o, codexFound),
		clientCLICheck(idClaudeCLI, claudePath, claudeFound),
		clientCLICheck(idCodexCLI, codexPath, codexFound),
	)
	return Report{Version: o.Version, OK: Report{Checks: checks}.Failed() == 0, Checks: checks}
}

// dataDirCheck reports whether the database's directory exists and is
// writable (probed with a temp file that is removed at once).
func dataDirCheck(dbPath string) Check {
	dir := filepath.Dir(dbPath)
	fi, err := os.Stat(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return check(idDataDir, StatusWarn, "%s does not exist yet; Casebook creates it on first use", dir)
	case err != nil:
		return check(idDataDir, StatusFail, "%s cannot be read: %v", dir, err)
	case !fi.IsDir():
		return check(idDataDir, StatusFail, "%s is not a directory", dir)
	}
	f, err := os.CreateTemp(dir, ".casebook-doctor-*")
	if err != nil {
		return check(idDataDir, StatusFail, "%s is not writable: %v", dir, osReason(err))
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return check(idDataDir, StatusOK, "%s (writable)", dir)
}

// databaseCheck opens the database when it exists and summarizes it with
// numbers and refs only. It never creates the file; opening an existing one
// applies pending migrations, as every casebook command does. The returned App (nil
// unless the state is dbOpened) is the caller's to close.
func databaseCheck(ctx context.Context, dbPath, workDir string) (Check, *app.App, dbState) {
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		return check(idDatabase, StatusWarn, "%s does not exist yet; it is created on first use", dbPath), nil, dbMissing
	} else if err != nil {
		return check(idDatabase, StatusFail, "%s cannot be read: %v", dbPath, osReason(err)), nil, dbOpenFailed
	}
	a, err := app.Open(dbPath, app.WithBusyTimeout(openBudget), app.WithRetryBudget(openBudget), app.WithWorkDir(workDir))
	if err != nil {
		if errors.Is(err, app.ErrSchemaTooNew) {
			// The error names both schema versions and what to do.
			return check(idDatabase, StatusFail, "%s: %s", dbPath, err.Error()), nil, dbTooNew
		}
		return check(idDatabase, StatusFail, "%s", err.Error()), nil, dbOpenFailed
	}
	st, err := a.Status(ctx)
	if err != nil {
		a.Close()
		return check(idDatabase, StatusFail, "%s", err.Error()), nil, dbOpenFailed
	}
	active := st.ActiveInvestigationRef
	if active == "" {
		active = "none"
	} else if st.ActiveDir != "" {
		active += " (for " + st.ActiveDir + ")"
	}
	c := st.Counts
	return check(idDatabase, StatusOK, "%s: schema %d; %d investigations, %d evidence, %d claims, %d hypotheses, %d questions; active %s",
		dbPath, st.SchemaVersion, c.Investigations, c.Evidence, c.Claims, c.Hypotheses, c.Questions, active), a, dbOpened
}

// mcpStartupCheck spawns the server through o.MCPCheck.
func mcpStartupCheck(ctx context.Context, o Options) Check {
	if o.CLIPath == "" {
		return check(idMCPStartup, StatusFail, "cannot locate the casebook binary")
	}
	res, err := o.MCPCheck(ctx, o.CLIPath, o.DBPath)
	if err != nil {
		return check(idMCPStartup, StatusFail, "%s", err.Error())
	}
	detail := fmt.Sprintf("initialize and tools/list completed: %s %s, %d tools", res.ServerName, res.ServerVersion, res.Tools)
	if res.TempDB {
		detail += " (against a temporary database; none exists yet)"
	}
	return check(idMCPStartup, StatusOK, "%s", detail)
}

// osReason is the cause of a filesystem error without the operation and
// path *PathError adds (the detail already names the path).
func osReason(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}
