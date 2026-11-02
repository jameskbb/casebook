package mcpserver

import (
	"context"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// toolSpec is what TestListTools expects of one tool.
type toolSpec struct {
	input      any // zero value of the input struct
	readOnly   bool
	idempotent bool
	// scoped tools default to the active investigation; their description
	// must say so.
	scoped bool
}

// toolSpecs is the D-12 tool table.
var toolSpecs = map[string]toolSpec{
	"casebook_list_investigations":         {listInvestigationsInput{}, true, true, false},
	"casebook_search_investigations":       {searchInput{}, true, true, false},
	"casebook_get_investigation":           {getInvestigationInput{}, true, true, true},
	"casebook_get_active_investigation":    {getActiveInput{}, true, true, false},
	"casebook_get_evidence":                {getEvidenceInput{}, true, true, true},
	"casebook_get_context":                 {getContextInput{}, true, true, true},
	"casebook_get_report":                  {getReportInput{}, true, true, true},
	"casebook_create_investigation":        {createInvestigationInput{}, false, false, false},
	"casebook_add_evidence":                {addEvidenceInput{}, false, false, true},
	"casebook_add_claim":                   {addClaimInput{}, false, false, true},
	"casebook_add_hypothesis":              {addHypothesisInput{}, false, false, true},
	"casebook_add_question":                {addQuestionInput{}, false, false, true},
	"casebook_checkpoint":                  {checkpointInput{}, false, false, true},
	"casebook_set_active_investigation":    {setActiveInput{}, false, true, false},
	"casebook_update_investigation":        {updateInvestigationInput{}, false, true, true},
	"casebook_update_claim":                {updateClaimInput{}, false, true, true},
	"casebook_link_evidence_to_claim":      {linkClaimInput{}, false, true, true},
	"casebook_update_hypothesis":           {updateHypothesisInput{}, false, true, true},
	"casebook_link_evidence_to_hypothesis": {linkHypothesisInput{}, false, true, true},
	"casebook_resolve_question":            {resolveQuestionInput{}, false, true, true},
	"casebook_update_conclusion":           {updateConclusionInput{}, false, true, true},
}

// jsonFields returns the JSON names of the visible fields of struct type t
// (promoted fields of embedded structs included), and which are required
// (no omitempty).
func jsonFields(t reflect.Type) (all map[string]reflect.StructField, required []string) {
	all = map[string]reflect.StructField{}
	for _, f := range reflect.VisibleFields(t) {
		if f.Anonymous || !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if name == "" {
			name = f.Name
		}
		all[name] = f
		if !slices.Contains(strings.Split(opts, ","), "omitempty") {
			required = append(required, name)
		}
	}
	slices.Sort(required)
	return all, required
}

// requiredJSONFields lists the non-omitempty JSON field names of t, sorted.
func requiredJSONFields(t reflect.Type) []string {
	_, req := jsonFields(t)
	return req
}

func schemaStrings(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, x := range list {
		s, _ := x.(string)
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

// checkSchema compares the decoded JSON schema of one value with its Go type:
// struct schemas have exactly the JSON fields as properties and the
// non-omitempty ones as required; every enum-typed value (after pointer and
// slice unwrapping) lists enumValues (plus null for pointer fields). Every
// property has a description, except below the top level when topOnly is set
// (casebook_checkpoint describes batch item fields on the single-record
// tools, AR-05).
func checkSchema(t *testing.T, schema map[string]any, typ reflect.Type, path string, topOnly bool) {
	t.Helper()
	nullable := false
	for typ.Kind() == reflect.Pointer {
		typ, nullable = typ.Elem(), true
	}
	if want, ok := enumValues[typ]; ok {
		want = slices.Clone(want)
		if nullable {
			want = append(want, nil)
		}
		if got, _ := schema["enum"].([]any); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: enum = %v, want %v", path, got, want)
		}
		return
	}
	switch typ.Kind() {
	case reflect.Slice:
		items, ok := schema["items"].(map[string]any)
		if !ok {
			t.Errorf("%s: array schema without items: %v", path, schema)
			return
		}
		checkSchema(t, items, typ.Elem(), path+"[]", topOnly)
	case reflect.Struct:
		props, _ := schema["properties"].(map[string]any)
		fields, required := jsonFields(typ)
		if got := schemaStrings(schema["required"]); !slices.Equal(got, required) {
			t.Errorf("%s: required = %v, want %v", path, got, required)
		}
		if len(props) != len(fields) {
			t.Errorf("%s: %d properties, want %d", path, len(props), len(fields))
		}
		for name, f := range fields {
			p, ok := props[name].(map[string]any)
			if !ok {
				t.Errorf("%s: no property %s", path, name)
				continue
			}
			d, _ := p["description"].(string)
			switch {
			case path == "input" && d == "":
				t.Errorf("%s.%s: no description", path, name)
			case path != "input" && !topOnly && d == "":
				t.Errorf("%s.%s: no description", path, name)
			case path != "input" && topOnly && d != "":
				t.Errorf("%s.%s: nested description %q repeats a single-record tool", path, name, d)
			}
			checkSchema(t, p, f.Type, path+"."+name, topOnly)
		}
	}
}

func TestRequiredJSONFields(t *testing.T) {
	if got := requiredJSONFields(reflect.TypeFor[checkpointClaimUpdate]()); !slices.Equal(got, []string{"ref"}) {
		t.Fatalf("checkpointClaimUpdate required = %v", got)
	}
	if got := requiredJSONFields(reflect.TypeFor[addEvidenceInput]()); !slices.Equal(got, []string{"source_type", "summary", "title"}) {
		t.Fatalf("addEvidenceInput required = %v", got)
	}
}

func TestListTools(t *testing.T) {
	if len(toolSpecs) != 21 {
		t.Fatalf("toolSpecs has %d tools, want 21", len(toolSpecs))
	}
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	var names []string
	for tool, err := range cs.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		names = append(names, tool.Name)
		spec, ok := toolSpecs[tool.Name]
		if !ok {
			t.Errorf("unexpected tool %s", tool.Name)
			continue
		}
		t.Run(tool.Name, func(t *testing.T) {
			d := tool.Description
			if d == "" || utf8.RuneCountInString(d) > 900 {
				t.Errorf("description has %d runes, want 1 to 900", utf8.RuneCountInString(d))
			}
			if strings.ContainsRune(d, '\u2014') || strings.ContainsRune(tool.Title, '\u2014') {
				t.Error("em dash in description or title")
			}
			if tool.Title == "" {
				t.Error("no title")
			}
			in, ok := tool.InputSchema.(map[string]any)
			if !ok || in["type"] != "object" {
				t.Fatalf("input schema %v is not an object schema", tool.InputSchema)
			}
			// The active-investigation default is explained once in the
			// server instructions and on the property, not in every
			// description (AR-05).
			if spec.scoped {
				props, _ := in["properties"].(map[string]any)
				p, _ := props["investigation_ref"].(map[string]any)
				if pd, _ := p["description"].(string); !strings.Contains(pd, "active investigation") {
					t.Errorf("scoped tool investigation_ref description %q must name the active investigation", pd)
				}
			}
			checkSchema(t, in, reflect.TypeOf(spec.input), "input", tool.Name == "casebook_checkpoint")
			if got, want := schemaStrings(in["required"]), requiredJSONFields(reflect.TypeOf(spec.input)); !slices.Equal(got, want) {
				t.Errorf("required = %v, want %v", got, want)
			}
			if out, ok := tool.OutputSchema.(map[string]any); !ok || out["type"] != "object" {
				t.Errorf("output schema %v is not an object schema", tool.OutputSchema)
			}
			ann := tool.Annotations
			if ann == nil {
				t.Fatal("no annotations")
			}
			if ann.ReadOnlyHint != spec.readOnly || ann.IdempotentHint != spec.idempotent {
				t.Errorf("readOnly %v idempotent %v, want %v %v", ann.ReadOnlyHint, ann.IdempotentHint, spec.readOnly, spec.idempotent)
			}
			if ann.DestructiveHint == nil || *ann.DestructiveHint {
				t.Error("destructiveHint must be false")
			}
			if ann.OpenWorldHint == nil || *ann.OpenWorldHint {
				t.Error("openWorldHint must be false")
			}
		})
	}
	want := make([]string, 0, len(toolSpecs))
	for name := range toolSpecs {
		want = append(want, name)
	}
	slices.Sort(want)
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
}

// TestEveryToolHasATest checks that every tool is called by name in a test
// file other than the phase-level ones (D-22).
func TestEveryToolHasATest(t *testing.T) {
	skip := map[string]bool{"listtools_test.go": true, "size_test.go": true, "doc_test.go": true, "flow_test.go": true}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var src strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") || skip[e.Name()] {
			continue
		}
		b, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		src.Write(b)
	}
	for name := range toolSpecs {
		if !strings.Contains(src.String(), `"`+name+`"`) {
			t.Errorf("tool %s has no test (no string literal %q in a *_test.go file)", name, name)
		}
	}
}
