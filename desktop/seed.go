package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/demo"
)

// errSeedNotEmpty is returned by seedDemo when the database already holds
// an investigation or any activity; the demo never mixes with real data
// (D-08).
var errSeedNotEmpty = errors.New("the database is not empty; demo data was not added")

// errSeedDefault is returned by seedDemo when the target resolves to the
// per-user default database, however it was reached (--data-dir, a
// CASEBOOK_HOME or CASEBOOK_DB pointing at it, or a link to it).
var errSeedDefault = errors.New("this is the default per-user database; demo data is only added to another database chosen with --data-dir, CASEBOOK_HOME or CASEBOOK_DB")

// checkSeedTarget refuses the per-user default database. It fails closed:
// when the default cannot be located, nothing is seeded.
func checkSeedTarget(dbPath string) error {
	def, err := defaultDBPath()
	if err != nil {
		return fmt.Errorf("could not locate the default database, so demo data was not added: %w", err)
	}
	if samePath(dbPath, def) {
		return errSeedDefault
	}
	return nil
}

// seedDemo fills the empty database at dbPath with the demo investigations
// used for screenshots and smoke tests (D-08, UI-SPEC Screenshots; see
// internal/demo): the featured flaky-test case INV-0002, written by Claude
// Code, Codex and a person over Monday to Thursday, plus an earlier case, a
// blocked, a resolved and an archived investigation. stage
// demo.StageBeforeThursday stops before Thursday's Claude Code checkpoint so
// it can be written live (aiwrite --checkpoint).
func seedDemo(dbPath string, now time.Time, stage demo.Stage) error {
	if err := checkSeedTarget(dbPath); err != nil {
		return err
	}
	clock := demo.NewClock(demo.Start(now))
	a, err := app.Open(dbPath, app.WithClock(clock.Now))
	if err != nil {
		return err
	}
	defer a.Close()
	ctx := context.Background()

	existing, err := a.ListInvestigations(ctx, app.ListOptions{IncludeArchived: true, Limit: 1})
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return errSeedNotEmpty
	}
	activity, err := a.ListActivity(ctx, app.ActivityFilter{Limit: 1})
	if err != nil {
		return err
	}
	if len(activity) > 0 {
		return errSeedNotEmpty
	}
	return demo.Seed(ctx, a, clock, now, stage)
}
