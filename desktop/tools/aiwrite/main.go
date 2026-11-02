// Command aiwrite is a development helper for the Windows host smoke and the
// screenshot pass. From a separate process it writes to a Casebook database
// exactly as an AI client would (through internal/app with an MCP actor), so
// the open desktop app can be watched picking the change up (UI-22).
//
//	aiwrite --data-dir <dir> [--client claude-code] [--inv INV-0002]
//	aiwrite --data-dir <dir> --raw-detail-bytes 65536
//	aiwrite --data-dir <dir> --checkpoint
//	aiwrite --data-dir <dir> --backup-to <path>
//
// Without --inv it writes to the active investigation (in the demo seed,
// the featured INV-0002). --checkpoint saves Thursday's Claude Code
// checkpoint of the demo story (internal/demo) on a database seeded with
// --seed-demo --seed-demo-stage before-thursday, as one casebook_checkpoint
// call would.
//
// It refuses to run without --data-dir and never creates a database, so the
// real per-user Casebook data is never touched (D-08).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/demo"
)

// dbFileName is the database file inside a data dir (same name the desktop
// app and the CLI use).
const dbFileName = "casebook.db"

const (
	evidenceTitle       = "Four Q4 deals above $250k are already signed under the July playbook"
	evidenceSummary     = "The deal desk pipeline export for October 8 shows four Northeast deals above $250k signed since October 1, every one on the white-glove tier with an onboarding fee of $0; none has started onboarding yet."
	evidenceSourceLabel = "Deal desk pipeline export"
	claimStatement      = "Unless the fee is restored before delivery starts, the four signed Q4 deals add about $800k of onboarding cost at no charge."
)

// usageError marks a command line problem (exit code 2).
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

// exitCode maps a run error to the process exit code.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var u usageError
	if errors.As(err, &u) {
		return 2
	}
	return 1
}

func main() {
	err := run(os.Args[1:], os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "aiwrite:", err)
	}
	os.Exit(exitCode(err))
}

// run parses args and performs one write (or one backup), printing what it
// did to stdout.
func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("aiwrite", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dataDir := fs.String("data-dir", "", "data dir holding casebook.db (required)")
	client := fs.String("client", "claude-code", "AI client name recorded on the writes")
	invRef := fs.String("inv", "", "investigation ref (default: the active investigation)")
	backupTo := fs.String("backup-to", "", "take a backup to this path and exit")
	rawBytes := fs.Int("raw-detail-bytes", 0, "give the new evidence a raw detail of this many bytes (UI backstop for long raw detail)")
	checkpoint := fs.Bool("checkpoint", false, "save the demo's Thursday Claude Code checkpoint (needs --seed-demo-stage before-thursday)")
	if err := fs.Parse(args); err != nil {
		return usageError{err.Error()}
	}
	if fs.NArg() > 0 {
		return usageError{"unexpected argument: " + fs.Arg(0)}
	}
	if *rawBytes < 0 {
		return usageError{"--raw-detail-bytes must not be negative"}
	}
	if strings.TrimSpace(*dataDir) == "" {
		return usageError{"--data-dir is required; aiwrite never writes to the default database"}
	}
	dbPath := filepath.Join(*dataDir, dbFileName)
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("no Casebook database at %s: %w", dbPath, err)
	}

	ctx := context.Background()
	a, err := app.Open(dbPath)
	if err != nil {
		return err
	}
	defer a.Close()

	if *backupTo != "" {
		info, err := a.Backup(ctx, *backupTo)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "backup %s\n", info.Path)
		return nil
	}

	ref := *invRef
	if ref == "" {
		active, err := a.GetActiveInvestigation(ctx)
		if err != nil {
			return err
		}
		if active == nil {
			return errors.New("no active investigation; pass --inv")
		}
		ref = active.Ref
	}

	actor := app.ActorMCP(*client)
	if *checkpoint {
		return writeCheckpoint(ctx, a, actor, ref, stdout)
	}
	ev, err := a.AddEvidence(ctx, actor, ref, app.EvidenceInput{
		Title:       evidenceTitle,
		Summary:     evidenceSummary,
		SourceType:  app.SourceTypeAIToolOutput,
		SourceLabel: evidenceSourceLabel,
		RawDetail:   logLines(*rawBytes),
	})
	if err != nil {
		return err
	}
	cl, err := a.AddClaim(ctx, actor, ref, app.ClaimInput{
		Statement:  claimStatement,
		Confidence: app.ConfidenceMedium,
		Links:      []app.EvidenceLinkInput{{EvidenceRef: ev.Ref, Relation: app.RelationSupports}},
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s %s %s\n", ref, ev.Ref, cl.Ref)
	return nil
}

// writeCheckpoint saves demo.ThursdayCheckpoint on ref. It refuses an
// investigation that already has a conclusion (the before-thursday stage
// leaves it empty), so it never doubles the demo's Thursday on a fully
// seeded database.
func writeCheckpoint(ctx context.Context, a *app.App, actor app.Actor, ref string, stdout io.Writer) error {
	inv, err := a.GetInvestigation(ctx, ref)
	if err != nil {
		return err
	}
	if inv.Conclusion != "" {
		return fmt.Errorf("%s already has a conclusion; --checkpoint expects the demo seeded with --seed-demo-stage before-thursday", ref)
	}
	res, err := a.CheckpointBatch(ctx, actor, ref, demo.ThursdayCheckpoint())
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s checkpoint created %s updated %s\n", res.InvestigationRef,
		strings.Join(res.Created, ","), strings.Join(res.Updated, ","))
	// The numbers the MCP result would carry, so a mock-up of the Claude
	// Code session (the README hero) can copy them from a real run.
	since := res.PreviousCheckpointAt
	if since == "" {
		since = "the start"
	}
	fmt.Fprintf(stdout, "activity %d since %s: %d changes\n", res.ActivityID, since, len(res.Changes))
	return nil
}

// logLines returns deal desk pipeline lines cut to exactly n bytes,
// or "" for 0.
func logLines(n int) string {
	if n <= 0 {
		return ""
	}
	var b strings.Builder
	for i := 1; b.Len() < n; i++ {
		fmt.Fprintf(&b, "2026-10-%02d deal=NE-%04d acv_usd=%d onboarding_tier=white-glove fee_usd=0\n", 1+i%8, 2400+i%7600, 250000+(i%40)*10000)
	}
	return b.String()[:n]
}
