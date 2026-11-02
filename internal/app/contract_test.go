package app_test

// Contract tests (D-16, D-17, D-18, D-30): the internal/app method names and
// signatures phases 2 and 3 code against, the JSON shape of every DTO, and
// docs/api.md coverage.

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// engineContract lists every D-16/D-17 method of *app.App with its exact
// signature. Renaming a method or changing a parameter or result type breaks
// the build here before it breaks a surface.
type engineContract interface {
	// Investigations
	CreateInvestigation(ctx context.Context, actor app.Actor, in app.CreateInvestigationInput) (app.Investigation, error)
	GetInvestigation(ctx context.Context, invRef string) (app.Investigation, error)
	GetInvestigationDetail(ctx context.Context, invRef string, o app.DetailOptions) (app.InvestigationDetail, error)
	ListInvestigations(ctx context.Context, o app.ListOptions) ([]app.InvestigationSummary, error)
	UpdateInvestigation(ctx context.Context, actor app.Actor, invRef string, p app.InvestigationPatch) (app.Investigation, error)
	ArchiveInvestigation(ctx context.Context, actor app.Actor, invRef string) (app.Investigation, error)
	UnarchiveInvestigation(ctx context.Context, actor app.Actor, invRef string) (app.Investigation, error)
	UpdateConclusion(ctx context.Context, actor app.Actor, invRef string, in app.ConclusionInput) (app.Investigation, error)
	DeleteInvestigation(ctx context.Context, actor app.Actor, invRef string) error

	// Human review
	GetReviewFindings(ctx context.Context, invRef string) ([]app.ReviewFinding, error)
	ReviewStatement(ctx context.Context, actor app.Actor, invRef string, in app.ReviewStatementInput) (app.ReviewFinding, error)

	// Evidence
	AddEvidence(ctx context.Context, actor app.Actor, invRef string, in app.EvidenceInput) (app.Evidence, error)
	GetEvidence(ctx context.Context, invRef, evidenceRef string, o app.GetEvidenceOptions) (app.Evidence, error)
	UpdateEvidence(ctx context.Context, actor app.Actor, invRef, evidenceRef string, p app.EvidencePatch) (app.Evidence, error)
	UpdateEvidenceOutcome(ctx context.Context, actor app.Actor, invRef, evidenceRef string, p app.EvidencePatch) (app.Evidence, app.UpdateOutcome, error)
	ListEvidence(ctx context.Context, invRef string) ([]app.Evidence, error)
	SetEvidenceVerification(ctx context.Context, actor app.Actor, invRef, evidenceRef string, v app.Verification) (app.Evidence, error)
	DeleteEvidence(ctx context.Context, actor app.Actor, invRef, evidenceRef string) error

	// Claims
	AddClaim(ctx context.Context, actor app.Actor, invRef string, in app.ClaimInput) (app.Claim, error)
	GetClaim(ctx context.Context, invRef, claimRef string) (app.Claim, error)
	ListClaims(ctx context.Context, invRef string) ([]app.Claim, error)
	UpdateClaim(ctx context.Context, actor app.Actor, invRef, claimRef string, p app.ClaimPatch) (app.Claim, error)
	UpdateClaimOutcome(ctx context.Context, actor app.Actor, invRef, claimRef string, p app.ClaimPatch) (app.Claim, app.UpdateOutcome, error)
	SetClaimVerification(ctx context.Context, actor app.Actor, invRef, claimRef string, v app.Verification) (app.Claim, error)
	SupersedeClaim(ctx context.Context, actor app.Actor, invRef, claimRef, replacementRef string) (app.Claim, error)
	DeleteClaim(ctx context.Context, actor app.Actor, invRef, claimRef string) error

	// Links
	LinkEvidenceToClaim(ctx context.Context, actor app.Actor, invRef, claimRef, evidenceRef string, rel app.Relation) (app.Claim, error)
	UnlinkEvidenceFromClaim(ctx context.Context, actor app.Actor, invRef, claimRef, evidenceRef string) (app.Claim, error)
	LinkEvidenceToHypothesis(ctx context.Context, actor app.Actor, invRef, hypRef, evidenceRef string, rel app.Relation) (app.Hypothesis, error)
	UnlinkEvidenceFromHypothesis(ctx context.Context, actor app.Actor, invRef, hypRef, evidenceRef string) (app.Hypothesis, error)

	// Hypotheses
	AddHypothesis(ctx context.Context, actor app.Actor, invRef string, in app.HypothesisInput) (app.Hypothesis, error)
	GetHypothesis(ctx context.Context, invRef, hypRef string) (app.Hypothesis, error)
	ListHypotheses(ctx context.Context, invRef string) ([]app.Hypothesis, error)
	UpdateHypothesis(ctx context.Context, actor app.Actor, invRef, hypRef string, p app.HypothesisPatch) (app.Hypothesis, error)
	UpdateHypothesisOutcome(ctx context.Context, actor app.Actor, invRef, hypRef string, p app.HypothesisPatch) (app.Hypothesis, app.UpdateOutcome, error)
	SetHypothesisVerification(ctx context.Context, actor app.Actor, invRef, hypRef string, v app.Verification) (app.Hypothesis, error)
	SupersedeHypothesis(ctx context.Context, actor app.Actor, invRef, hypRef, replacementRef string) (app.Hypothesis, error)
	DeleteHypothesis(ctx context.Context, actor app.Actor, invRef, hypRef string) error

	// Questions
	AddQuestion(ctx context.Context, actor app.Actor, invRef string, in app.QuestionInput) (app.Question, error)
	GetQuestion(ctx context.Context, invRef, qRef string) (app.Question, error)
	ListQuestions(ctx context.Context, invRef string) ([]app.Question, error)
	UpdateQuestion(ctx context.Context, actor app.Actor, invRef, qRef, text string) (app.Question, error)
	ResolveQuestion(ctx context.Context, actor app.Actor, invRef, qRef, resolution string) (app.Question, error)
	DeferQuestion(ctx context.Context, actor app.Actor, invRef, qRef, note string) (app.Question, error)
	ReopenQuestion(ctx context.Context, actor app.Actor, invRef, qRef string) (app.Question, error)
	DeleteQuestion(ctx context.Context, actor app.Actor, invRef, qRef string) error

	// Activity and the active investigation
	ListActivity(ctx context.Context, f app.ActivityFilter) ([]app.Activity, error)
	GetActiveInvestigation(ctx context.Context) (*app.Investigation, error)
	SetActiveInvestigation(ctx context.Context, actor app.Actor, invRef string) (app.Investigation, error)
	ClearActiveInvestigation(ctx context.Context, actor app.Actor) error
	ClearActiveInvestigationIf(ctx context.Context, actor app.Actor, invRef string) (bool, error)
	SetGlobalActiveInvestigation(ctx context.Context, actor app.Actor, invRef string) (app.Investigation, error)
	ActiveInvestigationInfo(ctx context.Context) (app.ActiveInfo, error)
	BindDirectory(ctx context.Context, actor app.Actor, dir, invRef string) (app.DirectoryBinding, error)
	UnbindDirectory(ctx context.Context, actor app.Actor, dir string) (string, error)
	ListDirectoryBindings(ctx context.Context) ([]app.DirectoryBinding, error)
	WorkDir() string

	// Search, context pack, checkpoint
	Search(ctx context.Context, query string, o app.SearchOptions) ([]app.SearchHit, error)
	ContextPack(ctx context.Context, invRef string, o app.ContextOptions) (app.Pack, error)
	RenderContext(ctx context.Context, invRef, format string, o app.ContextOptions) (string, error)
	Report(ctx context.Context, invRef string, o app.ReportOptions) (string, error)
	ReportHTML(ctx context.Context, invRef string, o app.ReportOptions) (string, error)
	SharePreview(ctx context.Context, invRef string, o app.ShareOptions) (app.SharePreview, error)
	Checkpoint(ctx context.Context, actor app.Actor, invRef, note string) (app.CheckpointResult, error)
	CheckpointBatch(ctx context.Context, actor app.Actor, invRef string, in app.CheckpointInput) (app.CheckpointResult, error)

	// Portability, backup and restore
	ExportInvestigation(ctx context.Context, invRef string) ([]byte, error)
	ImportInvestigation(ctx context.Context, actor app.Actor, data []byte) (app.ImportResult, error)
	ImportInvestigationFrom(ctx context.Context, actor app.Actor, r io.Reader) (app.ImportResult, error)
	Backup(ctx context.Context, destPath string) (app.BackupInfo, error)
	ValidateBackup(ctx context.Context, path string) (app.BackupInfo, error)
	Restore(ctx context.Context, actor app.Actor, srcPath string) (app.RestoreResult, error)

	// Live refresh, status and diagnostics, lifecycle
	ChangeToken(ctx context.Context) (string, error)
	Status(ctx context.Context) (app.StatusInfo, error)
	CheckIntegrity(ctx context.Context) error
	Close() error
	DBPath() string
}

