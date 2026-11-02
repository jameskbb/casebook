package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/version"
)

// Version is the build version stamped into internal/version, shared with the
// desktop app so both executables always report the same value.
var Version = version.Version

// versionOutput is the --json shape of casebook version.
type versionOutput struct {
	Version string `json:"version"`
}

func newVersionCommand(env *Env) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the casebook version",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if env.JSON {
				return writeJSON(env.Stdout, versionOutput{Version: Version})
			}
			_, err := fmt.Fprintf(env.Stdout, "casebook %s\n", Version)
			return err
		},
	}
}
