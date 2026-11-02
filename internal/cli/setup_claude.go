package cli

import (
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/integrations"
)

// Next steps reported by setup claude.
const (
	claudeNextSetup  = "Restart Claude Code (or run /mcp), then check with: claude mcp get casebook"
	claudeNextHooks  = "Restart Claude Code (or run /mcp), then check with: claude mcp get casebook; the hooks run in new sessions (list them with /hooks)"
	claudeNextRemove = "Restart Claude Code to unload Casebook."
	claudeNextPrint  = "Run casebook setup claude without --print to write it, or merge it by hand."
	// claudeNextRefused is the next step of a --json refusal report.
	claudeNextRefused = "Nothing was written; merge the configuration in preview by hand, or fix the file and rerun."
)

// newSetupClaudeCommand registers the Casebook MCP server in Claude Code's
// user config, removes it (--remove) or prints it (--print). It never opens
// the database.
func newSetupClaudeCommand(env *Env) *cobra.Command {
	var remove, print, hooks, skill, quiet bool
	var scope string
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Register the Casebook MCP server in Claude Code user config",
		Long: `Register the Casebook MCP server in Claude Code's user config: the
"mcpServers" object of ~/.claude.json, or $CLAUDE_CONFIG_DIR/.claude.json when
CLAUDE_CONFIG_DIR is set. The entry runs this casebook binary by its absolute
path with "mcp" (plus "--db <path>" when --db is given).

--hooks also merges Casebook hook groups into the "hooks" object of
~/.claude/settings.json ($CLAUDE_CONFIG_DIR/settings.json): SessionStart runs
"casebook hook session-start" to add the active case's context to new
sessions, and Stop and PreCompact run "casebook hook checkpoint": at Stop it
reminds the model to checkpoint findings, before compaction it warns you
(Claude Code shows PreCompact output to the user only). Groups holding your own hooks are never edited;
a Casebook group keeps the matcher and timeout you set on it. On Windows
each hook is written in exec form (the program in "command", its
arguments in "args", run with no shell; Claude Code 2.1.139 or later), so
it works whether Claude Code runs hooks through Git Bash or PowerShell.

--scope all (the default) leaves the hooks speaking in every Claude Code
session while a case is active. --scope marked writes
"--scope marked" into each hook command, so the hooks speak only in
projects that hold a .casebook file, with no environment variable to set.
Rerun with --scope all to undo it. CASEBOOK_HOOK_SCOPE=marked still
narrows hooks installed with --scope all.

When no case is active, the session-start hook gives Claude one
line: offer a Casebook case if the session turns into an investigation.
--quiet-when-inactive writes that flag into the hook command so the
session gets nothing instead; rerun without it to restore the hint.

--skill also writes the Casebook skill to skills/casebook/SKILL.md in
~/.claude ($CLAUDE_CONFIG_DIR): Claude Code offers it as /casebook to start
or resume a case, and lists it to Claude so it can pick up a case when you
agree to one. The file carries a casebook-managed line; a SKILL.md without
it is yours and is never changed. "casebook setup instructions" prints a
short CLAUDE.md or AGENTS.md section to paste yourself.

Only Casebook-owned entries are written; every other key is left as it is.
Inside mcpServers.casebook only type, command and args are set: env and any
other member you added there are kept. A
backup of the previous file is written first, and a rerun that would change
nothing writes nothing. --remove deletes only mcpServers.casebook, any
Casebook hook handlers in settings.json and the Casebook skill; the mcpServers and hooks objects and
the hook event arrays stay, even when that leaves them empty. --print shows
the configuration (with --hooks, the hooks too) and changes nothing.`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if print && remove {
				return newUsageError("--print cannot be combined with --remove")
			}
			if quiet && (!hooks || remove) {
				return newUsageError("--quiet-when-inactive needs --hooks")
			}
			if cmd.Flags().Changed("scope") {
				if !hooks || remove {
					return newUsageError("--scope needs --hooks")
				}
				switch scope {
				case integrations.ScopeAll, integrations.ScopeMarked:
				default:
					return newUsageError(`--scope must be "all" or "marked"`)
				}
			}
			p, err := setupPaths(env)
			if err != nil {
				return err
			}
			if remove {
				if skill {
					return newUsageError("--skill cannot be combined with --remove (--remove also removes the skill)")
				}
				return runSetupClaudeRemove(env, p)
			}
			db, err := setupDBPath(env)
			if err != nil {
				return err
			}
			cli, err := setupCLIPath(env)
			if err != nil {
				return err
			}
			noteCasebookEnv(env)
			opts := integrations.ClaudeOptions{Paths: p, DBPath: db, Hooks: hooks, HookScope: scope, HookQuiet: quiet}
			if print {
				preview := integrations.ClaudeConfigPreview(cli, opts)
				if skill {
					preview += "\n" + integrations.SkillPreview(integrations.SkillPath(p.ClaudeConfigDir))
				}
				return writeSetupReport(env, setupReport{
					Client:  "claude",
					CLIPath: cli,
					DryRun:  true,
					Preview: preview,
					Next:    claudeNextPrint,
				})
			}
			changes, err := integrations.SetupClaude(cli, opts)
			if err != nil {
				var refused *integrations.RefusedError
				if len(changes) == 0 && env.JSON && errors.As(err, &refused) {
					return claudeRefusedJSON(env, cli, opts, err)
				}
				return claudePartial(env, cli, changes, err)
			}
			if skill {
				sc, err := integrations.InstallSkill(integrations.SkillPath(p.ClaudeConfigDir), time.Now())
				if err != nil {
					return claudePartial(env, cli, changes, err)
				}
				changes = append(changes, sc)
			}
			next := claudeNextSetup
			if hooks {
				next = claudeNextHooks
			}
			return writeSetupReport(env, setupReport{
				Client:  "claude",
				CLIPath: cli,
				Next:    next,
				Changes: changes,
			})
		},
	}
	cmd.Flags().BoolVar(&remove, "remove", false, "remove Casebook-owned entries (the MCP server and any Casebook hooks) and nothing else")
	cmd.Flags().BoolVar(&print, "print", false, "print the configuration that would be written and change nothing")
	cmd.Flags().BoolVar(&hooks, "hooks", false, "also install Claude Code SessionStart, Stop and PreCompact hooks")
	cmd.Flags().BoolVar(&skill, "skill", false, "also install the Casebook skill (/casebook) in the Claude Code config directory")
	cmd.Flags().StringVar(&scope, "scope", integrations.ScopeAll,
		`with --hooks: "all" (the default) lets the hooks speak in every session; "marked" writes --scope marked into the hook commands so they speak only in projects marked with a .casebook file`)
	cmd.Flags().BoolVar(&quiet, "quiet-when-inactive", false,
		"with --hooks: write --quiet-when-inactive into the session-start hook, so a session with no active case gets no hint to offer one")
	return cmd
}

