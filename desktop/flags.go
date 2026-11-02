package main

import (
	"errors"
	"flag"
	"io"
	"strings"

	"github.com/jameskbb/casebook/internal/demo"
)

// flags holds the command line options of the desktop app (D-08).
type flags struct {
	// DataDir opens <DataDir>/casebook.db instead of the per-user default.
	DataDir string
	// SeedDemo seeds demo data into an empty, non-default database.
	SeedDemo bool
	// SeedDemoStage is how much of the demo story to seed (internal/demo):
	// full, or before-thursday for the live-write screenshots.
	SeedDemoStage demo.Stage
}

// parseFlags parses the desktop command line. It never prints: usage and
// errors are returned so main decides what to show. Arguments starting with
// -psn_ (the process serial number macOS Finder passes) are ignored.
func parseFlags(args []string) (flags, error) {
	var cfg flags
	fs := flag.NewFlagSet("casebook", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.DataDir, "data-dir", "", "open <dir>/casebook.db instead of the per-user database")
	fs.BoolVar(&cfg.SeedDemo, "seed-demo", false, "seed demo data into an empty, non-default database")
	stage := fs.String("seed-demo-stage", "", "with --seed-demo: full (default) or before-thursday")

	kept := make([]string, 0, len(args))
	for _, a := range args {
		if strings.HasPrefix(a, "-psn_") {
			continue
		}
		kept = append(kept, a)
	}
	if err := fs.Parse(kept); err != nil {
		return flags{}, err
	}
	if fs.NArg() > 0 {
		return flags{}, errors.New("unexpected argument: " + fs.Arg(0))
	}
	cfg.DataDir = strings.TrimSpace(cfg.DataDir)
	if *stage != "" && !cfg.SeedDemo {
		return flags{}, errors.New("--seed-demo-stage needs --seed-demo")
	}
	st, err := demo.ParseStage(*stage)
	if err != nil {
		return flags{}, err
	}
	cfg.SeedDemoStage = st
	return cfg, nil
}

// usesExplicitDB reports whether the database is chosen by --data-dir or by
// CASEBOOK_DB or CASEBOOK_HOME rather than the per-user default. AI clients
// may start casebook without this process's environment, so the Connect AI
// clients panel then passes the path with --db.
func usesExplicitDB(cfg flags, getenv func(string) string) bool {
	return cfg.DataDir != "" || strings.TrimSpace(getenv("CASEBOOK_DB")) != "" || strings.TrimSpace(getenv("CASEBOOK_HOME")) != ""
}
