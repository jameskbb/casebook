package app_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

// TestTwoAppContention runs two App instances on one file: 4 writer
// goroutines per App x 25 CreateInvestigation calls each, plus 2 readers.
// Every write must succeed with a unique ref and every activity row must land.
func TestTwoAppContention(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "casebook.db")
	a1, err := app.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a1.Close()
	a2, err := app.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a2.Close()

	if _, err := a1.CreateInvestigation(ctx, app.ActorCLI(), app.CreateInvestigationInput{Title: "seed"}); err != nil {
		t.Fatal(err)
	}

	const writersPerApp, perWriter = 4, 25
	var (
		mu       sync.Mutex
		errs     []error
		refs     = map[string]bool{}
		writers  sync.WaitGroup
		readers  sync.WaitGroup
		stop     atomic.Bool
		readsRun atomic.Int64
	)
	record := func(err error) {
		mu.Lock()
		errs = append(errs, err)
		mu.Unlock()
	}
	for _, a := range []*app.App{a1, a2} {
		for w := 0; w < writersPerApp; w++ {
			writers.Add(1)
			go func(a *app.App, w int) {
				defer writers.Done()
				actor := app.ActorMCP(fmt.Sprintf("writer-%d", w))
				for i := 0; i < perWriter; i++ {
					inv, err := a.CreateInvestigation(ctx, actor, app.CreateInvestigationInput{Title: fmt.Sprintf("w%d-%d", w, i)})
					if err != nil {
						record(err)
						continue
					}
					mu.Lock()
					if refs[inv.Ref] {
						errs = append(errs, fmt.Errorf("duplicate ref %s", inv.Ref))
					}
					refs[inv.Ref] = true
					mu.Unlock()
				}
			}(a, w)
		}
	}
	for _, a := range []*app.App{a1, a2} {
		readers.Add(1)
		go func(a *app.App) {
			defer readers.Done()
			for !stop.Load() {
				if _, err := a.GetInvestigation(ctx, "INV-0001"); err != nil {
					record(err)
					return
				}
				readsRun.Add(1)
			}
		}(a)
	}
	writers.Wait()
	stop.Store(true)
	readers.Wait()

	if len(errs) != 0 {
		t.Fatalf("%d errors under contention, first: %v", len(errs), errors.Join(errs...))
	}
	if readsRun.Load() == 0 {
		t.Fatal("readers never ran")
	}
	if len(refs) != 2*writersPerApp*perWriter {
		t.Fatalf("got %d unique refs, want %d", len(refs), 2*writersPerApp*perWriter)
	}
	for n := 2; n <= 201; n++ {
		if !refs[app.FormatInvestigationRef(int64(n))] {
			t.Fatalf("missing ref INV-%04d", n)
		}
	}
	if _, err := a2.GetInvestigation(ctx, "INV-0202"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("INV-0202 = %v, want ErrNotFound", err)
	}
	acts, err := a2.ListActivity(ctx, app.ActivityFilter{Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	creates := 0
	for _, act := range acts {
		if act.Action == "investigation.create" {
			creates++
		}
	}
	if creates != 201 {
		t.Fatalf("investigation.create rows = %d, want 201", creates)
	}
}
