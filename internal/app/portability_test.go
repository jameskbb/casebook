package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// exportDoc exports invRef and decodes the result.
func exportDoc(t *testing.T, a *app.App, invRef string) (app.ExportDocument, []byte) {
	t.Helper()
	data, err := a.ExportInvestigation(context.Background(), invRef)
	if err != nil {
		t.Fatalf("ExportInvestigation(%q): %v", invRef, err)
	}
	var doc app.ExportDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("export is not valid JSON: %v", err)
	}
	return doc, data
}

// marshalDoc encodes doc for import.
func marshalDoc(t *testing.T, doc app.ExportDocument) []byte {
	t.Helper()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// cloneDoc deep-copies doc through JSON.
func cloneDoc(t *testing.T, doc app.ExportDocument) app.ExportDocument {
	t.Helper()
	var out app.ExportDocument
	if err := json.Unmarshal(marshalDoc(t, doc), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// findKey reports the JSON path of the first object key named key.
func findKey(v any, key, path string) string {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if k == key {
				return path + "." + k
			}
			if p := findKey(child, key, path+"."+k); p != "" {
				return p
			}
		}
	case []any:
		for _, child := range x {
			if p := findKey(child, key, path+"[]"); p != "" {
				return p
			}
		}
	}
	return ""
}

func TestExportFixtureShape(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	before, err := a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}

	doc, data := exportDoc(t, a, "")
	if !bytes.HasSuffix(data, []byte("}\n")) {
		t.Fatal("export does not end with a newline")
	}
	if !bytes.Contains(data, []byte("\n  \"format\": \"casebook.investigation\"")) {
		t.Fatalf("export is not indented JSON with the format first:\n%.200s", data)
	}
	var generic map[string]any
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"format", "version", "exported_at", "investigation", "evidence", "claims",
		"hypotheses", "questions", "claim_links", "hypothesis_links", "activity"} {
		if _, ok := generic[k]; !ok {
			t.Errorf("export lacks %q", k)
		}
	}
	for _, k := range []string{"evidence", "claims", "hypotheses", "questions", "claim_links", "hypothesis_links", "activity"} {
		if _, ok := generic[k].([]any); !ok {
			t.Errorf("%q is %T, want an array", k, generic[k])
		}
	}
	for _, k := range []string{"id", "investigation_id", "superseded_by_id", "claim_id", "evidence_id"} {
		if p := findKey(generic, k, "$"); p != "" {
			t.Errorf("export exposes internal id key at %s", p)
		}
	}

	if doc.Format != app.ExportFormat || doc.Version != app.ExportVersion || doc.ExportedAt != app.FormatTime(apptest.FixedTime) {
		t.Fatalf("header = %q %d %q", doc.Format, doc.Version, doc.ExportedAt)
	}
	if doc.Investigation.Ref != f.InvestigationRef || doc.Investigation.Title != "September cloud bill up 38%" ||
		doc.Investigation.Conclusion == "" || doc.Investigation.CreatedBy != apptest.Human {
		t.Fatalf("investigation = %+v", doc.Investigation)
	}
	if got := len(doc.Evidence); got != 4 {
		t.Fatalf("evidence = %d, want 4", got)
	}
	if doc.Evidence[0].Ref != "E-001" || !strings.Contains(doc.Evidence[0].RawDetail, "+  level: debug") {
		t.Fatalf("E-001 raw detail missing: %+v", doc.Evidence[0])
	}
	var meta map[string]any
	if err := json.Unmarshal(doc.Evidence[2].Metadata, &meta); err != nil || meta["affected_records"] != float64(214) {
		t.Fatalf("E-003 metadata = %s (%v)", doc.Evidence[2].Metadata, err)
	}
	if len(doc.Claims) != 3 || doc.Claims[2].Ref != "C-003" || doc.Claims[2].SupersededBy != "C-002" ||
		doc.Claims[2].Status != app.ClaimStatusSuperseded {
		t.Fatalf("claims = %+v", doc.Claims)
	}
	if len(doc.Hypotheses) != 2 || doc.Hypotheses[1].Status != app.HypothesisStatusDisproven {
		t.Fatalf("hypotheses = %+v", doc.Hypotheses)
	}
	if len(doc.Questions) != 1 || doc.Questions[0].Ref != "Q-001" {
		t.Fatalf("questions = %+v", doc.Questions)
	}
	if len(doc.ClaimLinks) != 5 || doc.ClaimLinks[0].ClaimRef != "C-001" || doc.ClaimLinks[0].EvidenceRef != "E-001" {
		t.Fatalf("claim links = %+v", doc.ClaimLinks)
	}
	if len(doc.HypothesisLinks) != 1 || doc.HypothesisLinks[0].HypothesisRef != "H-002" ||
		doc.HypothesisLinks[0].Relation != app.RelationContradicts {
		t.Fatalf("hypothesis links = %+v", doc.HypothesisLinks)
	}
	acts, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: f.InvestigationRef, Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	// Every activity row but the machine-local active.* ones (ADR-0016).
	shared := 0
	for _, r := range acts {
		if !strings.HasPrefix(r.Action, "active.") {
			shared++
		}
	}
	if shared == len(acts) {
		t.Fatal("fixture has no active.* row to leave out")
	}
	if len(doc.Activity) != shared || doc.Activity[0].Action != "investigation.create" {
		t.Fatalf("activity = %d rows (first %+v), want %d", len(doc.Activity), doc.Activity[0], shared)
	}
	for _, r := range doc.Activity {
		if strings.HasPrefix(r.Action, "active.") {
			t.Fatalf("export holds %s", r.Action)
		}
	}

	after, err := a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.Counts != before.Counts {
		t.Fatalf("export changed counts: %+v -> %+v", before.Counts, after.Counts)
	}
}

