package domain

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeTextCleans(t *testing.T) {
	got, err := SanitizeText("title", "  a\r\nb\x00c\x07  ", 200, true)
	if err != nil || got != "a\nbc" {
		t.Fatalf("SanitizeText = %q, %v; want %q", got, err, "a\nbc")
	}
	got, err = SanitizeText("body", "x\ry\tz\x7f\x1b", 200, false)
	if err != nil || got != "x\ny\tz" {
		t.Fatalf("SanitizeText CR/tab/DEL = %q, %v", got, err)
	}
}

// englandFlag is the subdivision flag emoji: black flag, the tags "gbeng"
// and CANCEL TAG.
const englandFlag = "\U0001F3F4\U000E0067\U000E0062\U000E0065\U000E006E\U000E0067\U000E007F"

// TestSanitizeTextDropsInvisibleControls is S4: C1 controls and the Unicode
// format characters that hide or reorder text are dropped, while the
// joiners, direction marks and emoji sequences ordinary text needs survive.
func TestSanitizeTextDropsInvisibleControls(t *testing.T) {
	cases := []struct{ name, in, want string }{
		// Trojan Source: the stored order differs from the displayed one.
		{"rlo override", "access \u202eniamod\u202c granted", "access niamod granted"},
		{"all embeddings and overrides", "a\u202ab\u202bc\u202cd\u202de\u202ef", "abcdef"},
		{"isolates", "x\u2066y\u2067z\u2068w\u2069v", "xyzwv"},
		{"zero-width space", "pass\u200bword", "password"},
		{"word joiner, bom, soft hyphen", "\ufeffa\u2060b\u00adc\u180ed", "abcd"},
		{"invisible operators and annotation", "f\u2061x\ufff9y\ufffaz\ufffb", "fxyz"},
		{"c1 controls", "a\u0085b\u0080c\u009fd", "abcd"},
		{"noncharacters", "a\ufffeb\uffffc\ufdd0d\U0001FFFE", "abcd"},
		{"ascii smuggling tags", "ok\U000E0069\U000E0067\U000E006E\U000E006F\U000E0072\U000E0065", "ok"},
		{"tags without a flag base", "\U000E0067\U000E0062\U000E0065\U000E006E\U000E0067\U000E007F", ""},
		{"flag tags too long", "\U0001F3F4" + strings.Repeat("\U000E0061", 7) + "\U000E007F", "\U0001F3F4"},
		{"flag tags uppercase", "\U0001F3F4\U000E0047\U000E0042\U000E0045\U000E007F", "\U0001F3F4"},
		// Kept: legitimate text.
		{"zwj family emoji", "family \U0001F468\u200d\U0001F469\u200d\U0001F467", "family \U0001F468\u200d\U0001F469\u200d\U0001F467"},
		{"zwj with vs16", "\U0001F3F3\ufe0f\u200d\U0001F308 flag", "\U0001F3F3\ufe0f\u200d\U0001F308 flag"},
		{"skin tone", "\U0001F44D\U0001F3FD", "\U0001F44D\U0001F3FD"},
		{"country flag", "\U0001F1EF\U0001F1F5", "\U0001F1EF\U0001F1F5"},
		{"subdivision flag", "go " + englandFlag + "!", "go " + englandFlag + "!"},
		{"persian zwnj", "\u0645\u06cc\u200c\u062e\u0648\u0627\u0647\u0645", "\u0645\u06cc\u200c\u062e\u0648\u0627\u0647\u0645"},
		{"hebrew with rlm", "\u05e9\u05dc\u05d5\u05dd 123\u200f.", "\u05e9\u05dc\u05d5\u05dd 123\u200f."},
		{"arabic with lrm and alm", "\u0645\u0631\u062d\u0628\u0627\u200e \u061cabc", "\u0645\u0631\u062d\u0628\u0627\u200e \u061cabc"},
		{"arabic number sign", "\u0600\u0661\u0662", "\u0600\u0661\u0662"},
		{"devanagari conjunct", "\u0915\u094d\u200d\u0937", "\u0915\u094d\u200d\u0937"},
		{"newline and tab", "line one\n\tline two", "line one\n\tline two"},
		{"accents and cjk", "caf\u00e9 \u6f22\u5b57", "caf\u00e9 \u6f22\u5b57"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SanitizeText("body", tc.in, 0, false)
			if err != nil || got != tc.want {
				t.Fatalf("SanitizeText(%+q) = %+q, %v; want %+q", tc.in, got, err, tc.want)
			}
			if again, _ := SanitizeText("body", got, 0, false); again != got {
				t.Fatalf("SanitizeText is not idempotent: %+q then %+q", got, again)
			}
			wantLine := collapseLine(tc.want)
			if line := TruncateLine(tc.in, 0); line != wantLine {
				t.Fatalf("TruncateLine(%+q) = %+q; want %+q", tc.in, line, wantLine)
			}
		})
	}
}

