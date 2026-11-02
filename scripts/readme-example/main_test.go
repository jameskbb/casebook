package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadmeExample: the scenario builds through internal/app and the
// compact pack keeps the lines the README quotes.
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
		"Investigation: INV-0002",
		"C-003 [High, Human verified]",
		"C-002 [High, AI generated]",
		"Disputed claims:\nC-001 [Low, Disputed]",
		"H-003 [High, Supported, AI generated]",
		"Known disproven hypotheses:\nH-001 July contractor hires",
		"Contradicted by: E-005",
		"Deferred questions:\nQ-004",
		"ai claude-code: Resolved question Q-002",
		"human casebook-ui: Set verification of C-003 to human_verified",
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
