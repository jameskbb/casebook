package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/demo"
)

// TestReadmeExample: the demo story builds through internal/app and the
// compact pack of the featured question keeps the lines the README quotes.
func TestReadmeExample(t *testing.T) {
	var a, b bytes.Buffer
	if err := run(filepath.Join(t.TempDir(), "a.db"), "compact", 0, &a); err != nil {
		t.Fatal(err)
	}
	if err := run(filepath.Join(t.TempDir(), "b.db"), "compact", 0, &b); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatal("two runs produced different packs; the example must be deterministic")
	}
	for _, want := range []string{
		"Question: " + demo.FeaturedRef + "\nTitle: " + demo.FeaturedTitle + "\n",
		"Answer so far:\n" + demo.Conclusion + "\n",
		"Answer marked: A person checked this",
		"C-003 [Not checked]",
		"E-007 [A person checked this]",
		"Findings where sources disagree:\nC-001 [Sources disagree]",
		"H-003 [Looks right so far, Not checked]",
		"H-004 [Not tested yet, Not checked]",
		"Findings ruled out:\nH-001 A new ad brought in more callers.",
		"Ruled out by: E-004",
		"Still to find out, left for later:\nQ-004",
		"E-001 Calls to buy: 2,000 in September, 2,050 in October.",
		"[AI checked the source]",
		"human casebook-ui: Checked the source E-007",
	} {
		if !strings.Contains(a.String(), want) {
			t.Errorf("pack lacks %q:\n%s", want, a.String())
		}
	}
}

func TestReadmeExampleRejectsExistingDB(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.db")
	if err := run(p, "compact", 0, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := run(p, "compact", 0, &bytes.Buffer{}); err == nil {
		t.Fatal("a second run on the same database succeeded")
	}
}
