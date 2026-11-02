package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/integrations"
)

// Next steps reported by setup codex.
const (
	codexNextSetup   = "Check with: codex mcp list (or codex mcp get casebook --json)"
	codexNextRemove  = "Restart Codex sessions to unload Casebook."
	codexNextPrint   = "Run casebook setup codex without --print to write it, or add it by hand."
	codexNextRefused = "Nothing was written; add the block above to the file by hand, or fix the file and rerun."
)

// newSetupCodexCommand registers the Casebook MCP server in Codex's
// config.toml inside a Casebook-managed block, removes that block (--remove)
// or prints it (--print). It never opens the database.
func newSetupCodexCommand(env *Env) *cobra.Command {
	var remove, print, skill bool
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Register the Casebook MCP server in Codex config.toml",
		Long: `Register the Casebook MCP server in Codex's config.toml
(~/.codex/config.toml, or $CODEX_HOME/config.toml when CODEX_HOME is set). The
[mcp_servers.casebook] table runs this casebook binary by its absolute path with
"mcp" (plus "--db <path>" when --db is given).

The table is written inside a block between "# BEGIN casebook" and
"# END casebook" lines; every byte outside the block is left as it is. A backup
of the previous file is written first, and a rerun that would change nothing
writes nothing. Lines you add inside the block (such as startup_timeout_sec
or an [mcp_servers.casebook.env] table) are kept when setup rewrites it; when
they cannot be carried over safely, setup changes nothing and says so. An
[mcp_servers.casebook] you wrote yourself is never edited: setup prints the
block for you to merge by hand and exits 1.

--skill also writes the Casebook skill to skills/casebook/SKILL.md in
~/.codex ($CODEX_HOME), so Codex can start or resume a case when you agree
to one. Codex loads MCP tools on demand and does not show the model the
Casebook server's instructions, so the skill, or the section that
"casebook setup instructions" prints for your AGENTS.md, is how Codex
learns when to use Casebook. A SKILL.md without the casebook-managed line
is yours and is never changed.

--remove deletes only the Casebook-managed block (and one adjacent blank
line) and the Casebook skill; --print shows the block and changes nothing.`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(_ *cobra.Command, _ []string) error {
			if print && remove {
				return newUsageError("--print cannot be combined with --remove")
			}
			p, err := setupPaths(env)
			if err != nil {
				return err
			}
			if remove {
				if skill {
					return newUsageError("--skill cannot be combined with --remove (--remove also removes the skill)")
				}
				return runSetupCodexRemove(env, p)
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
			opts := integrations.CodexOptions{Paths: p, DBPath: db}
			if print {
				return writeSetupReport(env, setupReport{
					Client:  "codex",
					CLIPath: cli,
					DryRun:  true,
					Preview: codexPreview(cli, opts, skill),
					Next:    codexNextPrint,
				})
			}
			change, err := integrations.SetupCodex(cli, opts)
			if err != nil {
				var refused *integrations.RefusedError
				if errors.As(err, &refused) {
					return codexRefused(env, cli, opts, err)
				}
				return err
			}
			changes := []integrations.FileChange{change}
			if skill {
				sc, err := integrations.InstallSkill(integrations.SkillPath(p.CodexHome), time.Now())
				if err != nil {
					return err
				}
				changes = append(changes, sc)
			}
			return writeSetupReport(env, setupReport{
				Client:  "codex",
				CLIPath: cli,
				Next:    codexNextSetup,
				Changes: changes,
			})
		},
	}
	cmd.Flags().BoolVar(&remove, "remove", false, "remove the Casebook-managed block and nothing else")
	cmd.Flags().BoolVar(&print, "print", false, "print the configuration block that would be added and change nothing")
	cmd.Flags().BoolVar(&skill, "skill", false, "also install the Casebook skill in the Codex home directory")
	return cmd
}

// runSetupCodexRemove is setup codex --remove. The CLI path is reported
// when known but is not needed to remove the block.
func runSetupCodexRemove(env *Env, p integrations.Paths) error {
	change, err := integrations.RemoveCodex(integrations.CodexOptions{Paths: p})
	if err != nil {
		return err
	}
	changes := []integrations.FileChange{change}
	sc, err := integrations.RemoveSkill(integrations.SkillPath(p.CodexHome))
	if err != nil {
		return err
	}
	if sc.Action == integrations.ActionRemoved {
		changes = append(changes, sc)
	}
	cli, err := setupCLIPath(env)
	if err != nil {
		cli = ""
	}
	return writeSetupReport(env, setupReport{
		Client:  "codex",
		CLIPath: cli,
		Next:    codexNextRemove,
		Changes: changes,
	})
}

// codexRefused prints the manual config on stdout (inside the JSON report
// with --json) and returns err, so the refusal itself is the one stderr
// line and the exit code is 1 (INT-05).
func codexRefused(env *Env, cli string, opts integrations.CodexOptions, err error) error {
	preview := integrations.CodexConfigPreview(cli, opts)
	if env.JSON {
		if werr := writeSetupReport(env, setupReport{
			Client:  "codex",
			CLIPath: cli,
			Preview: preview,
			Next:    codexNextRefused,
		}); werr != nil {
			return werr
		}
		return err
	}
	if _, werr := fmt.Fprint(env.Stdout, preview); werr != nil {
		return werr
	}
	return err
}

// codexPreview is the setup codex --print text: the config block, plus the
// skill when skill is set.
func codexPreview(cli string, opts integrations.CodexOptions, skill bool) string {
	text := integrations.CodexConfigPreview(cli, opts)
	if skill {
		text += "\n" + integrations.SkillPreview(integrations.SkillPath(opts.Paths.CodexHome))
	}
	return text
}
