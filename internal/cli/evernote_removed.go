package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// evernoteRemovedMessage is what casebook evernote prints, on stderr as the
// command's one-line error, now that the Evernote integration is gone.
const evernoteRemovedMessage = "the Evernote commands were removed from Casebook. " +
	"To share a case, run casebook report (add --brief for the leadership brief) or use Share summary in the app, " +
	"then copy or save the text; or ask your AI client to save the brief or report into an app it is connected to. " +
	"See docs/sharing.md."

// errEvernoteRemoved is the error casebook evernote returns.
var errEvernoteRemoved = errors.New(evernoteRemovedMessage)

// newEvernoteRemovedCommand is a hidden transition command: scripts and
// habits that still call casebook evernote (with any subcommand, argument or
// flag) get a short pointer to the replacement instead of "unknown command".
// It does nothing else and always fails (exit 1).
func newEvernoteRemovedCommand(_ *Env) *cobra.Command {
	return &cobra.Command{
		Use:                "evernote",
		Short:              "Removed; use casebook report",
		Hidden:             true,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(*cobra.Command, []string) error {
			return errEvernoteRemoved
		},
	}
}