func TestExportNoActive(t *testing.T) {
	a := apptest.Open(t)
	if _, err := a.ExportInvestigation(context.Background(), ""); err == nil {
		t.Fatal("export with no active investigation succeeded")
	}
	if _, err := a.ExportInvestigation(context.Background(), "INV-0042"); err == nil {
		t.Fatal("export of a missing investigation succeeded")
	}
}

func TestImportRoundTripFreshApp(t *testing.T) {
	ctx := context.Background()
	src := apptest.Open(t)
	apptest.SeedCloudSpend(t, src)
	doc, data := exportDoc(t, src, "")

	dst := apptest.Open(t)
	res, err := dst.ImportInvestigation(ctx, apptest.Human, data)
	if err != nil {
		t.Fatalf("ImportInvestigation: %v", err)
	}
	want := app.ImportResult{
		InvestigationRef: "INV-0001", Title: "September cloud bill up 38%", OriginalRef: "INV-0001", RefChanged: false,
		Counts: app.Counts{Investigations: 1, Evidence: 4, Claims: 3, Hypotheses: 2, Questions: 1,
			ClaimLinks: 5, HypothesisLinks: 1, Activity: len(doc.Activity) + 1},
	}
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("result = %+v, want %+v", res, want)
	}
	st, err := dst.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Counts != want.Counts {
		t.Fatalf("db counts = %+v, want %+v", st.Counts, want.Counts)
	}
	if active, err := dst.GetActiveInvestigation(ctx); err != nil || active != nil {
		t.Fatalf("import changed the active investigation: %v, %v", active, err)
	}

	again, _ := exportDoc(t, dst, "INV-0001")
	n := len(again.Activity)
	if n != len(doc.Activity)+1 {
		t.Fatalf("re-export activity = %d, want %d", n, len(doc.Activity)+1)
	}
	last := again.Activity[n-1]
	if last.Action != "import" || last.Actor != (app.Actor{Kind: app.ActorImport, Client: "casebook-ui"}) ||
		last.TargetRef != "INV-0001" ||
		last.Summary != "Imported INV-0001 (4 evidence, 3 claims, 2 hypotheses, 1 questions); 3 verifications reset to ai_generated" {
		t.Fatalf("import row = %+v", last)
	}
	again.Activity = again.Activity[:n-1]
	again.ExportedAt = doc.ExportedAt
	doc.Investigation.ConclusionVerification = app.VerificationAIGenerated
	// AR-09: verification a file asserts is not trusted; everything else
	// round trips.
	for i := range doc.Evidence {
		if v := doc.Evidence[i].Verification; v == app.VerificationHumanVerified || v == app.VerificationSourceVerified {
			doc.Evidence[i].Verification = app.VerificationAIGenerated
		}
	}
	for i := range doc.Claims {
		if v := doc.Claims[i].Verification; v == app.VerificationHumanVerified || v == app.VerificationSourceVerified {
			doc.Claims[i].Verification = app.VerificationAIGenerated
		}
	}
	for i := range doc.Hypotheses {
		if v := doc.Hypotheses[i].Verification; v == app.VerificationHumanVerified || v == app.VerificationSourceVerified {
			doc.Hypotheses[i].Verification = app.VerificationAIGenerated
		}
	}
	if !reflect.DeepEqual(again, doc) {
		a, _ := json.MarshalIndent(again, "", " ")
		b, _ := json.MarshalIndent(doc, "", " ")
		t.Fatalf("re-export differs from the original:\n%s\n----\n%s", a, b)
	}

	// The INV counter moved past the imported ref.
	next, err := dst.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Next"})
	if err != nil || next.Ref != "INV-0002" {
		t.Fatalf("next investigation = %q, %v; want INV-0002", next.Ref, err)
	}
	// Record ref counters continue after the imported refs.
	ev, err := dst.AddEvidence(ctx, apptest.Human, "INV-0001", app.EvidenceInput{
		Title: "After import", Summary: "s", SourceType: app.SourceTypeFile})
	if err != nil || ev.Ref != "E-005" {
		t.Fatalf("evidence after import = %q, %v; want E-005", ev.Ref, err)
	}
}

