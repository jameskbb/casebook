package app_test

// Cross-process proofs (D-29, TEST-03, STORE-04, STORE-07, DATA-03, DATA-04).
// The test binary re-executes itself (os.Args[0]) as child processes that
// run TestHelperProcess in a mode chosen by environment variables, so real
// OS processes share one SQLite file exactly as the desktop app, the CLI and
// several MCP servers will.

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// Environment variables a child process reads. They exist only in this test
// file, which is never compiled into a release binary.
const (
	envHelper = "CASEBOOK_TEST_HELPER"
	envDB     = "CASEBOOK_TEST_DB"
	envMode   = "CASEBOOK_TEST_MODE"
	envN      = "CASEBOOK_TEST_N"
	envInv    = "CASEBOOK_TEST_INV"
	envStop   = "CASEBOOK_TEST_STOP"
	envWorker = "CASEBOOK_TEST_WORKER"
)

// childTimeout bounds every child process.
const childTimeout = 60 * time.Second

// writeLoopMax caps the write-loop mode; writeLoopPause spaces its writes so
// the loop is still running while the parent takes backups.
const (
	writeLoopMax   = 500
	writeLoopPause = 2 * time.Millisecond
)

// TestHelperProcess is the child side of the multi-process tests. It skips
// unless CASEBOOK_TEST_HELPER=1, then opens CASEBOOK_TEST_DB with default
// options (real clock, default busy timeout and retry budget) and runs
// CASEBOOK_TEST_MODE. Any error fails the test, so the child exits non-zero.
func TestHelperProcess(t *testing.T) {
	if os.Getenv(envHelper) != "1" {
		t.Skip("child process helper for the multi-process tests")
	}
	ctx := context.Background()
	a, err := app.Open(os.Getenv(envDB))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer a.Close()
	inv := os.Getenv(envInv)
	worker := os.Getenv(envWorker)
	if worker == "" {
		worker = "child"
	}

	switch mode := os.Getenv(envMode); mode {
	case "mixed":
		n, err := strconv.Atoi(os.Getenv(envN))
		if err != nil {
			t.Fatalf("%s: %v", envN, err)
		}
		if err := runMixed(ctx, a, worker, inv, n); err != nil {
			t.Fatal(err)
		}
	case "add-evidence":
		if _, err := a.AddEvidence(ctx, app.ActorMCP(worker), inv, evidenceInput(worker, 0)); err != nil {
			t.Fatal(err)
		}
	case "set-active":
		if _, err := a.SetActiveInvestigation(ctx, app.ActorCLI(), inv); err != nil {
			t.Fatal(err)
		}
	case "get-active":
		active, err := a.GetActiveInvestigation(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if active == nil {
			fmt.Println("ACTIVE=none")
		} else {
			fmt.Println("ACTIVE=" + active.Ref)
		}
	case "write-loop":
		stop := os.Getenv(envStop)
		writes := 0
		for writes < writeLoopMax {
			if _, err := os.Stat(stop); err == nil {
				break
			}
			if _, err := a.AddEvidence(ctx, app.ActorMCP(worker), inv, evidenceInput(worker, writes)); err != nil {
				t.Fatal(err)
			}
			writes++
			time.Sleep(writeLoopPause)
		}
		fmt.Printf("WRITES=%d\n", writes)
	case "count":
		ev, err := a.ListEvidence(ctx, inv)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("EVIDENCE=%d\n", len(ev))
	default:
		t.Fatalf("unknown %s %q", envMode, mode)
	}
}

// evidenceInput is the evidence every writer adds; its summary contains
// "debug" so the readers' Search has something to find.
func evidenceInput(worker string, i int) app.EvidenceInput {
	return app.EvidenceInput{
		Title:      fmt.Sprintf("%s evidence %d", worker, i),
		Summary:    "Debug logging observation recorded by " + worker,
		SourceType: app.SourceTypeManualObservation,
	}
}

// runMixed is one writer's workload, shared by the parent and the children:
// one CreateInvestigation, then n iterations of AddEvidence to the shared
// investigation followed by AddClaim linking that evidence as supports.
func runMixed(ctx context.Context, a *app.App, worker, inv string, n int) error {
	actor := app.ActorMCP(worker)
	if _, err := a.CreateInvestigation(ctx, actor, app.CreateInvestigationInput{Title: "Worker " + worker}); err != nil {
		return fmt.Errorf("%s create: %w", worker, err)
	}
	for i := 0; i < n; i++ {
		ev, err := a.AddEvidence(ctx, actor, inv, evidenceInput(worker, i))
		if err != nil {
			return fmt.Errorf("%s evidence %d: %w", worker, i, err)
		}
		_, err = a.AddClaim(ctx, actor, inv, app.ClaimInput{
			Statement: fmt.Sprintf("%s claim %d", worker, i),
			Links:     []app.EvidenceLinkInput{{EvidenceRef: ev.Ref, Relation: app.RelationSupports}},
		})
		if err != nil {
			return fmt.Errorf("%s claim %d: %w", worker, i, err)
		}
	}
	return nil
}

// child is a running copy of the test binary in TestHelperProcess.
type child struct {
	cmd  *exec.Cmd
	out  *bytes.Buffer
	done chan struct{}
	err  error
}

// spawn starts os.Args[0] -test.run=^TestHelperProcess$ with
// CASEBOOK_TEST_HELPER=1 and env added to os.Environ(), capturing combined
// output. Every child runs under a 60 second context, and a child still
// running when the test ends is killed and reaped, so none is left behind.
func spawn(t *testing.T, ctx context.Context, env map[string]string) *child {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, childTimeout)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHelperProcess$", "-test.count=1")
	cmd.Env = append(os.Environ(), envHelper+"=1")
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	c := &child{cmd: cmd, out: &bytes.Buffer{}, done: make(chan struct{})}
	cmd.Stdout = c.out
	cmd.Stderr = c.out
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start child: %v", err)
	}
	go func() {
		c.err = cmd.Wait()
		cancel()
		close(c.done)
	}()
	t.Cleanup(func() {
		select {
		case <-c.done:
		default:
			_ = cmd.Process.Kill()
			<-c.done
		}
	})
	return c
}

