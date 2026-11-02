package mcpserver

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

func enumOf[T ~string](vs []T) []any { return anyValues(vs) }

func TestInputSchemaAddEvidence(t *testing.T) {
	s := inputSchema[addEvidenceInput]()
	if s.Type != "object" {
		t.Fatalf("type = %q, want object", s.Type)
	}
	req := slices.Clone(s.Required)
	slices.Sort(req)
	if want := []string{"source_type", "summary", "title"}; !slices.Equal(req, want) {
		t.Errorf("required = %v, want %v", req, want)
	}
	if got := s.Properties["source_type"].Enum; !reflect.DeepEqual(got, enumOf(app.AllSourceTypes())) {
		t.Errorf("source_type enum = %v", got)
	}
	if got, want := s.Properties["verification"].Enum, []any{"ai_generated", "source_verified", "disputed", "superseded"}; !reflect.DeepEqual(got, want) {
		t.Errorf("verification enum = %v", got)
	}
	if _, ok := s.Properties["investigation_ref"]; !ok {
		t.Error("investigation_ref property missing")
	}
	if s.AdditionalProperties == nil || s.AdditionalProperties.Not == nil {
		t.Errorf("additionalProperties must be false, got %+v", s.AdditionalProperties)
	}
	for name, p := range s.Properties {
		if p.Description == "" {
			t.Errorf("property %s has no description", name)
		}
	}
}

type schemaProbe struct {
	C *app.Confidence `json:"c,omitempty"`
	L []struct {
		R app.Relation `json:"r"`
	} `json:"l,omitempty"`
	F contextFormat          `json:"f,omitempty"`
	S []investigationSection `json:"s,omitempty"`
}

func TestInputSchemaNestedEnums(t *testing.T) {
	s := inputSchema[schemaProbe]()
	wantC := append(enumOf(app.AllConfidences()), nil)
	if got := s.Properties["c"].Enum; !reflect.DeepEqual(got, wantC) {
		t.Errorf("pointer enum = %v, want %v", got, wantC)
	}
	if !slices.Contains(s.Properties["c"].Types, "null") {
		t.Errorf("pointer types = %v, want null included", s.Properties["c"].Types)
	}
	items := s.Properties["l"].Items
	if items == nil {
		t.Fatal("array items schema missing")
	}
	if got := items.Properties["r"].Enum; !reflect.DeepEqual(got, enumOf(app.AllRelations())) {
		t.Errorf("array item enum = %v", got)
	}
	if got := s.Properties["f"].Enum; !reflect.DeepEqual(got, []any{"markdown", "json"}) {
		t.Errorf("contextFormat enum = %v", got)
	}
	if got := s.Properties["s"].Items.Enum; !reflect.DeepEqual(got, []any{"claims", "hypotheses", "questions", "evidence", "activity"}) {
		t.Errorf("section enum = %v", got)
	}
	// The schema must resolve and accept null for the pointer enum.
	r, err := s.Resolve(nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if err := r.Validate(map[string]any{"c": nil}); err != nil {
		t.Errorf("null pointer enum rejected: %v", err)
	}
	if err := r.Validate(map[string]any{"c": "bogus"}); err == nil {
		t.Error("bogus enum value accepted")
	}
}

func TestEnumValuesCoverAppEnums(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeFor[app.InvestigationStatus](), reflect.TypeFor[app.Confidence](),
		reflect.TypeFor[app.Verification](), reflect.TypeFor[app.ClaimStatus](),
		reflect.TypeFor[app.HypothesisStatus](), reflect.TypeFor[app.QuestionStatus](),
		reflect.TypeFor[app.SourceType](), reflect.TypeFor[app.Relation](), reflect.TypeFor[app.RecordType](),
	} {
		if len(enumValues[typ]) == 0 {
			t.Errorf("enumValues has no entry for %v", typ)
		}
	}
}

// TestStatementVerificationEnum proves claims and hypotheses are never
// offered verification superseded (only superseded_by sets it, ADR-0013),
// no input offers human_verified (only a person sets it, ADR-0014), and
// evidence keeps superseded.
func TestStatementVerificationEnum(t *testing.T) {
	want := []any{"ai_generated", "source_verified", "disputed"}
	for name, s := range map[string]map[string]any{
		"update_conclusion": schemaMap(t, inputSchema[updateConclusionInput]()),
		"add_claim":         schemaMap(t, inputSchema[addClaimInput]()),
		"add_hypothesis":    schemaMap(t, inputSchema[addHypothesisInput]()),
		"update_claim":      schemaMap(t, inputSchema[updateClaimInput]()),
		"update_hypothesis": schemaMap(t, inputSchema[updateHypothesisInput]()),
	} {
		props, _ := s["properties"].(map[string]any)
		v, _ := props["verification"].(map[string]any)
		got, _ := v["enum"].([]any)
		if len(got) > 0 && got[len(got)-1] == nil {
			got = got[:len(got)-1]
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s verification enum = %v, want %v", name, got, want)
		}
	}
	if got := inputSchema[addEvidenceInput]().Properties["verification"].Enum; !slices.Contains(got, any("superseded")) {
		t.Errorf("evidence verification enum %v lost superseded", got)
	}
	for name, got := range map[string][]any{
		"add_evidence":             inputSchema[addEvidenceInput]().Properties["verification"].Enum,
		"checkpoint evidence":      inputSchema[checkpointInput]().Properties["evidence"].Items.Properties["verification"].Enum,
		"checkpoint claim_updates": inputSchema[checkpointInput]().Properties["claim_updates"].Items.Properties["verification"].Enum,
	} {
		if slices.Contains(got, any("human_verified")) {
			t.Errorf("%s verification enum %v offers human_verified", name, got)
		}
	}
}

// schemaMap returns s as decoded JSON.
func schemaMap(t *testing.T, s any) map[string]any {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
