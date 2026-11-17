package domain

import (
	"strings"
	"testing"
)

func TestNormalizeMetadataEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "\n\t"} {
		got, err := NormalizeMetadata("metadata", in)
		if err != nil || got != "" {
			t.Fatalf("NormalizeMetadata(%q) = %q, %v; want \"\", nil", in, got, err)
		}
	}
}

func TestNormalizeMetadataCompacts(t *testing.T) {
	got, err := NormalizeMetadata("metadata", "{ \"rows\" : 22 }")
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"rows":22}` {
		t.Fatalf("got %q, want {\"rows\":22}", got)
	}
	got, err = NormalizeMetadata("metadata", "\n {\"a\": {\"b\": [1, 2, \"x y\"]}}\n")
	if err != nil || got != `{"a":{"b":[1,2,"x y"]}}` {
		t.Fatalf("nested: %q, %v", got, err)
	}
}

func TestNormalizeMetadataRejects(t *testing.T) {
	big := `{"k":"` + strings.Repeat("x", MaxMetadataBytes-8+1) + `"}`
	if len(big) != MaxMetadataBytes+1 {
		t.Fatalf("test setup: len %d", len(big))
	}
	exact := `{"k":"` + strings.Repeat("x", MaxMetadataBytes-8) + `"}`
	if _, err := NormalizeMetadata("metadata", exact); err != nil {
		t.Fatalf("exactly %d bytes rejected: %v", MaxMetadataBytes, err)
	}
	deep := strings.Repeat(`{"a":`, 200) + "1" + strings.Repeat("}", 200)
	for name, in := range map[string]string{
		"array":        "[1,2]",
		"string":       `"x"`,
		"number":       "42",
		"null":         "null",
		"invalid":      "{bad",
		"trailing":     `{"a":1} {"b":2}`,
		"too big":      big,
		"invalid utf8": "{\"a\":\"\xff\"}",
		"too deep":     deep,
	} {
		_, err := NormalizeMetadata("metadata", in)
		ve, ok := IsValidation(err)
		if !ok || ve.Field != "metadata" {
			t.Errorf("%s: err = %v, want ValidationError on metadata", name, err)
		}
	}
}