// Compile-time contract assertions: the method set and the D-16
// constructors and options.
var (
	_ engineContract = (*app.App)(nil)

	_ func(string, ...app.Option) (*app.App, error) = app.Open
	_ func(...app.Option) (*app.App, error)         = app.OpenDefault
	_ func(*slog.Logger) app.Option                 = app.WithLogger
	_ func(func() time.Time) app.Option             = app.WithClock
	_ func(time.Duration) app.Option                = app.WithBusyTimeout
	_ func(time.Duration) app.Option                = app.WithRetryBudget
	_ func(string) app.Option                       = app.WithWorkDir
	_ func(string) (string, error)                  = app.ProjectRoot
	_ func() app.Actor                              = app.ActorDesktop
	_ func() app.Actor                              = app.ActorCLI
	_ func(string) app.Actor                        = app.ActorMCP
)

// appMethods returns the exported method names of *app.App, sorted.
func appMethods() []string {
	typ := reflect.TypeOf((*app.App)(nil))
	names := make([]string, 0, typ.NumMethod())
	for i := 0; i < typ.NumMethod(); i++ {
		names = append(names, typ.Method(i).Name)
	}
	sort.Strings(names)
	return names
}

// TestContractCoversEveryMethod fails when *app.App gains an exported method
// that engineContract (and so docs/api.md review) does not list.
func TestContractCoversEveryMethod(t *testing.T) {
	contract := reflect.TypeOf((*engineContract)(nil)).Elem()
	for _, name := range appMethods() {
		if _, ok := contract.MethodByName(name); !ok {
			t.Errorf("(*app.App).%s is not in engineContract; add it there and to docs/api.md", name)
		}
	}
}

