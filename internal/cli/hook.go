package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/integrations"
	"github.com/jameskbb/casebook/internal/paths"
)

// hookReadTimeout bounds the wait for the hook payload on stdin (tests lower
// it).
var hookReadTimeout = time.Second

// hookNow is the hooks' clock for hook-state.json (tests replace it).
var hookNow = time.Now

// Hook budgets (D-10): the hooks run inside someone else's session, so they
// give up quickly instead of waiting on a busy database.
const (
	// hookOpenBudget is both the SQLite busy timeout and the write retry
	// budget of the App a hook opens.
	hookOpenBudget = time.Second
	// hookRunTimeout bounds all database work of one hook run.
	hookRunTimeout = 3 * time.Second
)

// newHookCommand is the hook group: the Claude Code hook handlers that
// `casebook setup claude --hooks` installs.
func newHookCommand(env *Env) *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Claude Code hook handlers installed by casebook setup claude --hooks",
		Long: `Claude Code hook handlers installed by "casebook setup claude --hooks".

Each handler reads the hook JSON from stdin and uses only the session id,
event name, source, stop flag and working directory (for the scope set by
--scope and CASEBOOK_HOOK_SCOPE, see "casebook hook session-start --help",
and to pick the case made active for that directory with
"casebook case activate <ref> --here", else the global one). It
never reads transcripts, never writes Casebook data, prints at most one
JSON object on stdout and always exits 0: an error is one
"casebook hook: <message>" line on stderr.`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.PersistentFlags().StringVar(&scope, "scope", "",
		`limit the hooks to projects marked with a .casebook file ("marked"); "all" or unset leaves it to CASEBOOK_HOOK_SCOPE`)
	cmd.AddCommand(newHookSessionStartCommand(env, &scope), newHookCheckpointCommand(env, &scope))
	return cmd
}

// newHookSessionStartCommand injects the active investigation's compact
// context pack at session start (D-11).
func newHookSessionStartCommand(env *Env, scope *string) *cobra.Command {
	var hint, quiet bool
	cmd := &cobra.Command{
		Use:   "session-start",
		Short: "Print the active case's compact context as SessionStart hook output",
		Long: `Print the SessionStart hook output that adds the active case's compact
context pack to a new Claude Code session. With no active case it prints a
one-line hint instead: if the session turns into an investigation, offer the
person a Casebook case, and find an earlier one with the search tools. The
hint asks for nothing in sessions that stay unrelated; --quiet-when-inactive
prints nothing instead. With a session id in the payload it records when the
session started in hook-state.json next to the database, so the checkpoint
hook reminds only about this session's work. It never creates the
database.

The pack is fenced and introduced as stored data written by people and AI
clients, not instructions, and is preceded by a line naming who made the
case active and when.

Scope: by default (no --scope flag, CASEBOOK_HOOK_SCOPE unset, or either
set to "all") the hooks speak in every Claude Code session. With --scope
marked (which "casebook setup claude --hooks --scope marked" writes into
the hook commands) or CASEBOOK_HOOK_SCOPE=marked they speak only when the
session's directory (the payload's cwd, else the current directory) is,
or is inside, a directory that holds a file or folder named .casebook;
elsewhere they print nothing. Either setting can narrow the scope to
"marked" and neither widens the other. On Windows a Linux cwd from Claude
Code inside WSL is translated: /mnt/c/... to C:\..., other paths to
\\wsl.localhost\<distro>\... (the distribution from WSL_DISTRO_NAME when
WSLENV passes it, else from the working directory WSL starts the program
in); when that fails the hooks print nothing.`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			in, status := integrations.ReadHookInput(env.Stdin, env.StdinIsTerminal(), hookReadTimeout)
			sessionID := ""
			if status == integrations.InputOK && integrations.ValidSessionID(in.SessionID) {
				sessionID = in.SessionID
			}
			var out []byte
			env.WorkDir = hookPayloadDir(env, in, status)
			inScope, err := hookInScope(env, *scope, in, status)
			if err == nil && inScope {
				out, err = hookSessionStart(cmd.Context(), env, sessionID, hint && !quiet)
			}
			if cerr := env.Close(); cerr != nil && err == nil {
				err = cerr
			}
			if err != nil {
				hookFail(env, err)
				return nil
			}
			if len(out) > 0 {
				if _, err := env.Stdout.Write(out); err != nil {
					hookFail(env, err)
				}
			}
			return nil
		},
	}
	// The hint is the default (ADR-0018; it was opt-in under AR-04). The
	// agent eval (scripts/agent-eval/README.md) showed that without it
	// Claude almost never offered a case for a quickly found cause the
	// person wanted to come back to; the hint is conditional, so it asks
	// nothing of unrelated sessions. --hint-when-inactive stays accepted so
	// hook commands written for the old default keep working.
	cmd.Flags().BoolVar(&hint, "hint-when-inactive", true, "print a one-line hint when no case is active (the default)")
	_ = cmd.Flags().MarkHidden("hint-when-inactive")
	cmd.Flags().BoolVar(&quiet, "quiet-when-inactive", false, "print nothing when no case is active")
	return cmd
}

