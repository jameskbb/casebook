package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// Ref number bounds.
//
// MaxRefNum is the single upper bound shared by parsing and allocation:
// ParseInvestigationRef and ParseRecordRef accept 1 to MaxRefNum, and the
// store refuses to allocate a number above it (ErrConflict), so every ref the
// engine ever returns can be parsed back.
//
// Import accepts smaller numbers only (MaxImportInvestigationRefNum and
// MaxImportRecordRefNum), so an untrusted file can never push a counter close
// to MaxRefNum and exhaust the refs of later investigations or records.
const (
	MaxRefNum                    = 999_999_999_999
	MaxImportInvestigationRefNum = 999_999
	MaxImportRecordRefNum        = 99_999
)

// FormatInvestigationRef renders an investigation ref number as INV-0001
// (growing past four digits naturally).
func FormatInvestigationRef(n int64) string {
	return fmt.Sprintf("INV-%04d", n)
}

// FormatRecordRef renders a per-investigation record ref such as E-001,
// C-007, H-1234 or Q-042.
func FormatRecordRef(t RecordType, n int64) string {
	if t == RecordInvestigation {
		return FormatInvestigationRef(n)
	}
	return fmt.Sprintf("%s-%03d", t.Prefix(), n)
}

// ParseInvestigationRef parses an investigation ref leniently and
// case-insensitively: "INV-0001", "inv-1" and " INV-1 " all give 1. Failures
// are a *ValidationError on field investigation_ref.
func ParseInvestigationRef(s string) (int64, error) {
	t := strings.TrimSpace(s)
	n, ok := parsePrefixed(t, "INV")
	if !ok {
		return 0, NewValidationError("investigation_ref", "expected an investigation ref such as INV-0001, got %q", t)
	}
	return n, nil
}

// RecordRef is a parsed record ref. InvestigationNum is 0 unless the
// qualified form INV-0001/E-001 was used.
type RecordRef struct {
	InvestigationNum int64 `json:"investigation_num"`
	Num              int64 `json:"num"`
}

// ParseRecordRef parses a record ref of type want leniently and
// case-insensitively: "E-001", "e-1" and the qualified "INV-0001/E-001" form.
// Failures, including a ref with the wrong prefix, are a *ValidationError on
// field ref.
func ParseRecordRef(s string, want RecordType) (RecordRef, error) {
	t := strings.TrimSpace(s)
	if want == RecordInvestigation || want.Prefix() == "" {
		return RecordRef{}, NewValidationError("ref", "unsupported record type %q", string(want))
	}
	example := FormatRecordRef(want, 1)
	bad := func() (RecordRef, error) {
		return RecordRef{}, NewValidationError("ref", "expected %s %s ref such as %s, got %q",
			article(string(want)), want, example, t)
	}
	var ref RecordRef
	rec := t
	if i := strings.IndexByte(t, '/'); i >= 0 {
		inv, ok := parsePrefixed(t[:i], "INV")
		if !ok {
			return bad()
		}
		ref.InvestigationNum = inv
		rec = t[i+1:]
	}
	n, ok := parsePrefixed(rec, want.Prefix())
	if !ok {
		return bad()
	}
	ref.Num = n
	return ref, nil
}

// article returns "an" before a vowel sound, else "a".
func article(word string) string {
	if word != "" && strings.ContainsRune("aeiou", rune(word[0])) {
		return "an"
	}
	return "a"
}

// parsePrefixed parses "<prefix>-<digits>" case-insensitively. Digits only
// (no sign), value from 1 to MaxRefNum.
func parsePrefixed(s, prefix string) (int64, bool) {
	if len(s) < len(prefix)+2 || !strings.EqualFold(s[:len(prefix)], prefix) || s[len(prefix)] != '-' {
		return 0, false
	}
	digits := s[len(prefix)+1:]
	if digits == "" || len(digits) > 15 {
		return 0, false
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || n < 1 || n > MaxRefNum {
		return 0, false
	}
	return n, true
}