// runSetupClaudeRemove is setup claude --remove. The CLI path is reported
// when known but is not needed to remove the entry.
func runSetupClaudeRemove(env *Env, p integrations.Paths) error {
	changes, err := integrations.RemoveClaude(integrations.ClaudeOptions{Paths: p})
	cli, cerr := setupCLIPath(env)
	if cerr != nil {
		cli = ""
	}
	if err != nil {
		return claudePartial(env, cli, changes, err)
	}
	if sc, err := integrations.RemoveSkill(integrations.SkillPath(p.ClaudeConfigDir)); err != nil {
		return claudePartial(env, cli, changes, err)
	} else if sc.Action == integrations.ActionRemoved {
		changes = append(changes, sc)
	}
	return writeSetupReport(env, setupReport{
		Client:  "claude",
		CLIPath: cli,
		Next:    claudeNextRemove,
		Changes: changes,
	})
}

// claudePartial returns err from a setup claude run. When the first file
// was already written before the second failed (changes is not empty), the
// error text names that file and its backup, and with --json the completed
// changes are also printed as the report (D-04).
func claudePartial(env *Env, cli string, changes []integrations.FileChange, err error) error {
	if len(changes) > 0 && env.JSON {
		if werr := writeSetupReport(env, setupReport{Client: "claude", CLIPath: cli, Changes: changes}); werr != nil {
			return werr
		}
	}
	return err
}

// claudeRefusedJSON prints the --json report of a refused setup claude run,
// with the configuration to merge by hand as its preview (as setup codex
// does), and returns err, so the refusal itself is the one stderr line and
// the exit code is 1.
func claudeRefusedJSON(env *Env, cli string, opts integrations.ClaudeOptions, err error) error {
	if werr := writeSetupReport(env, setupReport{
		Client:  "claude",
		CLIPath: cli,
		Preview: integrations.ClaudeConfigPreview(cli, opts),
		Next:    claudeNextRefused,
	}); werr != nil {
		return werr
	}
	return err
}