var (
	jsonTagRE = regexp.MustCompile(`^[a-z][a-z0-9_]*(,omitempty)?$`)
	timeType  = reflect.TypeOf(time.Time{})
	ctxType   = reflect.TypeOf((*context.Context)(nil)).Elem()
	errType   = reflect.TypeOf((*error)(nil)).Elem()
)

// TestDTOJSONTags walks every type the contract methods accept or return,
// through struct fields, slices, arrays and pointers: every exported field
// has a snake_case json tag, and no field is a time.Time, *time.Time or map
// (D-18: timestamps are strings, shapes are fixed).
func TestDTOJSONTags(t *testing.T) {
	contract := reflect.TypeOf((*engineContract)(nil)).Elem()
	seen := map[reflect.Type]bool{}
	var walk func(where string, typ reflect.Type)
	walk = func(where string, typ reflect.Type) {
		switch typ.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			walk(where, typ.Elem())
			return
		case reflect.Map:
			t.Errorf("%s: map type %s (DTOs use structs and slices)", where, typ)
			return
		case reflect.Struct:
		default:
			return
		}
		if typ == timeType {
			t.Errorf("%s: time.Time (timestamps are RFC3339 strings)", where)
			return
		}
		if seen[typ] {
			return
		}
		seen[typ] = true
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if !f.IsExported() {
				continue
			}
			at := typ.Name() + "." + f.Name
			if tag := f.Tag.Get("json"); !jsonTagRE.MatchString(tag) {
				t.Errorf("%s: json tag %q is not snake_case (optionally ,omitempty)", at, tag)
			}
			walk(at, f.Type)
		}
	}
	for i := 0; i < contract.NumMethod(); i++ {
		m := contract.Method(i)
		for j := 0; j < m.Type.NumIn(); j++ {
			if in := m.Type.In(j); in != ctxType {
				walk(m.Name+" argument "+in.String(), in)
			}
		}
		for j := 0; j < m.Type.NumOut(); j++ {
			if out := m.Type.Out(j); out != errType {
				walk(m.Name+" result "+out.String(), out)
			}
		}
	}
	// ValidationError is returned through error, so walk it explicitly.
	walk("ValidationError", reflect.TypeOf(app.ValidationError{}))
	if len(seen) < 30 {
		t.Fatalf("walked only %d struct types; the contract walk is broken", len(seen))
	}
}

