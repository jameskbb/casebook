package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/update"
)

// updatingGuideURL is the guide a person follows to replace their copy.
const updatingGuideURL = "https://github.com/jameskbb/casebook/blob/main/docs/updating.md"

const updateLong = `Ask GitHub for the latest Casebook release and print whether it is newer
than this copy, with its release page and download link. It never downloads
or installs anything; replacing your copy is your step, described in
https://github.com/jameskbb/casebook/blob/main/docs/updating.md.

This is the one network request Casebook makes, and only when you run it: one
request to GitHub's public releases API carrying no version, identifier or
data (https://github.com/jameskbb/casebook/blob/main/docs/privacy.md). It
exits 1 when GitHub cannot be reached or lists no release.`

// newUpdateCommand is casebook update. It calls internal/update directly,
// not internal/app, and never opens the database (ADR 0022).
func newUpdateCommand(env *Env) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Check GitHub for a newer Casebook release",
		Long:  updateLong,
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := env.CheckUpdate(cmd.Context(), update.Options{
				Current: Version,
				Flavor:  update.FlavorCLI,
			})
			if err != nil {
				return err
			}
			if env.JSON {
				return writeJSON(env.Stdout, res)
			}
			return writeUpdateResult(env.Stdout, res)
		},
	}
}

// writeUpdateResult prints the human form of a check for updates.
func writeUpdateResult(w io.Writer, r update.Result) error {
	var err error
	switch r.State {
	case update.StateUpToDate:
		_, err = fmt.Fprintf(w, "Casebook %s is the latest release.\n", r.Current)
	case update.StateUpdateAvailable:
		_, err = fmt.Fprintf(w, "A newer Casebook is available: %s (you have %s).\n", r.Latest, r.Current)
		if err == nil {
			_, err = fmt.Fprintf(w, "Release page: %s\n", r.ReleaseURL)
		}
		if err == nil && r.Asset != nil {
			_, err = fmt.Fprintf(w, "Download:     %s\n", r.Asset.URL)
		}
		if err == nil {
			_, err = fmt.Fprintf(w, "Updating Casebook: %s\n", updatingGuideURL)
		}
	case update.StateAhead:
		_, err = fmt.Fprintf(w, "You have Casebook %s, which is newer than the latest release, %s.\n", r.Current, r.Latest)
	default:
		_, err = fmt.Fprintf(w, "This build (%s) is not a release version. The latest release is %s.\n", r.Current, r.Latest)
		if err == nil {
			_, err = fmt.Fprintf(w, "Release page: %s\n", r.ReleaseURL)
		}
	}
	return err
}
