package cli

import "github.com/spf13/cobra"

// commandFactory builds one top-level command bound to env.
type commandFactory func(env *Env) *cobra.Command

// commandFactories is the single registry of top-level commands, in help
// order. Phase 4 adds setup, hook and doctor here.
//
// Never use cobra's built-in required-flag or mutually-exclusive-flag helpers
// anywhere in internal/cli: their errors bypass the flag error func and would
// exit 1 instead of 2. Commands check required and exclusive flags themselves
// in RunE and return newUsageError.
func commandFactories() []commandFactory {
	return []commandFactory{
		newMCPCommand,
		newStatusCommand,
		newInvestigationCommand,
		newContextCommand,
		newReportCommand,
		newSearchCommand,
		newExportCommand,
		newImportCommand,
		newBackupCommand,
		newRestoreCommand,
		newEvernoteRemovedCommand,
		newUICommand,
		newSetupCommand,
		newHookCommand,
		newDoctorCommand,
		newUpdateCommand,
		newVersionCommand,
	}
}