func TestImportRefCollision(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	_, data := exportDoc(t, a, "")

	res, err := a.ImportInvestigation(ctx, apptest.AI, data)
	if err != nil {
		t.Fatalf("ImportInvestigation: %v", err)
	}
	if res.InvestigationRef != "INV-0002" || res.OriginalRef != "INV-0001" || !res.RefChanged {
		t.Fatalf("result = %+v", res)
	}
	evs, err := a.ListEvidence(ctx, "INV-0002")
	if err != nil {
		t.Fatal(err)
	}
	var refs []string
	for _, e := range evs {
		refs = append(refs, e.Ref)
	}
	if strings.Join(refs, ",") != "E-001,E-002,E-003,E-004" {
		t.Fatalf("imported evidence refs = %v", refs)
	}
	raw, err := a.GetEvidence(ctx, "INV-0002", "E-001", app.GetEvidenceOptions{IncludeRawDetail: true})
	if err != nil || !strings.Contains(raw.RawDetail, "+  level: debug") {
		t.Fatalf("imported raw detail = %q, %v", raw.RawDetail, err)
	}
	c3, err := a.GetClaim(ctx, "INV-0002", "C-003")
	if err != nil || c3.SupersededBy != "C-002" || c3.Status != app.ClaimStatusSuperseded {
		t.Fatalf("C-003 = %+v, %v", c3, err)
	}
	c1, err := a.GetClaim(ctx, "INV-0002", "C-001")
	if err != nil || len(c1.Links) != 2 || c1.Links[0].EvidenceRef != "E-001" || c1.Links[1].EvidenceRef != "E-004" {
		t.Fatalf("C-001 links = %+v, %v", c1.Links, err)
	}
	h2, err := a.GetHypothesis(ctx, "INV-0002", "H-002")
	if err != nil || len(h2.Links) != 1 || h2.Links[0].Relation != app.RelationContradicts {
		t.Fatalf("H-002 = %+v, %v", h2, err)
	}
	hits, err := a.Search(ctx, "debug", app.SearchOptions{InvestigationRef: "INV-0002"})
	if err != nil || len(hits) == 0 {
		t.Fatalf("search in the imported investigation = %v, %v", hits, err)
	}
	for _, h := range hits {
		if h.InvestigationRef != "INV-0002" {
			t.Fatalf("scoped hit from %s", h.InvestigationRef)
		}
	}
	acts, err := a.ListActivity(ctx, app.ActivityFilter{Limit: 1})
	if err != nil || len(acts) != 1 {
		t.Fatalf("ListActivity: %v, %v", acts, err)
	}
	row := acts[0]
	if row.Action != "import" || row.InvestigationRef != "INV-0002" || row.TargetRef != "INV-0002" ||
		row.Actor != (app.Actor{Kind: app.ActorImport, Client: "claude-code"}) ||
		row.Summary != "Imported INV-0001 as INV-0002 (4 evidence, 3 claims, 2 hypotheses, 1 questions); 3 verifications reset to ai_generated" || row.Imported {
		t.Fatalf("import row = %+v", row)
	}
	active, err := a.GetActiveInvestigation(ctx)
	if err != nil || active == nil || active.Ref != f.InvestigationRef {
		t.Fatalf("active = %v, %v; want %s", active, err, f.InvestigationRef)
	}
	next, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Next"})
	if err != nil || next.Ref != "INV-0003" {
		t.Fatalf("next investigation = %q, %v; want INV-0003", next.Ref, err)
	}
}

