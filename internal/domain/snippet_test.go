package domain

import (
	"reflect"
	"testing"
)

func TestParseSnippet(t *testing.T) {
	cases := []struct {
		raw     string
		display string
		segs    []SnippetSegment
	}{
		{"", "", []SnippetSegment{}},
		{"plain", "plain", []SnippetSegment{{"plain", false}}},
		{"a \x02b\x03 c", "a **b** c", []SnippetSegment{{"a ", false}, {"b", true}, {" c", false}}},
		{"\x02x\x03\x02y\x03", "**x****y**", []SnippetSegment{{"xy", true}}},
		{"has ** stars \x02hit\x03", "has ** stars **hit**", []SnippetSegment{{"has ** stars ", false}, {"hit", true}}},
		{"stray \x03 end", "stray  end", []SnippetSegment{{"stray  end", false}}},
		{"open \x02never closed", "open **never closed**", []SnippetSegment{{"open ", false}, {"never closed", true}}},
	}
	for _, c := range cases {
		d, s := ParseSnippet(c.raw)
		if d != c.display || !reflect.DeepEqual(s, c.segs) {
			t.Errorf("ParseSnippet(%q) = %q %+v, want %q %+v", c.raw, d, s, c.display, c.segs)
		}
	}
}
