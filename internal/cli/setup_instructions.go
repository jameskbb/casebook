package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/integrations"
)

// newSetupInstructionsCommand prints the CLAUDE.md / AGENTS.md section that
// tells an AI client when to use Casebook. It writes nothing (ADR-0009):
// project instruction files belong to the person.
func newSetupInstructionsCommand(env *Env) *cobra.Command {
	return &cobra.Command{
		Use:   "instructions",
		Short: "Print a CLAUDE.md or AGENTS.md section that tells AI clients when to use Casebook",
		Long: `Print a short Markdown section for a project's CLAUDE.md (Claude Code) or
AGENTS.md (Codex and other clients) that tells the AI when to use Casebook:
offer to open a case when a task turns into an investigation and open it once
you agree, read the case before resuming earlier work, and record findings
as it goes. Nothing is written; paste it where you want it, for example:

  casebook setup instructions >> AGENTS.md

With --json the section is the "text" member of a JSON object.`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(_ *cobra.Command, _ []string) error {
			if env.JSON {
				return writeJSON(env.Stdout, map[string]string{"text": integrations.InstructionsSnippet})
			}
			_, err := fmt.Fprint(env.Stdout, integrations.InstructionsSnippet)
			return err
		},
	}
}
