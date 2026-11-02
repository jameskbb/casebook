package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Free-text limits, in bytes (not runes).
const (
	MaxTitleBytes               = 200
	MaxProblemStatementBytes    = 4000
	MaxConclusionBytes          = 4000
	MaxConclusionRationaleBytes = 2000
	MaxEvidenceSummaryBytes     = 4000
	MaxRawDetailBytes           = 65536
	MaxSourceRefBytes           = 2000
	MaxSourceLabelBytes         = 200
	MaxMetadataBytes            = 16384
	MaxStatementBytes           = 2000
	MaxRationaleBytes           = 2000
	MaxQuestionBytes            = 2000
	MaxResolutionBytes          = 4000
	MaxDeferNoteBytes           = 2000
	MaxActivitySummaryBytes     = 300
	MaxCheckpointNoteBytes      = 1000
	MaxClientBytes              = 64
	MaxSearchQueryBytes         = 500
	MaxImportBytes              = 20971520
)

// SanitizeText cleans a free-text field per D-15: invalid UTF-8 is rejected,
// CRLF and lone CR become LF, control and invisible format characters are
// dropped (see stripControls; tab and LF are kept), surrounding whitespace is
// trimmed, then required and maxBytes are enforced. Errors are
// *ValidationError naming field.
func SanitizeText(field, s string, maxBytes int, required bool) (string, error) {
	if !utf8.ValidString(s) {
		return "", NewValidationError(field, "must be valid UTF-8")
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = stripControls(s)
	s = strings.TrimSpace(s)
	if s == "" {
		if required {
			return "", NewValidationError(field, "is required")
		}
		return "", nil
	}
	if maxBytes > 0 && len(s) > maxBytes {
		return "", NewValidationError(field, "must be at most %d bytes", maxBytes)
	}
	return s, nil
}

// SanitizeLine is SanitizeText for single-line values (titles, labels, client
// names, refs): after cleaning, runs of newlines and tabs become one space.
func SanitizeLine(field, s string, maxBytes int, required bool) (string, error) {
	if !utf8.ValidString(s) {
		return "", NewValidationError(field, "must be valid UTF-8")
	}
	s, err := SanitizeText(field, collapseLine(s), maxBytes, required)
	if err != nil {
		return "", err
	}
	return s, nil
}

// collapseLine maps CR, LF and tab runs to a single space.
func collapseLine(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			if !space {
				b.WriteByte(' ')
				space = true
			}
			continue
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// TruncateLine collapses s to one clean line and truncates it on a rune
// boundary so the result is at most maxBytes, ending in "..." when cut. It
// never fails; use it for activity summaries.
func TruncateLine(s string, maxBytes int) string {
	s = strings.ToValidUTF8(s, "")
	s = stripControls(collapseLine(s))
	s = strings.TrimSpace(s)
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	const ell = "..."
	if maxBytes <= len(ell) {
		return ell[:maxBytes]
	}
	cut := maxBytes - len(ell)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	// The cut can split a flag tag sequence; strip again so no stray tag
	// characters survive.
	return strings.TrimRight(stripControls(s[:cut]), " ") + ell
}

// stripControls drops the characters that are invisible in the desktop app
// and terminal but still reach an AI client, or that make text display in a
// different order than it is stored (Trojan Source):
//
//   - every Cc control (C0, DEL and C1, including U+0085) except tab and LF;
//   - every Cf format character, which covers the bidi embeddings and
//     overrides U+202A to U+202E, the isolates U+2066 to U+2069, zero-width
//     space U+200B, word joiner U+2060, the BOM U+FEFF, soft hyphen U+00AD,
//     U+180E, the invisible math operators, interlinear annotation marks and
//     the Unicode tag characters, except the ones keepFormat lists;
//   - noncharacters such as U+FFFE and U+FFFF.
//
// Tag characters (U+E0020 to U+E007F) can spell hidden ASCII that models
// read and people cannot see, so they are kept only inside an emoji
// subdivision flag sequence (see flagTagLen).
func stripControls(s string) string {
	clean := true
	for _, r := range s {
		if dropRune(r) {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == blackFlag {
			if n := flagTagLen(s[i+size:]); n > 0 {
				b.WriteString(s[i : i+size+n])
				i += size + n
				continue
			}
		}
		if !dropRune(r) {
			b.WriteRune(r)
		}
		i += size
	}
	return b.String()
}

// dropRune reports whether stripControls removes r outside a flag sequence.
func dropRune(r rune) bool {
	switch {
	case r == '\n' || r == '\t':
		return false
	case r < 0x20 || (r >= 0x7f && r <= 0x9f):
		return true
	case isNoncharacter(r):
		return true
	case r < 0xad:
		return false
	}
	return unicode.Is(unicode.Cf, r) && !keepFormat(r)
}

// keepFormat lists the format characters that ordinary text needs:
//
//   - U+200C ZERO WIDTH NON-JOINER and U+200D ZERO WIDTH JOINER, required
//     in Persian, Urdu and Indic scripts and (ZWJ) in emoji sequences such
//     as family and profession emoji;
//   - U+200E LEFT-TO-RIGHT MARK, U+200F RIGHT-TO-LEFT MARK and U+061C
//     ARABIC LETTER MARK, which only set the direction of neighbouring
//     neutral characters (they cannot reorder words) and appear in text
//     copied from Hebrew and Arabic sources;
//   - prepended concatenation marks that render visibly: Arabic number
//     signs U+0600 to U+0605, U+06DD, U+0890, U+0891, U+08E2, Syriac U+070F
//     and Kaithi U+110BD, U+110CD;
//   - the Egyptian hieroglyph (U+13430 to U+1345F), shorthand (U+1BCA0 to
//     U+1BCA3) and musical symbol (U+1D173 to U+1D17A) format controls,
//     which only shape the signs of their own scripts.
func keepFormat(r rune) bool {
	switch {
	case r == 0x200c || r == 0x200d || r == 0x200e || r == 0x200f || r == 0x061c:
		return true
	case r >= 0x0600 && r <= 0x0605, r == 0x06dd, r == 0x070f, r == 0x0890, r == 0x0891, r == 0x08e2:
		return true
	case r == 0x110bd || r == 0x110cd:
		return true
	case r >= 0x13430 && r <= 0x1345f, r >= 0x1bca0 && r <= 0x1bca3, r >= 0x1d173 && r <= 0x1d17a:
		return true
	}
	return false
}

// isNoncharacter reports whether r is one of the 66 Unicode noncharacters:
// U+FDD0 to U+FDEF and the last two code points of every plane.
func isNoncharacter(r rune) bool {
	return (r >= 0xfdd0 && r <= 0xfdef) || r&0xfffe == 0xfffe
}

// blackFlag is U+1F3F4 WAVING BLACK FLAG, the base of the subdivision flag
// emoji (England, Scotland, Wales).
const blackFlag = 0x1f3f4

// flagTagLen returns the byte length of the emoji tag sequence that starts
// s, or 0 when s does not start with one. A subdivision flag is the black
// flag followed by a region code and subdivision written in tag characters
// (tag a to z and tag 0 to 9, 3 to 6 of them, as in "gbeng") and ended by
// U+E007F CANCEL TAG. Anything longer or with other tags is not a flag and
// its tags are dropped one by one.
func flagTagLen(s string) int {
	n := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == 0xe007f:
			if n >= 3 && n <= 6 {
				return i + size
			}
			return 0
		case (r >= 0xe0061 && r <= 0xe007a) || (r >= 0xe0030 && r <= 0xe0039):
			n++
			if n > 6 {
				return 0
			}
		default:
			return 0
		}
		i += size
	}
	return 0
}