func TestImportHostileInputs(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)
	base, baseData := exportDoc(t, a, "")
	before, err := a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}

	mutate := func(fn func(d *app.ExportDocument)) func(t *testing.T) []byte {
		return func(t *testing.T) []byte {
			d := cloneDoc(t, base)
			fn(&d)
			return marshalDoc(t, d)
		}
	}
	rawBytes := func(b []byte) func(t *testing.T) []byte {
		return func(*testing.T) []byte { return b }
	}
	oversize := make([]byte, app.MaxImportBytes+1)
	copy(oversize, baseData)
	for i := len(baseData); i < len(oversize); i++ {
		oversize[i] = ' '
	}

	cases := []struct {
		name  string
		data  func(t *testing.T) []byte
		field string
	}{
		{"20 MiB plus one byte", rawBytes(oversize), "data"},
		{"wrong format", mutate(func(d *app.ExportDocument) { d.Format = "casebook.other" }), "format"},
		{"missing format", mutate(func(d *app.ExportDocument) { d.Format = "" }), "format"},
		{"version 3", mutate(func(d *app.ExportDocument) { d.Version = 3 }), "version"},
		{"version 0", mutate(func(d *app.ExportDocument) { d.Version = 0 }), "version"},
		{"unknown top-level field", rawBytes(append([]byte(`{"secretfieldname":1,`), baseData[1:]...)), "data"},
		{"unknown nested field", rawBytes(bytes.Replace(baseData, []byte(`"title":`), []byte(`"secretfieldname":1,"title":`), 1)), "data"},
		{"trailing garbage", rawBytes(append(append([]byte{}, baseData...), []byte("garbage")...)), "data"},
		{"second document", rawBytes(append(append([]byte{}, baseData...), baseData...)), "data"},
		{"not JSON", rawBytes([]byte("secret not json")), "data"},
		{"truncated", rawBytes(baseData[:len(baseData)/2]), "data"},
		{"wrong type", rawBytes(bytes.Replace(baseData, []byte(`"evidence": [`), []byte(`"evidence": 7, "x": [`), 1)), "data"},
		{"invalid enum", mutate(func(d *app.ExportDocument) { d.Claims[0].Status = "maybe" }), "claims[0].status"},
		{"invalid source type", mutate(func(d *app.ExportDocument) { d.Evidence[1].SourceType = "rumor" }), "evidence[1].source_type"},
		{"oversized raw_detail", mutate(func(d *app.ExportDocument) {
			d.Evidence[0].RawDetail = strings.Repeat("x", app.MaxRawDetailBytes+1)
		}), "evidence[0].raw_detail"},
		{"oversized title", mutate(func(d *app.ExportDocument) {
			d.Investigation.Title = strings.Repeat("t", app.MaxTitleBytes+1)
		}), "investigation.title"},
		{"empty summary", mutate(func(d *app.ExportDocument) { d.Evidence[3].Summary = " \t" }), "evidence[3].summary"},
		{"metadata array", mutate(func(d *app.ExportDocument) { d.Evidence[2].Metadata = json.RawMessage(`[1,2]`) }), "evidence[2].metadata"},
		{"metadata string", mutate(func(d *app.ExportDocument) { d.Evidence[2].Metadata = json.RawMessage(`"{}"`) }), "evidence[2].metadata"},
		{"dangling link ref", mutate(func(d *app.ExportDocument) { d.ClaimLinks[0].EvidenceRef = "E-099" }), "claim_links[0].evidence_ref"},
		{"dangling link claim", mutate(func(d *app.ExportDocument) { d.ClaimLinks[1].ClaimRef = "C-042" }), "claim_links[1].claim_ref"},
		{"duplicate link", mutate(func(d *app.ExportDocument) { d.ClaimLinks = append(d.ClaimLinks, d.ClaimLinks[0]) }), "claim_links[5].evidence_ref"},
		{"duplicate evidence ref", mutate(func(d *app.ExportDocument) { d.Evidence[1].Ref = "E-001" }), "evidence[1].ref"},
		{"claim ref with E- prefix", mutate(func(d *app.ExportDocument) { d.Claims[0].Ref = "E-001" }), "claims[0].ref"},
		{"qualified record ref", mutate(func(d *app.ExportDocument) { d.Questions[0].Ref = "INV-0001/Q-001" }), "questions[0].ref"},
		{"bad investigation ref", mutate(func(d *app.ExportDocument) { d.Investigation.Ref = "C-001" }), "investigation.ref"},
		{"investigation ref past the import cap", mutate(func(d *app.ExportDocument) { d.Investigation.Ref = "INV-999999999999" }), "investigation.ref"},
		{"evidence ref past the import cap", mutate(func(d *app.ExportDocument) { d.Evidence[0].Ref = "E-100000" }), "evidence[0].ref"},
		{"claim ref past the import cap", mutate(func(d *app.ExportDocument) { d.Claims[0].Ref = "C-999999999999" }), "claims[0].ref"},
		{"question ref past the import cap", mutate(func(d *app.ExportDocument) { d.Questions[0].Ref = "Q-100000" }), "questions[0].ref"},
		{"self-superseding claim", mutate(func(d *app.ExportDocument) { d.Claims[2].SupersededBy = "C-003" }), "claims[2].superseded_by"},
		{"dangling superseded_by", mutate(func(d *app.ExportDocument) { d.Claims[2].SupersededBy = "C-009" }), "claims[2].superseded_by"},
		{"superseded_by without superseded status", mutate(func(d *app.ExportDocument) { d.Claims[0].SupersededBy = "C-002" }), "claims[0].superseded_by"},
		{"supersede cycle", mutate(func(d *app.ExportDocument) {
			d.Claims[1].Status = app.ClaimStatusSuperseded
			d.Claims[1].SupersededBy = "C-003"
		}), "claims[1].superseded_by"},
		{"superseded claim without superseded_by", mutate(func(d *app.ExportDocument) { d.Claims[2].SupersededBy = "" }), "claims[2].superseded_by"},
		{"superseded hypothesis without superseded_by", mutate(func(d *app.ExportDocument) {
			d.Hypotheses[0].Status = app.HypothesisStatusSuperseded
		}), "hypotheses[0].superseded_by"},
		{"resolution on an open question", mutate(func(d *app.ExportDocument) { d.Questions[0].Resolution = "yes" }), "questions[0].resolution"},
		{"resolution on a deferred question", mutate(func(d *app.ExportDocument) {
			d.Questions[0].Status = app.QuestionStatusDeferred
			d.Questions[0].Resolution = "yes"
		}), "questions[0].resolution"},
		{"defer note on an open question", mutate(func(d *app.ExportDocument) { d.Questions[0].DeferNote = "later" }), "questions[0].defer_note"},
		{"defer note on a resolved question", mutate(func(d *app.ExportDocument) {
			d.Questions[0].Status = app.QuestionStatusResolved
			d.Questions[0].Resolution = "yes"
			d.Questions[0].ResolvedAt = "2026-01-15T09:00:00.000Z"
			d.Questions[0].DeferNote = "later"
		}), "questions[0].defer_note"},
		{"invalid timestamp", mutate(func(d *app.ExportDocument) { d.Evidence[0].CreatedAt = "yesterday" }), "evidence[0].created_at"},
		{"missing captured_at", mutate(func(d *app.ExportDocument) { d.Evidence[0].CapturedAt = "" }), "evidence[0].captured_at"},
		{"unknown actor kind", mutate(func(d *app.ExportDocument) { d.Claims[1].CreatedBy.Kind = "robot" }), "claims[1].created_by.kind"},
		{"empty actor client", mutate(func(d *app.ExportDocument) { d.HypothesisLinks[0].CreatedBy.Client = "" }), "hypothesis_links[0].created_by.client"},
		{"archived without archived_at", mutate(func(d *app.ExportDocument) { d.Investigation.Status = app.InvestigationStatusArchived }), "investigation.archived_at"},
		{"resolved question without resolved_at", mutate(func(d *app.ExportDocument) {
			d.Questions[0].Status = app.QuestionStatusResolved
			d.Questions[0].Resolution = "yes"
		}), "questions[0].resolved_at"},
		{"bad activity action", mutate(func(d *app.ExportDocument) { d.Activity[0].Action = "DROP TABLE" }), "activity[0].action"},
		{"long activity summary", mutate(func(d *app.ExportDocument) {
			d.Activity[1].Summary = strings.Repeat("s", app.MaxActivitySummaryBytes+1)
		}), "activity[1].summary"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := a.ImportInvestigation(ctx, apptest.Human, tc.data(t))
			wantValidation(t, err, tc.field)
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error echoes input text: %v", err)
			}
			st, serr := a.Status(ctx)
			if serr != nil {
				t.Fatal(serr)
			}
			if st.Counts != before.Counts {
				t.Fatalf("rejected import changed counts: %+v -> %+v", before.Counts, st.Counts)
			}
		})
	}

	_, err = a.ImportInvestigation(ctx, apptest.Human, mutate(func(d *app.ExportDocument) { d.Version = 3 })(t))
	if ve, _ := app.IsValidation(err); ve == nil || ve.Message != "unsupported export version 3; this Casebook reads versions 1 and 2" {
		t.Fatalf("version message = %v", err)
	}
	_, err = a.ImportInvestigation(ctx, app.Actor{Kind: "robot", Client: "x"}, baseData)
	wantValidation(t, err, "actor.kind")

	// No rejected import consumed an INV ref.
	next, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Next"})
	if err != nil || next.Ref != "INV-0002" {
		t.Fatalf("next investigation = %q, %v; want INV-0002", next.Ref, err)
	}
}

