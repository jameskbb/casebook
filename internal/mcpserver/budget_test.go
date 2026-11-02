package mcpserver

import (
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

func TestLimitParam(t *testing.T) {
	for _, c := range []struct{ v, want int }{{0, 20}, {1, 1}, {100, 100}} {
		got, err := limitParam("limit", c.v, defaultListLimit, maxListLimit)
		if err != nil || got != c.want {
			t.Errorf("limitParam(%d) = %d, %v; want %d", c.v, got, err, c.want)
		}
	}
	for _, v := range []int{101, -1} {
		_, err := limitParam("limit", v, defaultListLimit, maxListLimit)
		ve, ok := app.IsValidation(err)
		if !ok || ve.Field != "limit" {
			t.Errorf("limitParam(%d) error = %v, want a ValidationError on limit", v, err)
		}
	}
}

func TestResultBytes(t *testing.T) {
	small := resultBytes("hi", map[string]any{"a": 1})
	if small <= 0 || small > 200 {
		t.Errorf("small result = %d bytes", small)
	}
	big := resultBytes(strings.Repeat("x", 1000), map[string]any{"a": strings.Repeat("y", 1000)})
	if big < 2000 {
		t.Errorf("big result = %d bytes, want at least 2000", big)
	}
}

func TestBudgetConstants(t *testing.T) {
	if MaxResponseBytes != 65536 || responseTarget >= MaxResponseBytes {
		t.Errorf("MaxResponseBytes %d, responseTarget %d", MaxResponseBytes, responseTarget)
	}
	if maxContextMaxBytes*2 > responseTarget {
		t.Errorf("maxContextMaxBytes %d cannot fit twice in %d", maxContextMaxBytes, responseTarget)
	}
}

func TestOneLine(t *testing.T) {
	got := oneLine("a\nb\t" + strings.Repeat("z", 300))
	if strings.ContainsAny(got, "\n\t") || len(got) > 240 || !strings.HasSuffix(got, "...") {
		t.Errorf("oneLine = %q (%d bytes)", got, len(got))
	}
}

func TestChangedFields(t *testing.T) {
	got := changedFields([]string{"title", "status", "problem_statement"},
		map[string]string{"title": "a", "status": "new", "problem_statement": "p"},
		map[string]string{"title": "b", "status": "new", "problem_statement": "q"})
	if strings.Join(got, ",") != "title,problem_statement" {
		t.Errorf("changedFields = %v", got)
	}
	none := changedFields([]string{"title"}, map[string]string{"title": "a"}, map[string]string{"title": "a"})
	if none == nil || len(none) != 0 {
		t.Errorf("no change = %#v, want empty non-nil", none)
	}
}
