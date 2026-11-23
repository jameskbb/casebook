// Command readme-example builds the demo questions in a throwaway database
// and prints the context pack of the featured one (the call center question,
// demo.FeaturedRef), so the briefing example in docs/mcp.md is real engine output
// rather than hand-written text.
//
// Usage (from the repository root):
//
//	go run ./scripts/readme-example                    # compact pack, as casebook context --format compact
//	go run ./scripts/readme-example -format md         # full Markdown pack
//	go run ./scripts/readme-example -format json
//	go run ./scripts/readme-example -db ./demo.db      # keep the database (it must not exist yet)
//
// The records are the ones the app's demo and the README images show
// (internal/demo): every write goes through internal/app with the actors a
// real run would have. The clock is fixed, so the output is byte for byte
// the same on every run.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/demo"
)

func main() {
	format := flag.String("format", "compact", "output format: compact, md or json")
	maxBytes := flag.Int("max-bytes", 0, "Markdown size budget (0: the format's default)")
	dbPath := flag.String("db", "", "database file to create (default: a temporary file that is removed)")
	flag.Parse()

	if err := run(*dbPath, *format, *maxBytes, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "readme-example:", err)
		os.Exit(1)
	}
}

// now is the fixed "today" of the example: the Thursday the story ends on.
var now = time.Date(2026, time.November, 5, 16, 0, 0, 0, time.UTC)

// run seeds the demo in dbPath (a temporary directory when empty) and
// writes the rendered pack of the featured question to w.
func run(dbPath, format string, maxBytes int, w io.Writer) error {
	if dbPath == "" {
		dir, err := os.MkdirTemp("", "casebook-readme-example-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		dbPath = filepath.Join(dir, "casebook.db")
	} else if _, err := os.Stat(dbPath); err == nil {
		return fmt.Errorf("%s already exists; pass a new path", dbPath)
	}

	name, compact := format, false
	switch format {
	case "compact":
		name, compact = "md", true
	case "md", "markdown", "json":
	default:
		return fmt.Errorf("unknown format %q (want compact, md or json)", format)
	}

	clock := demo.NewClock(demo.Start(now))
	a, err := app.Open(dbPath, app.WithClock(clock.Now))
	if err != nil {
		return err
	}
	defer a.Close()

	ctx := context.Background()
	if err := demo.Seed(ctx, a, clock, now, demo.StageFull); err != nil {
		return err
	}
	text, err := a.RenderContext(ctx, demo.FeaturedRef, name,
		app.ContextOptions{Compact: compact, MaxBytes: maxBytes})
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, text)
	return err
}
