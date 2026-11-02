package cli

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/app"
)

// newContextCommand prints the deterministic context pack of an investigation
// (the active one when no ref is given). The text is exactly what
// app.RenderContext returns, so the CLI, the MCP server and hooks agree byte
// for byte given the same format and budget. The defaults differ: Markdown
// here has no size limit unless --max-bytes is set, while
// casebook_get_context defaults to 24000 bytes, so a larger pack differs
// between the two unless both pass the same --max-bytes and max_bytes.
func newContextCommand(env *Env) *cobra.Command {
	var (
		format   string
		maxBytes int
	)
	cmd := &cobra.Command{
		Use:   "context [ref]",
		Short: "Print a case briefing for an AI session (default: the active case)",
		Long: `Print a case briefing for an AI session: the conclusion, current and
disputed claims, explanations still being tested, what was ruled out, open
questions, an evidence index and recent changes. With no ref, use the active case.

Formats: md (or markdown), compact (Markdown sized for session hooks, 6000
bytes unless --max-bytes is set) and json. --json is the same as --format json.
--max-bytes bounds the Markdown size (0 means no limit, or the compact default).`,
		Example: `  casebook context
  casebook context INV-0002 --format compact
  casebook context --json`,
		Args: usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, compact, err := contextFormat(format, cmd.Flags().Changed("format"), env.JSON)
			if err != nil {
				return err
			}
			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			a, err := env.App()
			if err != nil {
				return err
			}
			text, err := a.RenderContext(cmd.Context(), ref, name,
				app.ContextOptions{Compact: compact, MaxBytes: maxBytes})
			if err != nil {
				return err
			}
			return writeText(env.Stdout, text)
		},
	}
	cmd.Flags().StringVar(&format, "format", "md", "output format: md, markdown, compact or json")
	cmd.Flags().IntVar(&maxBytes, "max-bytes", 0, "maximum Markdown size in bytes (0 means the default)")
	return cmd
}

// contextFormat resolves --format and --json to the RenderContext format and
// the compact option. An explicit non-json --format with --json is a usage
// error; an unknown format is a validation error on format.
func contextFormat(format string, formatSet, jsonFlag bool) (name string, compact bool, err error) {
	f := strings.ToLower(strings.TrimSpace(format))
	if jsonFlag {
		if formatSet && f != "json" {
			return "", false, newUsageError("--json conflicts with --format %s", format)
		}
		return "json", false, nil
	}
	switch f {
	case "md", "markdown":
		return "markdown", false, nil
	case "compact":
		return "markdown", true, nil
	case "json":
		return "json", false, nil
	}
	return "", false, app.NewValidationError("format", "must be one of md, json, compact")
}

// writeText writes s unchanged, adding a newline only when s does not
// already end with one.
func writeText(w io.Writer, s string) error {
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	_, err := io.WriteString(w, s)
	return err
}
