package mcpserver

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func evidenceArgs(extra map[string]any) map[string]any {
	args := map[string]any{
		"title":       "Debug volume by service",
		"summary":     "214 services log at debug level since August 28.",
		"source_type": "sql_query",
	}
	for k, v := range extra {
		if v == nil {
			delete(args, k)
			continue
		}
		args[k] = v
	}
	return args
}

func TestAddEvidence(t *testing.T) {
	a, f := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_add_evidence")
	ctx := context.Background()

	t.Run("happy path with metadata", func(t *testing.T) {
		res := callTool(t, cs, "casebook_add_evidence", evidenceArgs(map[string]any{
			"source_label": "Comparison query",
			"raw_detail":   "service,level,gb_per_day\ncheckout,debug,412",
			"metadata":     map[string]any{"affected_records": 214},
		}))
		out := decode[mutationResult[evidenceRecord]](t, res)
		if out.Ref != "E-005" || out.InvestigationRef != f.InvestigationRef || out.Action != "created" {
			t.Fatalf("out = %+v", out)
		}
		if !out.Record.HasRawDetail || out.Record.SourceType != app.SourceTypeSQLQuery ||
			out.Record.Verification != app.VerificationAIGenerated {
			t.Errorf("record = %+v", out.Record)
		}
		if txt := resultText(res); txt != "Added evidence E-005 to INV-0001: Debug volume by service." {
			t.Errorf("text = %q", txt)
		}
		if strings.Contains(resultText(res), "gb_per_day") {
			t.Error("raw detail echoed back")
		}
		ev, err := a.GetEvidence(ctx, "", "E-005", app.GetEvidenceOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if ev.Metadata != `{"affected_records":214}` {
			t.Errorf("metadata = %q", ev.Metadata)
		}
		if ev.CreatedBy != apptest.AI {
			t.Errorf("created_by = %+v", ev.CreatedBy)
		}
	})

	t.Run("explicit qualified investigation ref", func(t *testing.T) {
		res := callTool(t, cs, "casebook_add_evidence", evidenceArgs(map[string]any{"investigation_ref": "inv-1"}))
		out := decode[mutationResult[evidenceRecord]](t, res)
		if out.InvestigationRef != "INV-0001" {
			t.Errorf("investigation_ref = %q", out.InvestigationRef)
		}
	})

	errorCases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"bad enum", evidenceArgs(map[string]any{"source_type": "bogus"}), "source_type"},
		{"missing title", evidenceArgs(map[string]any{"title": nil}), "title"},
		{"unknown property", evidenceArgs(map[string]any{"colour": "red"}), "colour"},
		{"oversized title", evidenceArgs(map[string]any{"title": strings.Repeat("t", 201)}), "invalid title: "},
		{"unknown investigation", evidenceArgs(map[string]any{"investigation_ref": "INV-0099"}), "not found: "},
		{"bad captured_at", evidenceArgs(map[string]any{"captured_at": "yesterday"}), "invalid captured_at: "},
	}
	for _, c := range errorCases {
		t.Run(c.name, func(t *testing.T) {
			wantToolError(t, callTool(t, cs, "casebook_add_evidence", c.args), c.want)
			// The session keeps serving after a tool error.
			res := callTool(t, cs, "casebook_get_active_investigation", map[string]any{})
			if res.IsError {
				t.Fatalf("next call failed: %s", resultText(res))
			}
		})
	}
}

func TestAddEvidenceNoActive(t *testing.T) {
	a := apptest.Open(t)
	cs := newTestSession(t, a, sessionOpts{})
	res := callTool(t, cs, "casebook_add_evidence", evidenceArgs(nil))
	wantToolError(t, res, "casebook_set_active_investigation")
	wantToolError(t, res, "investigation_ref")
}