func TestImportSanitizesControlBytes(t *testing.T) {
	ctx := context.Background()
	src := apptest.Open(t)
	apptest.SeedCloudSpend(t, src)
	doc, _ := exportDoc(t, src, "")
	doc.Investigation.Title = "  Bad\x00Ti\x07tle\x1b\x7f  "
	doc.Evidence[0].Summary = "line one\r\nline two\x00"
	doc.Evidence[2].Metadata = json.RawMessage("{\n  \"affected_records\": 214\n}")

	dst := apptest.Open(t)
	if _, err := dst.ImportInvestigation(ctx, apptest.Human, marshalDoc(t, doc)); err != nil {
		t.Fatalf("ImportInvestigation: %v", err)
	}
	inv, err := dst.GetInvestigation(ctx, "INV-0001")
	if err != nil || inv.Title != "BadTitle" {
		t.Fatalf("title = %q, %v; want BadTitle", inv.Title, err)
	}
	ev, err := dst.GetEvidence(ctx, "INV-0001", "E-001", app.GetEvidenceOptions{})
	if err != nil || ev.Summary != "line one\nline two" {
		t.Fatalf("summary = %q, %v", ev.Summary, err)
	}
	e3, err := dst.GetEvidence(ctx, "INV-0001", "E-003", app.GetEvidenceOptions{})
	if err != nil || e3.Metadata != `{"affected_records":214}` {
		t.Fatalf("metadata = %q, %v", e3.Metadata, err)
	}
}