// hookSessionStart returns the session-start output: the preamble and
// compact pack of the active investigation, else nothing (the no-active
// hint when hint is set). With a session id and an existing database it records the
// session's first-seen time in hook-state.json, so the checkpoint hook
// reminds only about activity from this session's lifetime (WR-02).
func hookSessionStart(parent context.Context, env *Env, sessionID string, hint bool) ([]byte, error) {
	inactive := func() ([]byte, error) {
		if !hint {
			return nil, nil
		}
		return integrations.SessionStartOutput(integrations.NoActiveHint()), nil
	}
	dbPath, exists, err := hookDBPath(env)
	if err != nil {
		return nil, err
	}
	if !exists {
		return inactive()
	}
	if sessionID != "" {
		// Best effort: the context matters more than the state, and a state
		// that cannot be written makes the checkpoint hook record the time
		// itself (or stay quiet) later.
		_, _ = integrations.SessionFirstSeen(hookStatePath(dbPath), sessionID, hookNow())
	}
	ctx, cancel := hookContext(parent)
	defer cancel()
	a, err := hookApp(env)
	if err != nil {
		return nil, err
	}
	active, err := a.GetActiveInvestigation(ctx)
	if err != nil {
		return nil, err
	}
	if active == nil {
		return inactive()
	}
	pack, err := a.RenderContext(ctx, active.Ref, "markdown", app.ContextOptions{Compact: true})
	if err != nil {
		return nil, err
	}
	setBy, err := activeSetLine(ctx, a, active.Ref)
	if err != nil {
		return nil, err
	}
	if setBy != "" {
		pack = setBy + "\n\n" + pack
	}
	return integrations.SessionStartOutput(
		integrations.SessionStartContext(pack, app.FrameContext, app.FrameContextOverhead())), nil
}

// activeSetWindow is how many of the newest activity rows the session-start
// hook searches for the row that made the investigation active.
const activeSetWindow = 200

// activeSetLine names who made invRef the active investigation and when,
// from its newest local active.set row among the newest activeSetWindow
// activity rows, or "" when there is none (imported rows are skipped: they
// record another database's history). Any surface or AI client can switch
// the active investigation, so the session sees who did.
func activeSetLine(ctx context.Context, a *app.App, invRef string) (string, error) {
	rows, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: invRef, Limit: activeSetWindow})
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if (r.Action != "active.set" && r.Action != "active.bind") || r.Imported {
			continue
		}
		who := strings.Join(strings.Fields(string(r.Actor.Kind)+" "+r.Actor.Client), " ")
		return "Made the active investigation at " + r.CreatedAt + " by " + who + ".", nil
	}
	return "", nil
}

