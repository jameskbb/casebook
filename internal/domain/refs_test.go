package domain

import (
	"strings"
	"testing"
)

func TestRefsFormat(t *testing.T) {
	cases := map[string]string{
		FormatInvestigationRef(1):               "INV-0001",
		FormatInvestigationRef(12345):           "INV-12345",
		FormatRecordRef(RecordClaim, 7):         "C-007",
		FormatRecordRef(RecordEvidence, 1):      "E-001",
		FormatRecordRef(RecordHypothesis, 1234): "H-1234",
		FormatRecordRef(RecordQuestion, 42):     "Q-042",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("format = %q, want %q", got, want)
		}
	}
}

func TestRefsParseInvestigation(t *testing.T) {
	for _, s := range []string{"INV-0001", "inv-1", " INV-1 ", "INV-00001", "Inv-1"} {
		n, err := ParseInvestigationRef(s)
		if err != nil || n != 1 {
			t.Errorf("ParseInvestigationRef(%q) = %d, %v; want 1", s, n, err)
		}
	}
	if n, err := ParseInvestigationRef("INV-999999999999"); err != nil || n != 999999999999 {
		t.Errorf("max ref = %d, %v", n, err)
	}
	for _, s := range []string{"INV-0", "INV--1", "INV-1x", "E-001", "", "INV-", "INV1", "INV-+1", "INV-1000000000000", "INV-0001/E-001"} {
		_, err := ParseInvestigationRef(s)
		ve, ok := IsValidation(err)
		if !ok || ve.Field != "investigation_ref" {
			t.Errorf("ParseInvestigationRef(%q) = %v, want ValidationError on investigation_ref", s, err)
		}
	}
}

func TestRefsParseRecord(t *testing.T) {
	r, err := ParseRecordRef("e-1", RecordEvidence)
	if err != nil || r.Num != 1 || r.InvestigationNum != 0 {
		t.Fatalf("ParseRecordRef(e-1) = %+v, %v", r, err)
	}
	r, err = ParseRecordRef("INV-0003/C-002", RecordClaim)
	if err != nil || r.InvestigationNum != 3 || r.Num != 2 {
		t.Fatalf("ParseRecordRef(INV-0003/C-002) = %+v, %v", r, err)
	}
	r, err = ParseRecordRef(" inv-3/c-2 ", RecordClaim)
	if err != nil || r.InvestigationNum != 3 || r.Num != 2 {
		t.Fatalf("ParseRecordRef(inv-3/c-2) = %+v, %v", r, err)
	}
	for _, c := range []struct {
		s    string
		want RecordType
	}{{"H-012", RecordHypothesis}, {"q-1", RecordQuestion}, {"E-001", RecordEvidence}} {
		if _, err := ParseRecordRef(c.s, c.want); err != nil {
			t.Errorf("ParseRecordRef(%q, %s) = %v", c.s, c.want, err)
		}
	}
}

func TestRefsParseRecordWrongPrefix(t *testing.T) {
	_, err := ParseRecordRef("C-001", RecordEvidence)
	ve, ok := IsValidation(err)
	if !ok || ve.Field != "ref" {
		t.Fatalf("ParseRecordRef(C-001, evidence) = %v, want ValidationError on ref", err)
	}
	if !strings.Contains(ve.Message, "expected an evidence ref such as E-001") || !strings.Contains(ve.Message, "C-001") {
		t.Fatalf("message = %q", ve.Message)
	}
	for _, s := range []string{"", "E-", "E-0", "E-1x", "E--1", "X-001", "INV-1/", "INV-1/E-1/E-2", "INV-0/E-1", "/E-1", "INV-1 / E-1"} {
		_, err := ParseRecordRef(s, RecordEvidence)
		ve, ok := IsValidation(err)
		if !ok || ve.Field != "ref" {
			t.Errorf("ParseRecordRef(%q) = %v, want ValidationError on ref", s, err)
		}
	}
	if _, err := ParseRecordRef("INV-0001", RecordInvestigation); err == nil {
		t.Error("ParseRecordRef with RecordInvestigation should fail")
	}
}