// TestSanitizeTextOnlyCountsKeptBytes checks that dropped characters do not
// count toward the byte limit and cannot satisfy required.
func TestSanitizeTextOnlyCountsKeptBytes(t *testing.T) {
	in := strings.Repeat("a", 10) + strings.Repeat("\u202e", 10)
	if got, err := SanitizeText("title", in, 10, true); err != nil || got != strings.Repeat("a", 10) {
		t.Fatalf("SanitizeText = %q, %v", got, err)
	}
	if _, err := SanitizeText("title", "\u200b\u2066", 10, true); err == nil {
		t.Fatal("a value of only invisible characters passed as required")
	}
}

func TestTruncateLineCutLeavesNoStrayTags(t *testing.T) {
	// The cut falls inside the flag's tag run (each tag is 4 bytes).
	in := strings.Repeat("a", 10) + englandFlag + " tail"
	got := TruncateLine(in, 10+4+4+3)
	if got != strings.Repeat("a", 10)+"\U0001F3F4..." {
		t.Fatalf("TruncateLine = %+q", got)
	}
}

func TestSanitizeTextInvalidUTF8(t *testing.T) {
	_, err := SanitizeText("summary", "ok\xff\xfe", 200, true)
	ve, ok := IsValidation(err)
	if !ok || ve.Field != "summary" || ve.Message != "must be valid UTF-8" {
		t.Fatalf("SanitizeText invalid UTF-8 = %v", err)
	}
}

func TestSanitizeTextRequired(t *testing.T) {
	_, err := SanitizeText("title", "   ", 200, true)
	ve, ok := IsValidation(err)
	if !ok || ve.Field != "title" || ve.Message != "is required" {
		t.Fatalf("required blank = %v", err)
	}
	got, err := SanitizeText("title", " \t\n ", 200, false)
	if err != nil || got != "" {
		t.Fatalf("optional blank = %q, %v", got, err)
	}
}

func TestSanitizeTextByteLimit(t *testing.T) {
	_, err := SanitizeText("title", strings.Repeat("a", 201), MaxTitleBytes, true)
	ve, ok := IsValidation(err)
	if !ok || ve.Message != "must be at most 200 bytes" {
		t.Fatalf("201 bytes = %v", err)
	}
	multi := strings.Repeat("é", 100) // 2 bytes each: exactly 200 bytes
	if got, err := SanitizeText("title", multi, MaxTitleBytes, true); err != nil || got != multi {
		t.Fatalf("200-byte multibyte = %v", err)
	}
	if _, err := SanitizeText("title", multi+"a", MaxTitleBytes, true); err == nil {
		t.Fatal("201-byte multibyte passed")
	}
}

func TestSanitizeLine(t *testing.T) {
	got, err := SanitizeLine("title", " a\nb\tc\r\n\td ", 200, true)
	if err != nil || got != "a b c d" {
		t.Fatalf("SanitizeLine = %q, %v; want %q", got, err, "a b c d")
	}
	if _, err := SanitizeLine("label", "\xff", 200, false); err == nil {
		t.Fatal("SanitizeLine accepted invalid UTF-8")
	}
}

func TestTruncateLine(t *testing.T) {
	if got := TruncateLine("short\nline", 300); got != "short line" {
		t.Fatalf("TruncateLine short = %q", got)
	}
	long := strings.Repeat("é", 400) // 800 bytes
	got := TruncateLine(long, MaxActivitySummaryBytes)
	if len(got) > MaxActivitySummaryBytes || !strings.HasSuffix(got, "...") || !utf8.ValidString(got) {
		t.Fatalf("TruncateLine long = %d bytes, valid=%v, %q", len(got), utf8.ValidString(got), got[len(got)-6:])
	}
}

func TestTextLimits(t *testing.T) {
	cases := map[string][2]int{
		"MaxTitleBytes":               {MaxTitleBytes, 200},
		"MaxProblemStatementBytes":    {MaxProblemStatementBytes, 4000},
		"MaxConclusionBytes":          {MaxConclusionBytes, 4000},
		"MaxConclusionRationaleBytes": {MaxConclusionRationaleBytes, 2000},
		"MaxEvidenceSummaryBytes":     {MaxEvidenceSummaryBytes, 4000},
		"MaxRawDetailBytes":           {MaxRawDetailBytes, 65536},
		"MaxSourceRefBytes":           {MaxSourceRefBytes, 2000},
		"MaxSourceLabelBytes":         {MaxSourceLabelBytes, 200},
		"MaxMetadataBytes":            {MaxMetadataBytes, 16384},
		"MaxStatementBytes":           {MaxStatementBytes, 2000},
		"MaxRationaleBytes":           {MaxRationaleBytes, 2000},
		"MaxQuestionBytes":            {MaxQuestionBytes, 2000},
		"MaxResolutionBytes":          {MaxResolutionBytes, 4000},
		"MaxDeferNoteBytes":           {MaxDeferNoteBytes, 2000},
		"MaxActivitySummaryBytes":     {MaxActivitySummaryBytes, 300},
		"MaxCheckpointNoteBytes":      {MaxCheckpointNoteBytes, 1000},
		"MaxClientBytes":              {MaxClientBytes, 64},
		"MaxSearchQueryBytes":         {MaxSearchQueryBytes, 500},
		"MaxImportBytes":              {MaxImportBytes, 20971520},
	}
	for name, c := range cases {
		if c[0] != c[1] {
			t.Errorf("%s = %d, want %d", name, c[0], c[1])
		}
	}
}