func TestGetEvidence(t *testing.T) {
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_get_evidence")
	ctx := context.Background()
	stored, err := a.GetEvidence(ctx, "", "E-001", app.GetEvidenceOptions{IncludeRawDetail: true})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("default has no raw detail", func(t *testing.T) {
		res := callTool(t, cs, "casebook_get_evidence", map[string]any{"evidence_ref": "e-1"})
		v := decode[evidenceView](t, res)
		if v.Ref != "E-001" || v.InvestigationRef != "INV-0001" || v.Title != stored.Title || v.Summary != stored.Summary ||
			v.SourceType != app.SourceTypeCode || v.SourceRef != stored.SourceRef || v.SourceLabel != stored.SourceLabel ||
			v.Verification != stored.Verification || v.CreatedBy != stored.CreatedBy || !v.HasRawDetail {
			t.Fatalf("view = %+v", v)
		}
		if !reflect.DeepEqual(v.ClaimLinks, []app.RecordLink{{Ref: "C-001", Relation: app.RelationSupports}}) ||
			v.HypothesisLinks == nil {
			t.Errorf("links = %+v / %+v", v.ClaimLinks, v.HypothesisLinks)
		}
		keys := structuredKeys(t, res.StructuredContent)
		for _, k := range []string{"raw_detail", "raw_detail_bytes", "raw_detail_offset", "raw_detail_truncated", "raw_detail_next_offset"} {
			if _, ok := keys[k]; ok {
				t.Errorf("key %s present without include_raw_detail", k)
			}
		}
		txt := resultText(res)
		if strings.Contains(txt, stored.RawDetail[:30]) {
			t.Error("text contains raw detail")
		}
		for _, want := range []string{"E-001 in INV-0001: " + stored.Title, stored.Summary, "C-001 supports", "include_raw_detail"} {
			if !strings.Contains(txt, want) {
				t.Errorf("text lacks %q:\n%s", want, txt)
			}
		}
	})

	t.Run("include raw detail", func(t *testing.T) {
		res := callTool(t, cs, "casebook_get_evidence", map[string]any{"evidence_ref": "E-001", "include_raw_detail": true})
		v := decode[evidenceView](t, res)
		if v.RawDetail != stored.RawDetail || v.RawDetailBytes == nil || *v.RawDetailBytes != len(stored.RawDetail) ||
			v.RawDetailTruncated == nil || *v.RawDetailTruncated || v.RawDetailOffset == nil || *v.RawDetailOffset != 0 ||
			v.RawDetailNextOffset != 0 {
			t.Fatalf("view = %+v", v)
		}
		if !strings.Contains(resultText(res), stored.RawDetail) {
			t.Error("text lacks the raw detail")
		}
	})

	t.Run("qualified ref", func(t *testing.T) {
		v := decode[evidenceView](t, callTool(t, cs, "casebook_get_evidence", map[string]any{"evidence_ref": "INV-0001/E-003"}))
		if v.Ref != "E-003" || v.Metadata != `{"affected_records":214}` {
			t.Fatalf("view = %+v", v)
		}
	})

	errorCases := []struct {
		name   string
		args   map[string]any
		prefix string
	}{
		{"max too big", map[string]any{"evidence_ref": "E-001", "include_raw_detail": true, "max_raw_bytes": 65537}, "invalid max_raw_bytes"},
		{"max negative", map[string]any{"evidence_ref": "E-001", "include_raw_detail": true, "max_raw_bytes": -1}, "invalid max_raw_bytes"},
		{"offset negative", map[string]any{"evidence_ref": "E-001", "include_raw_detail": true, "raw_detail_offset": -1}, "invalid raw_detail_offset"},
		{"offset beyond", map[string]any{"evidence_ref": "E-001", "include_raw_detail": true, "raw_detail_offset": len(stored.RawDetail) + 1}, "invalid raw_detail_offset"},
		{"wrong ref kind", map[string]any{"evidence_ref": "C-001"}, "invalid ref"},
		{"unknown evidence", map[string]any{"evidence_ref": "E-099"}, "not found:"},
	}
	for _, c := range errorCases {
		t.Run(c.name, func(t *testing.T) {
			wantErrorPrefix(t, callTool(t, cs, "casebook_get_evidence", c.args), c.prefix)
		})
	}
	t.Run("missing evidence_ref", func(t *testing.T) {
		wantToolError(t, callTool(t, cs, "casebook_get_evidence", map[string]any{}), "evidence_ref")
	})
}