func TestImportEmptyInvestigation(t *testing.T) {
	ctx := context.Background()
	src := apptest.Open(t)
	inv, err := src.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Empty"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.ArchiveInvestigation(ctx, apptest.Human, inv.Ref); err != nil {
		t.Fatal(err)
	}
	doc, data := exportDoc(t, src, inv.Ref)
	if doc.Investigation.Status != app.InvestigationStatusArchived || doc.Investigation.PreArchiveStatus != app.InvestigationStatusNew {
		t.Fatalf("archived export = %+v", doc.Investigation)
	}
	if !bytes.Contains(data, []byte(`"evidence": []`)) {
		t.Fatal("empty evidence is not an empty array")
	}
	dst := apptest.Open(t)
	if _, err := dst.ImportInvestigation(ctx, app.ActorCLI(), data); err != nil {
		t.Fatalf("ImportInvestigation: %v", err)
	}
	got, err := dst.GetInvestigation(ctx, inv.Ref)
	if err != nil || !got.Archived {
		t.Fatalf("imported = %+v, %v", got, err)
	}
	back, err := dst.UnarchiveInvestigation(ctx, apptest.Human, inv.Ref)
	if err != nil || back.Status != app.InvestigationStatusNew {
		t.Fatalf("unarchive imported = %+v, %v", back, err)
	}
}

