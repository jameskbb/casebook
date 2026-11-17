package app

import (
	"strings"
	"testing"
)

func TestSearchBuildMatchQuery(t *testing.T) {
	cases := []struct{ in, want string }{
		{"debug", `"debug"*`},
		{"Log ingestion", `"Log" "ingestion"*`},
		{`a"b c`, `"a""b" "c"*`},
		{`"quoted phrase"`, `"""quoted" "phrase"""*`},
		{"AND OR NOT", `"AND" "OR" "NOT"*`},
		{"NEAR(", `"NEAR("*`},
		{"title:debug", `"title:debug"*`},
		{"^x", `"^x"*`},
		{`" "" * ^ -- ;`, ""},
		{"\U0001F600", ""},
		{"", ""},
		{"x \U0001F600", `"x"*`},
	}
	for _, c := range cases {
		if got := buildMatchQuery(c.in); got != c.want {
			t.Errorf("buildMatchQuery(%q) = %s, want %s", c.in, got, c.want)
		}
	}

	// At most 16 terms; only the last kept term is a prefix.
	got := buildMatchQuery(strings.Repeat("w ", 20))
	if n := strings.Count(got, `"w"`); n != 16 {
		t.Fatalf("kept %d terms, want 16: %s", n, got)
	}
	if strings.Count(got, "*") != 1 || !strings.HasSuffix(got, `"w"*`) {
		t.Fatalf("prefix marker misplaced: %s", got)
	}
}