func TestGetEvidencePagesRawDetail(t *testing.T) {
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	// 21845 three-byte runes plus one byte: 65536 bytes, so a 16384-byte
	// window ends mid-rune and must move back.
	raw := strings.Repeat("日本語", 21845/3) + strings.Repeat("日", 21845%3) + "a"
	if len(raw) != app.MaxRawDetailBytes {
		t.Fatalf("raw is %d bytes", len(raw))
	}
	ev, err := a.AddEvidence(context.Background(), apptest.AI, "", app.EvidenceInput{
		Title: "Long log", Summary: "A long multibyte log.", SourceType: app.SourceTypeFile, RawDetail: raw,
	})
	if err != nil {
		t.Fatal(err)
	}

	get := func(extra map[string]any) (*mcp.CallToolResult, evidenceView) {
		args := map[string]any{"evidence_ref": ev.Ref, "include_raw_detail": true}
		for k, v := range extra {
			args[k] = v
		}
		res := callTool(t, cs, "casebook_get_evidence", args)
		return res, decode[evidenceView](t, res)
	}

	t.Run("default window", func(t *testing.T) {
		res, v := get(nil)
		n := len(v.RawDetail)
		if n > 16384 || n == 0 || !utf8.ValidString(v.RawDetail) || *v.RawDetailBytes != len(raw) ||
			!*v.RawDetailTruncated || v.RawDetailNextOffset != n || *v.RawDetailOffset != 0 {
			t.Fatalf("window %d bytes, view bytes %d truncated %v next %d", n, *v.RawDetailBytes, *v.RawDetailTruncated, v.RawDetailNextOffset)
		}
		if n != 16383 {
			t.Errorf("window = %d bytes, want 16383 (moved back to a rune start)", n)
		}
		if !strings.Contains(resultText(res), "Raw detail (bytes 0 to 16383 of 65536):") {
			t.Errorf("text header missing:\n%.300s", resultText(res))
		}
		_, next := get(map[string]any{"raw_detail_offset": v.RawDetailNextOffset})
		if *next.RawDetailOffset != n || next.RawDetail != raw[n:n+len(next.RawDetail)] {
			t.Errorf("next page offset %d does not continue", *next.RawDetailOffset)
		}
	})

	t.Run("pages cover the whole text", func(t *testing.T) {
		var b strings.Builder
		offset := 0
		for i := 0; i < 20; i++ {
			_, v := get(map[string]any{"raw_detail_offset": offset, "max_raw_bytes": 20000})
			b.WriteString(v.RawDetail)
			if !*v.RawDetailTruncated {
				break
			}
			offset = v.RawDetailNextOffset
		}
		if b.String() != raw {
			t.Fatalf("pages rebuild %d bytes, want %d", b.Len(), len(raw))
		}
	})

	t.Run("mid-rune offset moves back", func(t *testing.T) {
		_, v := get(map[string]any{"raw_detail_offset": 1, "max_raw_bytes": 10})
		if *v.RawDetailOffset != 0 || v.RawDetail != "日本語" || v.RawDetailNextOffset != 9 {
			t.Fatalf("offset %d window %q next %d", *v.RawDetailOffset, v.RawDetail, v.RawDetailNextOffset)
		}
	})

	t.Run("window smaller than a rune still advances", func(t *testing.T) {
		_, v := get(map[string]any{"max_raw_bytes": 1})
		if v.RawDetail != "日" || v.RawDetailNextOffset != 3 {
			t.Fatalf("window %q next %d", v.RawDetail, v.RawDetailNextOffset)
		}
	})

	t.Run("offset at the end", func(t *testing.T) {
		_, v := get(map[string]any{"raw_detail_offset": len(raw)})
		if v.RawDetail != "" || *v.RawDetailTruncated || *v.RawDetailOffset != len(raw) {
			t.Fatalf("view = %+v", v)
		}
	})

	t.Run("maximum window shrinks to the budget", func(t *testing.T) {
		res, v := get(map[string]any{"max_raw_bytes": 65536})
		b, err := json.Marshal(res)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) >= MaxResponseBytes {
			t.Fatalf("result is %d bytes", len(b))
		}
		if !*v.RawDetailTruncated || !utf8.ValidString(v.RawDetail) || v.RawDetailNextOffset != len(v.RawDetail) {
			t.Fatalf("window %d bytes truncated %v next %d", len(v.RawDetail), *v.RawDetailTruncated, v.RawDetailNextOffset)
		}
	})
}