// wait blocks until the child exits and returns its output and exit error.
func (c *child) wait() (string, error) {
	<-c.done
	return c.out.String(), c.err
}

// exited reports whether the child has already exited.
func (c *child) exited() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// runChild runs one child to completion, failing the test unless it exits 0
// with output free of raw lock text. It returns the output.
func runChild(t *testing.T, env map[string]string) string {
	t.Helper()
	out, err := spawn(t, context.Background(), env).wait()
	if err != nil {
		t.Fatalf("child %s failed: %v\n%s", env[envMode], err, out)
	}
	assertNoLockText(t, "child "+env[envMode], out)
	return out
}

// assertNoLockText fails when s contains raw lock or SQLite text.
func assertNoLockText(t *testing.T, who, s string) {
	t.Helper()
	low := strings.ToLower(s)
	if strings.Contains(low, "locked") || strings.Contains(low, "sqlite") {
		t.Fatalf("%s output has raw lock or SQLite text:\n%s", who, s)
	}
}

// outputValue returns the value of the first "KEY=value" line in out.
func outputValue(t *testing.T, out, key string) string {
	t.Helper()
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), key+"="); ok {
			return v
		}
	}
	t.Fatalf("child output has no %s= line:\n%s", key, out)
	return ""
}

// TestMultiProcessContention runs 3 child processes and the parent as
// concurrent writers on one file (30 evidence plus 30 linked claims each,
// all on one shared investigation) while 4 parent goroutines read. Every
// write must land exactly once with contiguous refs and no raw lock error.
func TestMultiProcessContention(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "casebook.db")
	a, err := app.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	shared, err := a.CreateInvestigation(ctx, app.ActorCLI(), app.CreateInvestigationInput{Title: "Shared contention target"})
	if err != nil {
		t.Fatal(err)
	}

	const children, perWriter, readers = 3, 30, 4
	const writers = children + 1
	kids := make([]*child, children)
	for i := range kids {
		kids[i] = spawn(t, ctx, map[string]string{
			envDB:     path,
			envMode:   "mixed",
			envN:      strconv.Itoa(perWriter),
			envInv:    shared.Ref,
			envWorker: fmt.Sprintf("child-%d", i+1),
		})
	}

	var (
		mu       sync.Mutex
		errs     []error
		stop     atomic.Bool
		reads    atomic.Int64
		readerWG sync.WaitGroup
	)
	record := func(err error) {
		mu.Lock()
		errs = append(errs, err)
		mu.Unlock()
	}
	for r := 0; r < readers; r++ {
		readerWG.Add(1)
		go func() {
			defer readerWG.Done()
			for !stop.Load() {
				if _, err := a.GetInvestigationDetail(ctx, shared.Ref, app.DetailOptions{}); err != nil {
					record(fmt.Errorf("detail: %w", err))
					return
				}
				if _, err := a.Search(ctx, "debug", app.SearchOptions{}); err != nil {
					record(fmt.Errorf("search: %w", err))
					return
				}
				if _, err := a.ChangeToken(ctx); err != nil {
					record(fmt.Errorf("change token: %w", err))
					return
				}
				reads.Add(1)
			}
		}()
	}

	parentDone := make(chan error, 1)
	go func() { parentDone <- runMixed(ctx, a, "parent", shared.Ref, perWriter) }()

	outs := make([]string, children)
	childErrs := make([]error, children)
	for i, k := range kids {
		outs[i], childErrs[i] = k.wait()
	}
	if err := <-parentDone; err != nil {
		record(err)
	}
	stop.Store(true)
	readerWG.Wait()

	for i := range kids {
		if childErrs[i] != nil {
			t.Errorf("child-%d exited with %v:\n%s", i+1, childErrs[i], outs[i])
		}
		assertNoLockText(t, fmt.Sprintf("child-%d", i+1), outs[i])
	}
	if len(errs) != 0 {
		joined := errors.Join(errs...)
		assertNoLockText(t, "parent", joined.Error())
		t.Fatalf("%d parent errors, first: %v", len(errs), joined)
	}
	if t.Failed() {
		t.FailNow()
	}
	if reads.Load() == 0 {
		t.Fatal("parent readers never completed a read")
	}

	const total = writers * perWriter
	evidence, err := a.ListEvidence(ctx, shared.Ref)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := a.ListClaims(ctx, shared.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != total || len(claims) != total {
		t.Fatalf("evidence %d, claims %d; want %d each", len(evidence), len(claims), total)
	}
	for i := 0; i < total; i++ {
		if want := app.FormatRecordRef(app.RecordEvidence, int64(i+1)); evidence[i].Ref != want {
			t.Fatalf("evidence[%d] = %s, want %s (refs must be unique and contiguous)", i, evidence[i].Ref, want)
		}
		if want := app.FormatRecordRef(app.RecordClaim, int64(i+1)); claims[i].Ref != want {
			t.Fatalf("claims[%d] = %s, want %s (refs must be unique and contiguous)", i, claims[i].Ref, want)
		}
	}
	links := 0
	for _, c := range claims {
		if len(c.Links) != 1 || c.Links[0].Relation != app.RelationSupports {
			t.Fatalf("%s links = %+v, want one supports link", c.Ref, c.Links)
		}
		// Each claim links the evidence its own writer added just before it.
		worker, n, _ := strings.Cut(c.Statement, " claim ")
		if want := worker + " evidence " + n; c.Links[0].EvidenceTitle != want {
			t.Fatalf("%s (%q) links %q, want %q", c.Ref, c.Statement, c.Links[0].EvidenceTitle, want)
		}
		links++
	}
	if links != total {
		t.Fatalf("links = %d, want %d", links, total)
	}

	st, err := a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// 1 shared create + 4 worker creates + 2 records per iteration.
	wantActivity := 1 + writers + 2*total
	if st.Counts.Investigations != 1+writers || st.Counts.Evidence != total ||
		st.Counts.Claims != total || st.Counts.ClaimLinks != total || st.Counts.Activity != wantActivity {
		t.Fatalf("counts = %+v; want investigations %d, evidence/claims/claim_links %d, activity %d",
			st.Counts, 1+writers, total, wantActivity)
	}
	if err := a.CheckIntegrity(ctx); err != nil {
		t.Fatalf("integrity: %v", err)
	}
}

