package domain

import "strings"

// Read-model DTOs: the one-call investigation detail, list summaries and
// search hits every surface shows.

// DetailOptions tunes GetInvestigationDetail. ActivityLimit 0 means 20; it
// must be between 0 and 200. Raw evidence detail is loaded only with
// IncludeRawDetail.
type DetailOptions struct {
	ActivityLimit    int  `json:"activity_limit"`
	IncludeRawDetail bool `json:"include_raw_detail"`
}

// InvestigationDetail is an investigation with every record and its recent
// activity, read from one snapshot. Every slice is non-nil; records are
// ordered by ref and activity newest first.
type InvestigationDetail struct {
	Investigation  Investigation       `json:"investigation"`
	Evidence       []Evidence          `json:"evidence"`
	Claims         []Claim             `json:"claims"`
	Hypotheses     []Hypothesis        `json:"hypotheses"`
	Questions      []Question          `json:"questions"`
	RecentActivity []Activity          `json:"recent_activity"`
	ReviewBases    []RecordReviewBasis `json:"review_bases"`
}

// RecordReviewBasis identifies the exact statement and source snapshot read.
type RecordReviewBasis struct {
	Ref   string `json:"ref"`
	Basis string `json:"basis"`
}

// ListOptions selects investigations for ListInvestigations. Archived
// investigations are hidden unless IncludeArchived is true or Status is
// archived. Status "" means any status. Limit 0 means 100; it must be
// between 0 and 1000.
type ListOptions struct {
	IncludeArchived bool   `json:"include_archived"`
	Status          string `json:"status"`
	Limit           int    `json:"limit"`
}

// RecordCounts counts the records of one investigation.
type RecordCounts struct {
	Evidence      int `json:"evidence"`
	Claims        int `json:"claims"`
	Hypotheses    int `json:"hypotheses"`
	Questions     int `json:"questions"`
	OpenQuestions int `json:"open_questions"`
}

// InvestigationSummary is one row of the investigation list.
type InvestigationSummary struct {
	ID                   int64               `json:"id"`
	Ref                  string              `json:"ref"`
	Title                string              `json:"title"`
	Status               InvestigationStatus `json:"status"`
	ConclusionConfidence Confidence          `json:"conclusion_confidence"`
	CreatedAt            string              `json:"created_at"`
	UpdatedAt            string              `json:"updated_at"`
	ArchivedAt           string              `json:"archived_at"` // "" when not archived
	Archived             bool                `json:"archived"`
	IsActive             bool                `json:"is_active"`
	Counts               RecordCounts        `json:"counts"`
}

// SearchOptions scopes Search. An empty InvestigationRef searches every
// investigation (not the active one). Archived investigations are skipped
// unless IncludeArchived is true or InvestigationRef names one. Limit 0
// means 20; it must be between 0 and 100.
type SearchOptions struct {
	InvestigationRef string `json:"investigation_ref"`
	IncludeArchived  bool   `json:"include_archived"`
	Limit            int    `json:"limit"`
}

// SearchHit is one full-text match. RecordRef is the INV ref for an
// investigation hit. Snippet marks matched terms with ** and elides with
// "..." (for text output); Segments is the same snippet as plain text
// pieces with the matched ones flagged, so a surface that highlights
// matches never parses marker characters out of user text. Rank is the
// bm25 score: lower is better.
type SearchHit struct {
	InvestigationRef   string           `json:"investigation_ref"`
	InvestigationTitle string           `json:"investigation_title"`
	RecordType         RecordType       `json:"record_type"`
	RecordRef          string           `json:"record_ref"`
	Snippet            string           `json:"snippet"`
	Segments           []SnippetSegment `json:"segments"`
	Rank               float64          `json:"rank"`
}

// SnippetSegment is one piece of a search snippet: Match is true for a
// matched term.
type SnippetSegment struct {
	Text  string `json:"text"`
	Match bool   `json:"match"`
}

// Search snippet match markers. They are C0 control characters, which
// SanitizeText and SanitizeLine drop from every stored field, so they can
// never collide with stored text.
const (
	SnippetMatchStart = "\x02"
	SnippetMatchEnd   = "\x03"
)

// ParseSnippet turns a raw snippet marked with SnippetMatchStart and
// SnippetMatchEnd into the display snippet (matches wrapped in **) and its
// segments. Empty pieces are dropped; the segments slice is never nil.
func ParseSnippet(raw string) (string, []SnippetSegment) {
	segs := []SnippetSegment{}
	var display strings.Builder
	match := false
	for raw != "" {
		marker := SnippetMatchStart
		if match {
			marker = SnippetMatchEnd
		}
		text, rest, found := strings.Cut(raw, marker)
		// A stray marker of the other kind is dropped, never shown.
		text = strings.NewReplacer(SnippetMatchStart, "", SnippetMatchEnd, "").Replace(text)
		if text != "" {
			if n := len(segs); n > 0 && segs[n-1].Match == match {
				segs[n-1].Text += text
			} else {
				segs = append(segs, SnippetSegment{Text: text, Match: match})
			}
			if match {
				display.WriteString("**" + text + "**")
			} else {
				display.WriteString(text)
			}
		}
		if !found {
			break
		}
		raw = rest
		match = !match
	}
	return display.String(), segs
}
