package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/integrations"
)

// Skip details of the checks that need an open database.
const (
	skipNoDatabase = "no database yet"
	skipNotOpened  = "database could not be opened"
)

// dependentSkip is the skip check for id when the database is not open.
func dependentSkip(id string, s dbState) Check {
	if s == dbMissing {
		return check(id, StatusSkip, skipNoDatabase)
	}
	return check(id, StatusSkip, skipNotOpened)
}

// integrityCheck runs integrity_check and foreign_key_check. Problems are
// sanitized by internal/app (tables, indexes and row ids only).
func integrityCheck(ctx context.Context, a *app.App, s dbState) Check {
	if s != dbOpened {
		return dependentSkip(idIntegrity, s)
	}
	err := a.CheckIntegrity(ctx)
	if err == nil {
		return check(idIntegrity, StatusOK, "integrity_check and foreign_key_check are clean")
	}
	if ie, ok := app.IsIntegrity(err); ok {
		shown := ie.Problems
		if len(shown) > 3 {
			shown = shown[:3]
		}
		return check(idIntegrity, StatusFail, "%d problems: %s", ie.Total, strings.Join(shown, "; "))
	}
	return check(idIntegrity, StatusFail, "%s", err.Error())
}

// latestSchema is the schema version a fresh database gets from this
// build, read from a temporary database that is removed afterwards.
func latestSchema(ctx context.Context) (int, error) {
	dir, err := os.MkdirTemp("", "casebook-doctor-schema-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	a, err := app.Open(filepath.Join(dir, "casebook.db"))
	if err != nil {
		return 0, err
	}
	defer a.Close()
	st, err := a.Status(ctx)
	if err != nil {
		return 0, err
	}
	return st.SchemaVersion, nil
}

// migrationsCheck compares the database's schema version with the latest
// this build knows. databaseCheck's open has already applied any pending
// migrations, as every casebook command does, so the report says so: a
// version below the latest here means a migration did not apply, and a newer
// database never opens (dbTooNew).
func migrationsCheck(ctx context.Context, a *app.App, s dbState) Check {
	switch s {
	case dbTooNew:
		return check(idMigrations, StatusFail, "database is newer than this build")
	case dbOpened:
	default:
		return dependentSkip(idMigrations, s)
	}
	latest, err := latestSchema(ctx)
	if err != nil {
		return check(idMigrations, StatusFail, "cannot determine the latest schema version: %s", err.Error())
	}
	st, err := a.Status(ctx)
	if err != nil {
		return check(idMigrations, StatusFail, "%s", err.Error())
	}
	switch {
	case st.SchemaVersion == latest:
		return check(idMigrations, StatusOK, "schema version %d (latest for this build; opening the database applies pending migrations, as every casebook command does)", latest)
	case st.SchemaVersion < latest:
		return check(idMigrations, StatusFail, "schema version %d after opening, but this build's latest is %d: pending migrations did not apply", st.SchemaVersion, latest)
	default:
		return check(idMigrations, StatusFail, "schema version %d is newer than this build (%d)", st.SchemaVersion, latest)
	}
}

// writeAccessCheck checks that the database file is writable without opening
// it (probeWritable), so the SQLite locks of the App databaseCheck still
// holds open are never released. When the database exists but could not be
// opened the probe still runs, because a read-only file is a common reason
// for that: an unwritable file fails, a writable one skips.
func writeAccessCheck(dbPath string, s dbState) Check {
	if s == dbMissing {
		return dependentSkip(idWriteAccess, s)
	}
	err := probeWritable(dbPath)
	switch {
	case err != nil:
		return check(idWriteAccess, StatusFail, "database file is not writable: %v", osReason(err))
	case s != dbOpened:
		return dependentSkip(idWriteAccess, s)
	}
	return check(idWriteAccess, StatusOK, "database file is writable (checked without opening it)")
}

// cliPathCheck reports the CLI and the desktop app the packaged layout puts
// at dir(dir(CLI)): <install>/Casebook.exe next to <install>/bin/casebook.exe
// on Windows, Casebook.app/Contents/MacOS/Casebook next to .../MacOS/bin on
// macOS.
func cliPathCheck(o Options) Check {
	if o.CLIPath == "" {
		return check(idCLIPath, StatusWarn, "cannot locate the casebook binary")
	}
	if desktop := desktopBeside(o); desktop != "" {
		return check(idCLIPath, StatusOK, "%s; desktop app: %s", o.CLIPath, desktop)
	}
	// A warning, not ok: in an installed product this means `casebook ui`
	// cannot start the desktop app (AR-20). A source or CLI-only build sees
	// it too, which is why it is not a failure.
	return check(idCLIPath, StatusWarn, "%s; desktop app: not found next to the CLI (casebook ui will not work; expected for a CLI-only or source build)", o.CLIPath)
}

// commandProblem classifies a registered command: a missing command is a
// failure (the client cannot start Casebook), anything else a warning. It
// returns "" when the command is usable and matches this CLI.
func commandProblem(command string, exists, executable, matches bool) (Status, string) {
	switch {
	case command == "":
		return StatusWarn, "the casebook entry has no command"
	case !exists:
		return StatusFail, "the casebook command " + command + " does not exist"
	case !executable:
		return StatusWarn, "the casebook command " + command + " is not executable"
	case !matches:
		return StatusWarn, "casebook runs " + command + ", not this CLI"
	}
	return StatusOK, ""
}

// claudeConfigCheck reports Casebook's Claude Code registration and hooks.
func claudeConfigCheck(o Options, onPath bool) Check {
	st := integrations.InspectClaude(o.Paths, o.CLIPath)
	hooks := "hooks: none"
	switch {
	case st.SettingsExists && !st.SettingsValid:
		hooks = "hooks: " + st.SettingsPath + " is not valid JSON"
	case len(st.Hooks) > 0:
		hooks = "hooks: " + strings.Join(st.Hooks, ", ")
	}
	switch {
	case !st.ConfigExists && !onPath:
		return check(idClaudeConfig, StatusSkip, "%s does not exist and claude is not on PATH", st.ConfigPath)
	case !st.ConfigExists:
		return check(idClaudeConfig, StatusWarn, "%s does not exist; run casebook setup claude", st.ConfigPath)
	case !st.ConfigValid:
		return check(idClaudeConfig, StatusWarn, "%s is not valid JSON (or its mcpServers is not an object)", st.ConfigPath)
	case !st.Registered:
		return check(idClaudeConfig, StatusWarn, "%s: casebook is not registered; run casebook setup claude; %s", st.ConfigPath, hooks)
	}
	if s, problem := commandProblem(st.Command, st.CommandExists, st.CommandExecutable, st.MatchesCLI); s != StatusOK {
		return check(idClaudeConfig, s, "%s: %s; rerun casebook setup claude; %s", st.ConfigPath, problem, hooks)
	}
	status := StatusOK
	if st.SettingsExists && !st.SettingsValid {
		status = StatusWarn
	}
	return check(idClaudeConfig, status, "%s: casebook registered, command matches this CLI; %s", st.ConfigPath, hooks)
}

// codexConfigCheck reports Casebook's Codex registration.
func codexConfigCheck(o Options, onPath bool) Check {
	st := integrations.InspectCodex(o.Paths, o.CLIPath)
	switch {
	case !st.ConfigExists && !onPath:
		return check(idCodexConfig, StatusSkip, "%s does not exist and codex is not on PATH", st.ConfigPath)
	case !st.ConfigExists:
		return check(idCodexConfig, StatusWarn, "%s does not exist; run casebook setup codex", st.ConfigPath)
	case !st.ConfigValid:
		return check(idCodexConfig, StatusWarn, "%s is not valid TOML", st.ConfigPath)
	case !st.Registered:
		return check(idCodexConfig, StatusWarn, "%s: casebook is not registered; run casebook setup codex", st.ConfigPath)
	}
	kind := "managed block"
	if !st.Managed {
		kind = "hand-written [mcp_servers.casebook]"
	}
	if s, problem := commandProblem(st.Command, st.CommandExists, st.CommandExecutable, st.MatchesCLI); s != StatusOK {
		return check(idCodexConfig, s, "%s (%s): %s", st.ConfigPath, kind, problem)
	}
	if st.ForeignInBlock {
		return check(idCodexConfig, StatusWarn, "%s: the Casebook block holds other settings (such as a [projects] trust entry "+
			"Codex added); run casebook setup codex to move them out of the block", st.ConfigPath)
	}
	return check(idCodexConfig, StatusOK, "%s: casebook registered (%s), command matches this CLI", st.ConfigPath, kind)
}

// lookup finds a client CLI on PATH without running it.
func lookup(o Options, name string) (string, bool) {
	p, err := o.LookPath(name)
	return p, err == nil && p != ""
}

// clientCLICheck reports whether a client CLI is on PATH.
func clientCLICheck(id, path string, found bool) Check {
	if !found {
		return check(id, StatusSkip, "not on PATH")
	}
	return check(id, StatusOK, "found at %s", path)
}