// TestNoJSONNulls checks that the DTOs every surface renders marshal with no
// null anywhere, on the seeded fixture (D-18).
func TestNoJSONNulls(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	ref := f.InvestigationRef
	check := func(name string, v any, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		t.Run(name, func(t *testing.T) { apptest.AssertNoJSONNulls(t, v) })
	}

	inv, err := a.GetInvestigation(ctx, ref)
	check("GetInvestigation", inv, err)
	detail, err := a.GetInvestigationDetail(ctx, ref, app.DetailOptions{IncludeRawDetail: true})
	check("GetInvestigationDetail", detail, err)
	list, err := a.ListInvestigations(ctx, app.ListOptions{})
	check("ListInvestigations", list, err)
	hits, err := a.Search(ctx, "debug", app.SearchOptions{})
	check("Search", hits, err)
	if len(hits) == 0 {
		t.Fatal("Search found nothing in the fixture")
	}
	none, err := a.Search(ctx, "zzzznothing", app.SearchOptions{})
	check("SearchEmpty", none, err)
	acts, err := a.ListActivity(ctx, app.ActivityFilter{})
	check("ListActivity", acts, err)
	pack, err := a.ContextPack(ctx, ref, app.ContextOptions{})
	check("ContextPack", pack, err)
	cp, err := a.Checkpoint(ctx, apptest.AI, ref, "Contract checkpoint")
	check("Checkpoint", cp, err)
	st, err := a.Status(ctx)
	check("Status", st, err)

	data, err := a.ExportInvestigation(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("ExportInvestigation", func(t *testing.T) {
		// RawMessage marshals as the document itself.
		apptest.AssertNoJSONNulls(t, jsonRaw(data))
	})
	imp, err := a.ImportInvestigation(ctx, apptest.Human, data)
	check("ImportInvestigation", imp, err)

	info, err := a.Backup(ctx, filepath.Join(t.TempDir(), "contract.db"))
	check("Backup", info, err)
	valid, err := a.ValidateBackup(ctx, info.Path)
	check("ValidateBackup", valid, err)

	// Empty investigation: every list must still be [] rather than null.
	empty, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Empty"})
	if err != nil {
		t.Fatal(err)
	}
	emptyDetail, err := a.GetInvestigationDetail(ctx, empty.Ref, app.DetailOptions{})
	check("GetInvestigationDetailEmpty", emptyDetail, err)
	emptyPack, err := a.ContextPack(ctx, empty.Ref, app.ContextOptions{})
	check("ContextPackEmpty", emptyPack, err)
	emptyEvidence, err := a.ListEvidence(ctx, empty.Ref)
	check("ListEvidenceEmpty", emptyEvidence, err)
}

// jsonRaw lets AssertNoJSONNulls inspect already encoded JSON.
type jsonRaw []byte

func (r jsonRaw) MarshalJSON() ([]byte, error) { return r, nil }

// moduleRoot walks up from the package directory to the directory holding
// go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the package directory")
		}
		dir = parent
	}
}

// TestAPIDocCoversEveryMethod requires docs/api.md to mention every exported
// method of *app.App, and the constructors and options, as "Name(" (D-30).
func TestAPIDocCoversEveryMethod(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(moduleRoot(t), "docs", "api.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	names := append(appMethods(), "Open", "OpenDefault", "WithLogger", "WithClock", "WithBusyTimeout", "WithRetryBudget",
		"ActorDesktop", "ActorCLI", "ActorMCP", "IsValidation")
	for _, name := range names {
		if !strings.Contains(doc, name+"(") {
			t.Errorf("docs/api.md does not document %s(", name)
		}
	}
	if strings.Contains(doc, "\xe2\x80\x94") {
		t.Error("docs/api.md contains an em dash")
	}
}