// hookInScope reports whether the hooks may speak in this session under
// the hook command's --scope flag (flag) and the CASEBOOK_HOOK_SCOPE
// setting (integrations.EffectiveHookScope). Under "marked" the session's
// directory is the payload's cwd (a Linux cwd from WSL translated on
// Windows, integrations.HookSessionDir), else (a manual run, or a payload
// without cwd) the hook's own working directory, which Claude Code sets to
// the project directory. An unknown setting is an error, so the hook
// prints nothing.
func hookInScope(env *Env, flag string, in integrations.HookInput, status integrations.InputStatus) (bool, error) {
	scope, err := integrations.EffectiveHookScope(flag, env.Getenv(integrations.HookScopeEnv))
	if err != nil || scope == integrations.ScopeAll {
		return err == nil, err
	}
	wd, _ := os.Getwd()
	cwd := ""
	if status == integrations.InputOK && in.Cwd != "" {
		cwd = integrations.HookSessionDir(in.Cwd, runtime.GOOS, env.Getenv(integrations.WSLDistroEnv), wd)
		if cwd == "" {
			return false, nil
		}
	}
	if cwd == "" {
		cwd = wd
	}
	return integrations.InScope(scope, cwd, env.Stat), nil
}

// hookPayloadDir is the session directory from the hook payload's cwd (a
// Linux cwd from WSL translated on Windows, integrations.HookSessionDir),
// or "" without one, so the App falls back to the hook's own working
// directory, which Claude Code sets to the project directory. The App
// resolves a per-directory active investigation from it (ADR-0015).
func hookPayloadDir(env *Env, in integrations.HookInput, status integrations.InputStatus) string {
	if status != integrations.InputOK || in.Cwd == "" {
		return ""
	}
	wd, _ := os.Getwd()
	return integrations.HookSessionDir(in.Cwd, runtime.GOOS, env.Getenv(integrations.WSLDistroEnv), wd)
}

// hookReminderMinAge is how long a session must have worked without a
// checkpoint before Stop reminds it: a short exchange is not worth an extra
// model turn, and a session that just checkpointed has nothing to add.
const hookReminderMinAge = 10 * time.Minute

// newHookCheckpointCommand reminds the model to checkpoint material findings
// at Stop and warns the user before compaction (D-12, D-13, AR-01, AR-03).
func newHookCheckpointCommand(env *Env, scope *string) *cobra.Command {
	return &cobra.Command{
		Use:   "checkpoint",
		Short: "Remind to checkpoint findings (Stop and PreCompact hook)",
		Long: `Remind about persisting findings with one casebook_checkpoint call while a
case is active.

At Stop it reminds the model, once per session per case, when the session
has run for at least 10 minutes since it started (its first hook run) or
since its own latest checkpoint of the case, whichever is later. Before
compaction (PreCompact) it warns the user, once per session per case,
unless the session checkpointed the case in the last 10 minutes; Claude Code shows PreCompact output to the user only.
Only the session's own checkpoints count: the casebook MCP server records
each casebook_checkpoint call in hook-state.json under the session id that
Claude Code passes it (CLAUDE_CODE_SESSION_ID), so checkpoints written by
other sessions, clients or the desktop app do not hold the reminder back.

It stays silent when no hook payload with a session id arrives on stdin (a
manual run, an empty or invalid payload, or a timeout), when no case is
active, when Claude Code reports the Stop hook is
already active, and outside the scope set by --scope and
CASEBOOK_HOOK_SCOPE (see "casebook hook session-start --help"). It writes
no Casebook data; the only file it writes is hook-state.json next to the
database.`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			in, status := integrations.ReadHookInput(env.Stdin, env.StdinIsTerminal(), hookReadTimeout)
			env.WorkDir = hookPayloadDir(env, in, status)
			out, err := hookCheckpoint(cmd.Context(), env, *scope, in, status)
			if cerr := env.Close(); cerr != nil && err == nil {
				err = cerr
			}
			if err != nil {
				hookFail(env, err)
				return nil
			}
			if len(out) > 0 {
				if _, err := env.Stdout.Write(out); err != nil {
					hookFail(env, err)
				}
			}
			return nil
		},
	}
}