// TestImportLargestRefsLeaveHeadroom proves the import caps keep every later
// ref parseable: after importing the largest accepted refs, new
// investigations and records still get refs the API can read back.
func TestImportLargestRefsLeaveHeadroom(t *testing.T) {
	ctx := context.Background()
	src := apptest.Open(t)
	apptest.SeedCloudSpend(t, src)
	doc, _ := exportDoc(t, src, "")
	maxInv := app.FormatInvestigationRef(app.MaxImportInvestigationRefNum)
	maxEv := "E-099999"
	doc.Investigation.Ref = maxInv
	for i := range doc.ClaimLinks {
		if doc.ClaimLinks[i].EvidenceRef == doc.Evidence[0].Ref {
			doc.ClaimLinks[i].EvidenceRef = maxEv
		}
	}
	for i := range doc.HypothesisLinks {
		if doc.HypothesisLinks[i].EvidenceRef == doc.Evidence[0].Ref {
			doc.HypothesisLinks[i].EvidenceRef = maxEv
		}
	}
	doc.Evidence[0].Ref = maxEv

	a := apptest.Open(t)
	res, err := a.ImportInvestigation(ctx, apptest.Human, marshalDoc(t, doc))
	if err != nil || res.InvestigationRef != maxInv {
		t.Fatalf("ImportInvestigation = %+v, %v", res, err)
	}
	next, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "After import"})
	if err != nil {
		t.Fatal(err)
	}
	if next.Ref != "INV-1000000" {
		t.Fatalf("next ref = %q, want INV-1000000", next.Ref)
	}
	if got, err := a.GetInvestigation(ctx, next.Ref); err != nil || got.Ref != next.Ref {
		t.Fatalf("GetInvestigation(%q) = %+v, %v", next.Ref, got, err)
	}
	ev, err := a.AddEvidence(ctx, apptest.Human, maxInv, app.EvidenceInput{
		Title: "After", Summary: "s", SourceType: app.SourceTypeFile,
	})
	if err != nil || ev.Ref != "E-100000" {
		t.Fatalf("AddEvidence = %q, %v; want E-100000", ev.Ref, err)
	}
	if _, err := a.GetEvidence(ctx, maxInv, ev.Ref, app.GetEvidenceOptions{}); err != nil {
		t.Fatalf("GetEvidence(%q): %v", ev.Ref, err)
	}
}