func TestGetEvidenceNoActive(t *testing.T) {
	a := apptest.Open(t)
	cs := newTestSession(t, a, sessionOpts{})
	wantToolError(t, callTool(t, cs, "casebook_get_evidence", map[string]any{"evidence_ref": "E-001"}), "casebook_set_active_investigation")
}

// TestEvidenceMetadataExact is the WR-02 regression: metadata sent through
// MCP is stored exactly as sent (compacted), so integers beyond 2^53 keep
// every digit and <, > and & stay literal, for add_evidence and checkpoint.
func TestEvidenceMetadataExact(t *testing.T) {
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	ctx := context.Background()
	const sent = `{"id": 12345678901234567890, "html": "<b>", "ratio": 1.10, "nested": {"n": -0.000001e-3}}`
	const want = `{"id":12345678901234567890,"html":"<b>","ratio":1.10,"nested":{"n":-0.000001e-3}}`
	rawCall := func(t *testing.T, name, args string) *mcp.CallToolResult {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: json.RawMessage(args)})
		if err != nil {
			t.Fatalf("call %s: %v", name, err)
		}
		if res.IsError {
			t.Fatalf("tool error: %s", resultText(res))
		}
		return res
	}
	stored := func(t *testing.T, ref string) string {
		t.Helper()
		ev, err := a.GetEvidence(ctx, "", ref, app.GetEvidenceOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return ev.Metadata
	}

	t.Run("add_evidence", func(t *testing.T) {
		res := rawCall(t, "casebook_add_evidence",
			`{"title":"Big id","summary":"An id above 2^53.","source_type":"api_response","metadata":`+sent+`}`)
		ref := decode[mutationResult[evidenceRecord]](t, res).Ref
		if got := stored(t, ref); got != want {
			t.Fatalf("stored metadata = %s, want %s", got, want)
		}
		v := decode[evidenceView](t, callTool(t, cs, "casebook_get_evidence", map[string]any{"evidence_ref": ref}))
		if v.Metadata != want {
			t.Errorf("get_evidence metadata = %s, want %s", v.Metadata, want)
		}
	})
	t.Run("empty object means none", func(t *testing.T) {
		for _, m := range []string{`{ }`, `{}`} {
			res := rawCall(t, "casebook_add_evidence", `{"title":"None","summary":"No metadata.","source_type":"code","metadata":`+m+`}`)
			if got := stored(t, decode[mutationResult[evidenceRecord]](t, res).Ref); got != "" {
				t.Errorf("metadata %s stored as %q", m, got)
			}
		}
	})
	t.Run("checkpoint", func(t *testing.T) {
		res := rawCall(t, "casebook_checkpoint", `{"note":"Exact metadata.","evidence":[`+
			`{"title":"Plain","summary":"No metadata.","source_type":"code"},`+
			`{"title":"Big id","summary":"An id above 2^53.","source_type":"code","metadata":`+sent+`}]}`)
		out := decode[checkpointOut](t, res)
		if len(out.Created) != 2 {
			t.Fatalf("created = %v", out.Created)
		}
		if got := stored(t, out.Created[0]); got != "" {
			t.Errorf("first evidence metadata = %q, want none", got)
		}
		if got := stored(t, out.Created[1]); got != want {
			t.Fatalf("stored metadata = %s, want %s", got, want)
		}
	})
}