// hookCheckpoint returns the Stop reminder or the PreCompact warning, or
// nothing when neither is warranted. Only output that is actually printed
// claims the session in hook-state.json (the Stop reminder and the
// PreCompact warning are claimed separately). A run without a well-formed
// payload carrying a usable session id (integrations.ValidSessionID) prints
// nothing: without the session claim and the stop_hook_active flag a Stop
// reminder could repeat on every turn, so the loop guards fail closed. A
// session outside the hook scope (hookInScope) gets nothing either.
//
// The reminder does not depend on what the session already wrote to
// Casebook (AR-01): a session that never touched Casebook is exactly the
// one that most needs it. It is gated on how long the session has worked
// since its start or its own latest checkpoint instead
// (integrations.SessionCheckpoint, recorded by the MCP server of the same
// session): a checkpoint another session or client wrote for the same
// investigation says nothing about what this session still holds.
func hookCheckpoint(parent context.Context, env *Env, scope string, in integrations.HookInput, status integrations.InputStatus) ([]byte, error) {
	if status != integrations.InputOK || !integrations.ValidSessionID(in.SessionID) || in.StopHookActive {
		return nil, nil
	}
	if inScope, err := hookInScope(env, scope, in, status); err != nil || !inScope {
		return nil, err
	}
	dbPath, exists, err := hookDBPath(env)
	if err != nil || !exists {
		return nil, err
	}
	statePath := hookStatePath(dbPath)
	now := hookNow()
	firstSeen, err := integrations.SessionFirstSeen(statePath, in.SessionID, now)
	if err != nil {
		return nil, err
	}
	ctx, cancel := hookContext(parent)
	defer cancel()
	a, err := hookApp(env)
	if err != nil {
		return nil, err
	}
	active, err := a.GetActiveInvestigation(ctx)
	if err != nil || active == nil {
		return nil, err
	}
	lastCheckpoint, checkpointed, err := integrations.SessionCheckpoint(statePath, in.SessionID, active.Ref)
	if err != nil {
		return nil, err
	}
	if in.HookEventName == "PreCompact" {
		if checkpointed && now.Sub(lastCheckpoint) < hookReminderMinAge {
			return nil, nil
		}
		first, err := integrations.ClaimPreCompactWarning(statePath, in.SessionID, active.Ref, now)
		if err != nil || !first {
			return nil, err
		}
		return integrations.PreCompactOutput(integrations.PreCompactMessage(active.Ref)), nil
	}
	since := firstSeen
	if checkpointed && lastCheckpoint.After(since) {
		since = lastCheckpoint
	}
	if now.Sub(since) < hookReminderMinAge {
		return nil, nil
	}
	first, err := integrations.ClaimReminder(statePath, in.SessionID, active.Ref, now)
	if err != nil || !first {
		return nil, err
	}
	return integrations.StopOutput(integrations.CheckpointReminder(active.Ref)), nil
}

// hookStatePath is hook-state.json next to the database at dbPath.
func hookStatePath(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), integrations.HookStateFile)
}

// hookContext bounds a hook's database work by hookRunTimeout.
func hookContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, hookRunTimeout)
}

// hookApp opens the App with the hook busy timeout and retry budget.
func hookApp(env *Env) (*app.App, error) {
	return env.App(app.WithBusyTimeout(hookOpenBudget), app.WithRetryBudget(hookOpenBudget))
}

// hookDBPath is the database a hook (or bare casebook) reads: --db made
// absolute, else paths.DBPath(). exists reports whether something is at
// that path, so the caller never creates the database by opening it.
func hookDBPath(env *Env) (path string, exists bool, err error) {
	if env.DBPath != "" {
		path, err = filepath.Abs(env.DBPath)
		if err != nil {
			return "", false, fmt.Errorf("resolve --db %s: %w", env.DBPath, err)
		}
	} else {
		path, err = paths.DBPath()
		if err != nil {
			return "", false, err
		}
	}
	if _, err := env.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return path, false, nil
		}
		return path, false, err
	}
	return path, true, nil
}

// hookFail prints err as the one hook error line on stderr:
// "casebook hook: <message>". The text is formatError's (app errors are
// safe to show), so it never holds record text.
func hookFail(env *Env, err error) {
	fmt.Fprintln(env.Stderr, "casebook hook: "+strings.TrimPrefix(formatError(err), "casebook: "))
}