// TestImportMarksProvenance reproduces AR-09: a hand-edited file claiming
// human verification and a forged human activity row imports, but every
// row is flagged imported and the verification it asserts is not trusted.
func TestImportMarksProvenance(t *testing.T) {
	ctx := context.Background()
	src := apptest.Open(t)
	apptest.SeedCloudSpend(t, src)
	doc, _ := exportDoc(t, src, "")
	doc.Evidence[0].CreatedBy = app.Actor{Kind: app.ActorHuman, Client: "casebook-ui"}
	doc.Evidence[0].Verification = app.VerificationHumanVerified
	doc.Claims[1].Verification = app.VerificationSourceVerified
	doc.Activity = append(doc.Activity, app.ExportActivity{
		Actor: app.Actor{Kind: app.ActorHuman, Client: "james"}, Action: "evidence.add", TargetRef: "E-099",
		Summary: "Forged: the CEO approved X", CreatedAt: "2020-01-01T00:00:00.000Z",
	})

	dst := apptest.Open(t)
	if _, err := dst.ImportInvestigation(ctx, apptest.Human, marshalDoc(t, doc)); err != nil {
		t.Fatal(err)
	}
	detail, err := dst.GetInvestigationDetail(ctx, "INV-0001", app.DetailOptions{ActivityLimit: 200})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range detail.Evidence {
		if !e.Imported || e.Verification == app.VerificationHumanVerified || e.Verification == app.VerificationSourceVerified {
			t.Fatalf("evidence %s imported=%v verification=%s", e.Ref, e.Imported, e.Verification)
		}
	}
	for _, c := range detail.Claims {
		if !c.Imported || c.Verification == app.VerificationHumanVerified || c.Verification == app.VerificationSourceVerified {
			t.Fatalf("claim %s imported=%v verification=%s", c.Ref, c.Imported, c.Verification)
		}
	}
	for _, h := range detail.Hypotheses {
		if !h.Imported {
			t.Fatalf("hypothesis %s not flagged imported", h.Ref)
		}
	}
	for _, q := range detail.Questions {
		if !q.Imported {
			t.Fatalf("question %s not flagged imported", q.Ref)
		}
	}
	var forged, importRow bool
	for _, a := range detail.RecentActivity {
		switch a.Action {
		case "import":
			importRow = !a.Imported
		case "evidence.add":
			if a.TargetRef == "E-099" {
				forged = a.Imported && a.Actor.Client == "james"
			}
		default:
			if !a.Imported {
				t.Fatalf("historical row %+v not flagged imported", a)
			}
		}
	}
	if !forged || !importRow {
		t.Fatalf("forged row flagged=%v, import row local=%v", forged, importRow)
	}
	// The context pack attributes imported changes.
	md, err := dst.RenderContext(ctx, "INV-0001", "markdown", app.ContextOptions{RecentChanges: 50})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "human james (imported): Forged: the CEO approved X") {
		t.Fatalf("pack does not attribute the imported row:\n%s", md)
	}
	// Verifying locally restores trust; the row stays marked imported.
	c, err := dst.SetClaimVerification(ctx, apptest.Human, "INV-0001", "C-001", app.VerificationHumanVerified)
	if err != nil || c.Verification != app.VerificationHumanVerified || !c.Imported {
		t.Fatalf("verified imported claim = %+v, %v", c, err)
	}
}
