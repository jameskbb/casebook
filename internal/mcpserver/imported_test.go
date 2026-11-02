package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app/apptest"
)

// TestImportedMarked proves rows copied from an imported file are marked in
// the read tools (AR-09): imported true in the JSON and "(imported)" next to
// the verification in the text, while native rows carry neither.
func TestImportedMarked(t *testing.T) {
	ctx := context.Background()
	a, f := seeded(t)
	data, err := a.ExportInvestigation(ctx, f.InvestigationRef)
	if err != nil {
		t.Fatal(err)
	}
	res, err := a.ImportInvestigation(ctx, apptest.Human, data)
	if err != nil {
		t.Fatal(err)
	}
	cs := newTestSession(t, a, sessionOpts{})

	for ref, want := range map[string]bool{res.InvestigationRef: true, f.InvestigationRef: false} {
		got := callTool(t, cs, "casebook_get_investigation", map[string]any{"investigation_ref": ref})
		raw, _ := json.Marshal(got.StructuredContent)
		if strings.Contains(string(raw), `"imported":true`) != want || strings.Contains(resultText(got), "(imported)") != want {
			t.Errorf("%s: imported marks = %v, want %v\n%s", ref, !want, want, resultText(got))
		}
		ev := callTool(t, cs, "casebook_get_evidence", map[string]any{"investigation_ref": ref, "evidence_ref": "E-001"})
		if strings.Contains(resultText(ev), "(imported)") != want {
			t.Errorf("%s E-001: text = %s", ref, resultText(ev))
		}
	}
}
