package main

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

// TestFrontendLimitsMatchEngine: the counters in frontend/src/lib/bytes.ts
// are copies of the engine's byte limits (AR-28); this keeps them equal.
func TestFrontendLimitsMatchEngine(t *testing.T) {
	want := map[string]int{
		"title":               app.MaxTitleBytes,
		"problemStatement":    app.MaxProblemStatementBytes,
		"conclusion":          app.MaxConclusionBytes,
		"conclusionRationale": app.MaxConclusionRationaleBytes,
		"evidenceSummary":     app.MaxEvidenceSummaryBytes,
		"rawDetail":           app.MaxRawDetailBytes,
		"sourceRef":           app.MaxSourceRefBytes,
		"sourceLabel":         app.MaxSourceLabelBytes,
		"metadata":            app.MaxMetadataBytes,
		"statement":           app.MaxStatementBytes,
		"rationale":           app.MaxRationaleBytes,
		"question":            app.MaxQuestionBytes,
		"resolution":          app.MaxResolutionBytes,
		"deferNote":           app.MaxDeferNoteBytes,
		"searchQuery":         app.MaxSearchQueryBytes,
	}
	b, err := os.ReadFile("frontend/src/lib/bytes.ts")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)export const LIMITS = \{(.*?)\}`).FindSubmatch(b)
	if block == nil {
		t.Fatal("LIMITS not found in bytes.ts")
	}
	got := map[string]int{}
	for _, m := range regexp.MustCompile(`(\w+):\s*(\d+)`).FindAllSubmatch(block[1], -1) {
		n, _ := strconv.Atoi(string(m[2]))
		got[string(m[1])] = n
	}
	if len(got) != len(want) {
		t.Errorf("bytes.ts has %d limits, the test knows %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("LIMITS.%s = %d, engine says %d", k, got[k], v)
		}
	}
}