// TestChangeTokenCrossProcess proves the change token moves when another
// process writes and stays equal when nothing is written (D-17, STORE-07).
func TestChangeTokenCrossProcess(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "casebook.db")
	a, err := app.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	inv, err := a.CreateInvestigation(ctx, app.ActorCLI(), app.CreateInvestigationInput{Title: "Token target"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := a.ChangeToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := a.ChangeToken(ctx); err != nil || again != before {
		t.Fatalf("idle token %q, %v; want %q", again, err, before)
	}

	runChild(t, map[string]string{envDB: path, envMode: "add-evidence", envInv: inv.Ref})

	after, err := a.ChangeToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after == before {
		t.Fatalf("token stayed %q after another process wrote", after)
	}
	third, err := a.ChangeToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if third != after {
		t.Fatalf("token moved from %q to %q with no write", after, third)
	}
}

// TestActiveAcrossProcesses proves the active investigation is one shared
// setting: set by a child it is read by the parent, set by the parent it is
// read by a child, and two Apps in one process agree (D-29, APP-02).
func TestActiveAcrossProcesses(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "casebook.db")
	a, err := app.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	for _, title := range []string{"First", "Second"} {
		if _, err := a.CreateInvestigation(ctx, app.ActorCLI(), app.CreateInvestigationInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	runChild(t, map[string]string{envDB: path, envMode: "set-active", envInv: "INV-0002"})
	active, err := a.GetActiveInvestigation(ctx)
	if err != nil || active == nil || active.Ref != "INV-0002" {
		t.Fatalf("parent active after child set = %+v, %v; want INV-0002", active, err)
	}
	if inv, err := a.GetInvestigation(ctx, ""); err != nil || inv.Ref != "INV-0002" {
		t.Fatalf(`GetInvestigation("") = %s, %v; want INV-0002`, inv.Ref, err)
	}

	if _, err := a.SetActiveInvestigation(ctx, app.ActorDesktop(), "INV-0001"); err != nil {
		t.Fatal(err)
	}
	out := runChild(t, map[string]string{envDB: path, envMode: "get-active"})
	if got := outputValue(t, out, "ACTIVE"); got != "INV-0001" {
		t.Fatalf("child read active %q, want INV-0001", got)
	}

	b, err := app.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	if got, err := b.GetActiveInvestigation(ctx); err != nil || got == nil || got.Ref != "INV-0001" {
		t.Fatalf("second App active = %+v, %v; want INV-0001", got, err)
	}
	if _, err := b.SetActiveInvestigation(ctx, app.ActorCLI(), "INV-0002"); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetActiveInvestigation(ctx); err != nil || got == nil || got.Ref != "INV-0002" {
		t.Fatalf("first App active after second App set = %+v, %v; want INV-0002", got, err)
	}

	if err := a.ClearActiveInvestigation(ctx, app.ActorDesktop()); err != nil {
		t.Fatal(err)
	}
	out = runChild(t, map[string]string{envDB: path, envMode: "get-active"})
	if got := outputValue(t, out, "ACTIVE"); got != "none" {
		t.Fatalf("child read active %q after clear, want none", got)
	}
}

// TestBackupWhileOtherProcessWrites takes backups while a child process keeps
// writing: every backup validates with an evidence count between the start
// and the end, and restoring the last one into a fresh database is clean
// (D-22, DATA-03, DATA-04).
func TestBackupWhileOtherProcessWrites(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	st, err := a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	start := st.Counts.Evidence

	dir := t.TempDir()
	stopFile := filepath.Join(dir, "stop")
	writer := spawn(t, ctx, map[string]string{
		envDB:     a.DBPath(),
		envMode:   "write-loop",
		envInv:    f.InvestigationRef,
		envStop:   stopFile,
		envWorker: "backup-writer",
	})

	deadline := time.Now().Add(30 * time.Second)
	for {
		st, err := a.Status(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if st.Counts.Evidence >= start+5 {
			break
		}
		if writer.exited() {
			out, err := writer.wait()
			t.Fatalf("writer exited early (%v):\n%s", err, out)
		}
		if time.Now().After(deadline) {
			t.Fatalf("writer added only %d evidence in 30s", st.Counts.Evidence-start)
		}
		time.Sleep(5 * time.Millisecond)
	}

	var backups []app.BackupInfo
	for i := 0; i < 3; i++ {
		info, err := a.Backup(ctx, filepath.Join(dir, fmt.Sprintf("during-%d.db", i)))
		if err != nil {
			t.Fatalf("backup %d while another process writes: %v", i, err)
		}
		backups = append(backups, info)
		time.Sleep(20 * time.Millisecond)
	}
	stillRunning := !writer.exited()
	if err := os.WriteFile(stopFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := writer.wait()
	if err != nil {
		t.Fatalf("writer failed: %v\n%s", err, out)
	}
	assertNoLockText(t, "writer", out)
	writes, err := strconv.Atoi(outputValue(t, out, "WRITES"))
	if err != nil {
		t.Fatal(err)
	}
	st, err = a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	final := st.Counts.Evidence
	if final != start+writes {
		t.Fatalf("final evidence %d, want %d + %d writes", final, start, writes)
	}
	if !stillRunning {
		t.Logf("writer finished its %d writes before the last backup", writeLoopMax)
	}

	for i, b := range backups {
		v, err := a.ValidateBackup(ctx, b.Path)
		if err != nil {
			t.Fatalf("backup %d does not validate: %v", i, err)
		}
		if v.Counts != b.Counts {
			t.Fatalf("backup %d counts %+v, validated %+v", i, b.Counts, v.Counts)
		}
		if ev := v.Counts.Evidence; ev < start+5 || ev > final {
			t.Fatalf("backup %d has %d evidence, want between %d and %d", i, ev, start+5, final)
		}
		if i > 0 && v.Counts.Evidence < backups[i-1].Counts.Evidence {
			t.Fatalf("backup %d has fewer evidence than backup %d", i, i-1)
		}
	}

	last := backups[len(backups)-1]
	fresh := apptest.Reopen(t, filepath.Join(t.TempDir(), "restored", "casebook.db"))
	res, err := fresh.Restore(ctx, apptest.Human, last.Path)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if err := fresh.CheckIntegrity(ctx); err != nil {
		t.Fatalf("restored database integrity: %v", err)
	}
	got, err := fresh.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Restore adds its own activity row after replacing the contents.
	want := last.Counts
	want.Activity++
	if got.Counts != want || res.Counts != want {
		t.Fatalf("restored counts %+v (result %+v), want backup counts plus the restore row %+v", got.Counts, res.Counts, want)
	}
}

// TestRestoreSeenByOtherProcess restores a backup in the parent and proves a
// separate process then reads the restored state, while the parent's change
// token moves (D-22, DATA-04, STORE-07).
func TestRestoreSeenByOtherProcess(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	backup, err := a.Backup(ctx, filepath.Join(t.TempDir(), "seed.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddEvidence(ctx, apptest.AI, f.InvestigationRef, evidenceInput("after-backup", 0)); err != nil {
		t.Fatal(err)
	}
	out := runChild(t, map[string]string{envDB: a.DBPath(), envMode: "count", envInv: f.InvestigationRef})
	if got := outputValue(t, out, "EVIDENCE"); got != strconv.Itoa(backup.Counts.Evidence+1) {
		t.Fatalf("child counted %s evidence before restore, want %d", got, backup.Counts.Evidence+1)
	}

	before, err := a.ChangeToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Restore(ctx, apptest.Human, backup.Path); err != nil {
		t.Fatalf("restore: %v", err)
	}
	after, err := a.ChangeToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after == before {
		t.Fatalf("change token stayed %q across a restore", after)
	}

	out = runChild(t, map[string]string{envDB: a.DBPath(), envMode: "count", envInv: f.InvestigationRef})
	if got := outputValue(t, out, "EVIDENCE"); got != strconv.Itoa(backup.Counts.Evidence) {
		t.Fatalf("child counted %s evidence after restore, want the backup's %d", got, backup.Counts.Evidence)
	}
}
