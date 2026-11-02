package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// RecordLink names a claim or hypothesis linked to a piece of evidence (or the
// reverse) and how it relates.
type RecordLink struct {
	Ref      string   `json:"ref"`
	Relation Relation `json:"relation"`
}

// Evidence is a captured observation with source provenance. RawDetail is
// only filled when requested (GetEvidenceOptions.IncludeRawDetail);
// HasRawDetail says whether any is stored. ClaimLinks and HypothesisLinks are
// never nil.
type Evidence struct {
	ID               int64      `json:"id"`
	Ref              string     `json:"ref"`
	InvestigationRef string     `json:"investigation_ref"`
	Title            string     `json:"title"`
	Summary          string     `json:"summary"`
	RawDetail        string     `json:"raw_detail,omitempty"`
	HasRawDetail     bool       `json:"has_raw_detail"`
	SourceType       SourceType `json:"source_type"`
	SourceRef        string     `json:"source_ref"`
	SourceLabel      string     `json:"source_label"`
	CapturedAt       string     `json:"captured_at"`
	CreatedAt        string     `json:"created_at"`
	UpdatedAt        string     `json:"updated_at"`
	CreatedBy        Actor      `json:"created_by"`
	// Imported: the row came from ImportInvestigation, so its author,
	// timestamps and text are what the file said (AR-09).
	Imported        bool         `json:"imported"`
	Verification    Verification `json:"verification"`
	Metadata        string       `json:"metadata"` // compact JSON object text, or ""
	ClaimLinks      []RecordLink `json:"claim_links"`
	HypothesisLinks []RecordLink `json:"hypothesis_links"`
}

// EvidenceInput is the input to AddEvidence. Title, Summary and SourceType
// are required. CapturedAt is optional RFC3339 (default: the creation time).
// Verification is optional (default by actor kind, see DefaultVerification).
// Metadata is optional JSON object text.
type EvidenceInput struct {
	Title        string       `json:"title"`
	Summary      string       `json:"summary"`
	RawDetail    string       `json:"raw_detail"`
	SourceType   SourceType   `json:"source_type"`
	SourceRef    string       `json:"source_ref"`
	SourceLabel  string       `json:"source_label"`
	CapturedAt   string       `json:"captured_at"`
	Verification Verification `json:"verification"`
	Metadata     string       `json:"metadata"`
}

// EvidencePatch is the input to UpdateEvidence. Nil fields are left
// unchanged; provided fields follow the AddEvidence rules (an empty
// CapturedAt resets it to the creation time; an empty RawDetail or Metadata
// clears it).
type EvidencePatch struct {
	Title        *string       `json:"title,omitempty"`
	Summary      *string       `json:"summary,omitempty"`
	RawDetail    *string       `json:"raw_detail,omitempty"`
	SourceType   *SourceType   `json:"source_type,omitempty"`
	SourceRef    *string       `json:"source_ref,omitempty"`
	SourceLabel  *string       `json:"source_label,omitempty"`
	CapturedAt   *string       `json:"captured_at,omitempty"`
	Verification *Verification `json:"verification,omitempty"`
	Metadata     *string       `json:"metadata,omitempty"`
}

// GetEvidenceOptions tunes GetEvidence.
type GetEvidenceOptions struct {
	IncludeRawDetail bool `json:"include_raw_detail"`
}

// NormalizeMetadata validates optional metadata text: "" (or whitespace)
// stays "", anything else must be a JSON object of at most MaxMetadataBytes
// once compacted. It returns the compact form. Failures are a
// *ValidationError on field.
func NormalizeMetadata(field, s string) (string, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return "", nil
	}
	tooBig := NewValidationError(field, "must be at most %d bytes", MaxMetadataBytes)
	// Bound the parse cost before decoding; the stored limit is checked on
	// the compact form below so indentation does not count against it.
	if len(t) > maxMetadataInputBytes {
		return "", tooBig
	}
	if !utf8.ValidString(t) {
		return "", NewValidationError(field, "must be valid UTF-8")
	}
	if t[0] != '{' {
		return "", NewValidationError(field, "must be a JSON object")
	}
	if err := checkJSONObject(t); err != nil {
		return "", NewValidationError(field, "must be a valid JSON object: %s", err)
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(t)); err != nil {
		return "", NewValidationError(field, "must be a valid JSON object")
	}
	if buf.Len() > MaxMetadataBytes {
		return "", tooBig
	}
	return buf.String(), nil
}

// Metadata parsing bounds.
const (
	maxMetadataInputBytes = 4 * MaxMetadataBytes
	// maxMetadataDepth stays well under SQLite's JSON nesting limit so the
	// json_valid CHECK on evidence.metadata always agrees with this check.
	maxMetadataDepth = 64
)

// checkJSONObject verifies t is exactly one JSON value, nested at most
// maxMetadataDepth deep.
func checkJSONObject(t string) error {
	dec := json.NewDecoder(strings.NewReader(t))
	depth := 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.New("invalid JSON")
		}
		if d, ok := tok.(json.Delim); ok {
			switch d {
			case '{', '[':
				depth++
				if depth > maxMetadataDepth {
					return errors.New("nested too deeply")
				}
			default:
				depth--
			}
		}
		if depth == 0 {
			if _, err := dec.Token(); !errors.Is(err, io.EOF) {
				return errors.New("unexpected data after the object")
			}
			return nil
		}
	}
	return errors.New("invalid JSON")
}
