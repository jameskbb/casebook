package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
)

// evidenceFields are the evidence input fields, shared by
// casebook_add_evidence and the checkpoint evidence batch.
type evidenceFields struct {
	Title        string               `json:"title" jsonschema:"short one-line title of the observation (at most 200 bytes)"`
	Summary      string               `json:"summary" jsonschema:"what was observed and why it matters, concise (at most 4000 bytes)"`
	SourceType   app.SourceType       `json:"source_type" jsonschema:"kind of source the observation came from (evernote_note is kept for existing records)"`
	SourceRef    string               `json:"source_ref,omitempty" jsonschema:"exact source a reader can check: path:line, the command run, URL, ticket key or query name"`
	SourceLabel  string               `json:"source_label,omitempty" jsonschema:"short human label for the source"`
	RawDetail    string               `json:"raw_detail,omitempty" jsonschema:"optional long source text such as query output or a code excerpt (at most 64 KiB); stored but not echoed back"`
	CapturedAt   string               `json:"captured_at,omitempty" jsonschema:"RFC3339 time the observation was made; defaults to now"`
	Verification evidenceVerification `json:"verification,omitempty" jsonschema:"defaults to ai_generated; source_verified when you checked it against the source; only a person can set human_verified"`
	Metadata     map[string]any       `json:"metadata,omitempty" jsonschema:"optional small JSON object of structured facts (at most 16384 bytes compact), stored exactly as sent, for example {\"affected_records\": 22}"`

	// metadataText is the metadata exactly as the client sent it, taken
	// from the raw arguments (see exactJSONInput); "" when not known.
	metadataText string
}

// exactJSONInput is a tool input that carries JSON objects to store
// verbatim. The SDK validates arguments by decoding them into map[string]any
// and encoding them again, which turns every number into a float64 (so
// 12345678901234567890 loses digits) and escapes <, > and &. The adapter
// hands such inputs the original arguments so they can take the exact text
// (WR-02).
type exactJSONInput interface {
	setExactJSON(args json.RawMessage) error
}

func (in *addEvidenceInput) setExactJSON(args json.RawMessage) error {
	var raw struct {
		Metadata json.RawMessage `json:"metadata"`
	}
	if err := json.Unmarshal(args, &raw); err != nil {
		return err
	}
	in.metadataText = string(raw.Metadata)
	return nil
}

// metadataJSON returns the metadata as the JSON object text the app expects;
// null or an empty object means none. The text the client sent is kept
// byte for byte (the app only compacts it); without it the decoded map is
// encoded again, without HTML escaping.
func (f evidenceFields) metadataJSON() (string, error) {
	if t := strings.TrimSpace(f.metadataText); t != "" {
		var buf bytes.Buffer
		if t == "null" || (json.Compact(&buf, []byte(t)) == nil && buf.String() == "{}") {
			return "", nil
		}
		return t, nil
	}
	if len(f.Metadata) == 0 {
		return "", nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(f.Metadata); err != nil {
		return "", app.NewValidationError("metadata", "must be a JSON object")
	}
	return strings.TrimSpace(buf.String()), nil
}

// toInput converts the fields to the app input.
func (f evidenceFields) toInput() (app.EvidenceInput, error) {
	meta, err := f.metadataJSON()
	if err != nil {
		return app.EvidenceInput{}, err
	}
	return app.EvidenceInput{
		Title:        f.Title,
		Summary:      f.Summary,
		RawDetail:    f.RawDetail,
		SourceType:   f.SourceType,
		SourceRef:    f.SourceRef,
		SourceLabel:  f.SourceLabel,
		CapturedAt:   f.CapturedAt,
		Verification: app.Verification(f.Verification),
		Metadata:     meta,
	}, nil
}

type addEvidenceInput struct {
	InvestigationRef string `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	evidenceFields
}

const addEvidenceDescription = `Record one concrete observation as evidence, with its source. Use it whenever you find a fact worth keeping: a query result, a code path, a documentation passage, something the user said. ` +
	`source_type says where it came from; source_ref lets a reader check it (path:line, the exact command, the query or URL). ` +
	`Keep summary to the finding itself (counts, the failing value), not a log dump. ` +
	`raw_detail holds long source text (at most 64 KiB) and is not echoed back. ` +
	`Returns the new E ref and a compact record; then link it to claims with casebook_add_claim or casebook_link_evidence_to_claim.`

func init() {
	registerTool("casebook_add_evidence", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_add_evidence",
			Title:       "Add evidence",
			Description: addEvidenceDescription,
			Annotations: mutatingTool(false),
		}, h.addEvidence)
	})
}

func (h *handlers) addEvidence(ctx context.Context, c call, in addEvidenceInput) (mutationResult[evidenceRecord], string, error) {
	var zero mutationResult[evidenceRecord]
	input, err := in.toInput()
	if err != nil {
		return zero, "", err
	}
	ev, err := h.app.AddEvidence(ctx, c.Actor, in.InvestigationRef, input)
	if err != nil {
		return zero, "", err
	}
	out := mutationResult[evidenceRecord]{
		Ref:              ev.Ref,
		InvestigationRef: ev.InvestigationRef,
		Action:           "created",
		Changed:          []string{},
		Record:           evidenceRecordOf(ev),
	}
	return out, fmt.Sprintf("Added evidence %s to %s: %s.", ev.Ref, ev.InvestigationRef, ev.Title), nil
}

// casebook_get_evidence

// Raw detail window sizes for casebook_get_evidence (D-15).
const (
	defaultRawWindowBytes = 16384
	maxRawWindowBytes     = 65536
)

type getEvidenceInput struct {
	InvestigationRef string `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	EvidenceRef      string `json:"evidence_ref" jsonschema:"evidence ref such as E-001 (case-insensitive), or qualified INV-0001/E-001"`
	IncludeRawDetail bool   `json:"include_raw_detail,omitempty" jsonschema:"true returns a window of the stored raw detail; omitted it is not returned"`
	MaxRawBytes      int    `json:"max_raw_bytes,omitempty" jsonschema:"raw detail window size in bytes; 0 or omitted means 16384, at most 65536; shrunk further to fit the 64 KiB response budget"`
	RawDetailOffset  int    `json:"raw_detail_offset,omitempty" jsonschema:"byte offset the raw detail window starts at; pass raw_detail_next_offset from the previous call to read on"`
}

// evidenceView is the full evidence record. The raw detail fields appear
// only when include_raw_detail is true.
type evidenceView struct {
	Ref              string           `json:"ref"`
	InvestigationRef string           `json:"investigation_ref"`
	Title            string           `json:"title"`
	Summary          string           `json:"summary"`
	SourceType       app.SourceType   `json:"source_type"`
	SourceRef        string           `json:"source_ref"`
	SourceLabel      string           `json:"source_label"`
	CapturedAt       string           `json:"captured_at"`
	CreatedAt        string           `json:"created_at"`
	UpdatedAt        string           `json:"updated_at"`
	CreatedBy        app.Actor        `json:"created_by"`
	Verification     app.Verification `json:"verification"`
	// Imported: created_by and verification came from an imported file.
	Imported bool `json:"imported,omitempty"`
	// Metadata is compact JSON object text exactly as stored, or "" when
	// none. A string keeps numbers and characters byte-exact.
	Metadata             string           `json:"metadata" jsonschema:"metadata as compact JSON object text exactly as stored, or empty when none; when truncated_fields lists metadata it is cut and no longer valid JSON"`
	ClaimLinks           []app.RecordLink `json:"claim_links"`
	ClaimLinksTotal      int              `json:"claim_links_total" jsonschema:"number of linked claims; more than len(claim_links) when the list was cut"`
	HypothesisLinks      []app.RecordLink `json:"hypothesis_links"`
	HypothesisLinksTotal int              `json:"hypothesis_links_total" jsonschema:"number of linked hypotheses; more than len(hypothesis_links) when the list was cut"`
	HasRawDetail         bool             `json:"has_raw_detail"`
	// Truncated and TruncatedFields report fields cut to fit the response
	// budget (D-16).
	Truncated       bool     `json:"truncated" jsonschema:"true when a field was cut to fit the 64 KiB response budget"`
	TruncatedFields []string `json:"truncated_fields,omitempty" jsonschema:"fields cut to fit the 64 KiB response budget: summary, source_ref, metadata, claim_links, hypothesis_links"`

	RawDetail           string `json:"raw_detail,omitempty" jsonschema:"the raw detail window (valid UTF-8)"`
	RawDetailBytes      *int   `json:"raw_detail_bytes,omitempty" jsonschema:"total stored raw detail size in bytes"`
	RawDetailOffset     *int   `json:"raw_detail_offset,omitempty" jsonschema:"byte offset the window starts at"`
	RawDetailTruncated  *bool  `json:"raw_detail_truncated,omitempty" jsonschema:"true when more raw detail follows the window"`
	RawDetailNextOffset int    `json:"raw_detail_next_offset,omitempty" jsonschema:"offset to pass as raw_detail_offset to read the next window; present when truncated"`
}

// evidenceCut bounds the variable-size fields of an evidence view. Zero
// means unbounded.
type evidenceCut struct {
	links    int // links shown per list
	text     int // summary and source_ref bytes
	metadata int // metadata bytes
	rawFloor int // smallest raw window tried at this level
}

// evidenceCuts are the shrink levels of casebook_get_evidence (D-16), mildest
// first. Every field the app lets grow is bounded by the last level, so any
// evidence within the app limits fits the response budget there: at most
// 240 bytes each of summary, source_ref and metadata (under 1.5 KiB each
// after JSON escaping, twice: content and structuredContent), 20 links per
// list and a one-rune raw window, next to the 200-byte title and label.
var evidenceCuts = []evidenceCut{
	{rawFloor: 4096},
	{links: 200, metadata: 2000, rawFloor: 4096},
	{links: 50, text: 1000, metadata: 1000, rawFloor: 1024},
	{links: 20, text: textLineBytes, metadata: textLineBytes, rawFloor: 1},
}

// cutText cuts s to at most n bytes at a rune start, ending in "..." when
// cut; n <= 0 keeps s whole. It reports whether s was cut.
func cutText(s string, n int) (string, bool) {
	if n <= 0 || len(s) <= n {
		return s, false
	}
	const ell = "..."
	cut := max(n-len(ell), 0)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + ell, true
}

// cutLinks keeps at most n links (all when n <= 0) and reports whether the
// list was cut. The result is never nil.
func cutLinks(links []app.RecordLink, n int) ([]app.RecordLink, bool) {
	if n > 0 && len(links) > n {
		return append([]app.RecordLink{}, links[:n]...), true
	}
	return append([]app.RecordLink{}, links...), false
}

func evidenceViewOf(e app.Evidence, c evidenceCut) evidenceView {
	v := evidenceView{
		Ref:                  e.Ref,
		InvestigationRef:     e.InvestigationRef,
		Title:                e.Title,
		SourceType:           e.SourceType,
		SourceLabel:          e.SourceLabel,
		CapturedAt:           e.CapturedAt,
		CreatedAt:            e.CreatedAt,
		UpdatedAt:            e.UpdatedAt,
		CreatedBy:            e.CreatedBy,
		Verification:         e.Verification,
		Imported:             e.Imported,
		ClaimLinksTotal:      len(e.ClaimLinks),
		HypothesisLinksTotal: len(e.HypothesisLinks),
		HasRawDetail:         e.HasRawDetail,
	}
	var cut [5]bool
	v.Summary, cut[0] = cutText(e.Summary, c.text)
	v.SourceRef, cut[1] = cutText(e.SourceRef, c.text)
	v.Metadata, cut[2] = cutText(e.Metadata, c.metadata)
	v.ClaimLinks, cut[3] = cutLinks(e.ClaimLinks, c.links)
	v.HypothesisLinks, cut[4] = cutLinks(e.HypothesisLinks, c.links)
	for i, name := range []string{"summary", "source_ref", "metadata", "claim_links", "hypothesis_links"} {
		if cut[i] {
			v.TruncatedFields = append(v.TruncatedFields, name)
		}
	}
	v.Truncated = len(v.TruncatedFields) > 0
	return v
}

// rawWindow returns the byte window [start, end) of raw that starts at
// offset and spans at most maxBytes, with both ends moved back to a rune
// start so the window is valid UTF-8 (raw is valid UTF-8: the engine
// sanitizes it). A window smaller than the rune at start is widened to that
// one rune so paging always advances.
func rawWindow(raw string, offset, maxBytes int) (start, end int) {
	start = offset
	for start > 0 && start < len(raw) && !utf8.RuneStart(raw[start]) {
		start--
	}
	end = start + maxBytes
	if end >= len(raw) {
		return start, len(raw)
	}
	for end > start && !utf8.RuneStart(raw[end]) {
		end--
	}
	if end == start {
		_, n := utf8.DecodeRuneInString(raw[start:])
		end = start + n
	}
	return start, end
}

// setRawWindow fills the raw detail fields of v from raw[start:end].
func (v *evidenceView) setRawWindow(raw string, start, end int) {
	v.RawDetail = raw[start:end]
	v.RawDetailBytes = ptr(len(raw))
	v.RawDetailOffset = ptr(start)
	v.RawDetailTruncated = ptr(end < len(raw))
	v.RawDetailNextOffset = 0
	if end < len(raw) {
		v.RawDetailNextOffset = end
	}
}

const getEvidenceDescription = `Read one piece of evidence in full: title, summary, source type, source ref and label, capture time, who recorded it, verification, metadata (JSON object text), and linked claims and hypotheses (with relation). ` +
	`Raw detail (long source text such as SQL, logs or code) is returned only when include_raw_detail is true, as a window of at most max_raw_bytes bytes (default 16384, max 65536) from raw_detail_offset; when raw_detail_truncated is true, call again with raw_detail_offset set to raw_detail_next_offset. ` +
	`To stay under 64 KiB the raw window shrinks first, then metadata, link lists (claim_links_total and hypothesis_links_total keep the counts), summary and source_ref are cut; truncated_fields names them.`

func init() {
	registerTool("casebook_get_evidence", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_get_evidence",
			Title:       "Get evidence",
			Description: getEvidenceDescription,
			Annotations: readOnlyTool(),
		}, h.getEvidence)
	})
}

func (h *handlers) getEvidence(ctx context.Context, _ call, in getEvidenceInput) (evidenceView, string, error) {
	var zero evidenceView
	maxBytes, err := limitParam("max_raw_bytes", in.MaxRawBytes, defaultRawWindowBytes, maxRawWindowBytes)
	if err != nil {
		return zero, "", err
	}
	if in.RawDetailOffset < 0 {
		return zero, "", app.NewValidationError("raw_detail_offset", "must be 0 or more")
	}
	ev, err := h.app.GetEvidence(ctx, in.InvestigationRef, in.EvidenceRef, app.GetEvidenceOptions{IncludeRawDetail: in.IncludeRawDetail})
	if err != nil {
		return zero, "", err
	}
	raw := ev.RawDetail
	if in.IncludeRawDetail && in.RawDetailOffset > len(raw) {
		return zero, "", app.NewValidationError("raw_detail_offset",
			"must be at most %d (the raw detail size in bytes)", len(raw))
	}
	// Content and structuredContent both carry every field (Claude Code
	// reads content, Codex structuredContent), so the pair must fit the
	// response target (D-16). Each level halves the raw window down to its
	// floor before the next level cuts the fields the raw window cannot
	// page through.
	for _, c := range evidenceCuts {
		window := maxBytes
		for {
			v := evidenceViewOf(ev, c)
			start, end := 0, 0
			if in.IncludeRawDetail {
				start, end = rawWindow(raw, in.RawDetailOffset, window)
				v.setRawWindow(raw, start, end)
			}
			text := renderEvidence(v, in.IncludeRawDetail)
			if resultBytes(text, v) <= responseTarget {
				return v, text, nil
			}
			if !in.IncludeRawDetail || window <= c.rawFloor {
				break
			}
			window = max(min(window, end-start)/2, c.rawFloor)
		}
	}
	// Unreachable for evidence within the app limits (see evidenceCuts).
	return zero, "", fmt.Errorf("casebook: evidence %s is too large to return within the 64 KiB response budget", ev.Ref)
}

// renderEvidence is the plain-text content of casebook_get_evidence.
func renderEvidence(v evidenceView, withRaw bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s in %s: %s\n", v.Ref, v.InvestigationRef, v.Title)
	source := string(v.SourceType)
	if v.SourceLabel != "" {
		source += ", " + v.SourceLabel
	}
	if v.SourceRef != "" {
		source += " (" + v.SourceRef + ")"
	}
	fmt.Fprintf(&b, "Source: %s\n", source)
	fmt.Fprintf(&b, "Verification: %s%s; recorded by %s/%s at %s; captured %s\n",
		v.Verification, importedMark(v.Imported), v.CreatedBy.Kind, v.CreatedBy.Client, v.CreatedAt, v.CapturedAt)
	fmt.Fprintf(&b, "Claims%s: %s\n", shownOf(len(v.ClaimLinks), v.ClaimLinksTotal), recordLinksText(v.ClaimLinks))
	fmt.Fprintf(&b, "Hypotheses%s: %s\n", shownOf(len(v.HypothesisLinks), v.HypothesisLinksTotal), recordLinksText(v.HypothesisLinks))
	fmt.Fprintf(&b, "Summary:\n%s\n", v.Summary)
	if v.Metadata != "" {
		fmt.Fprintf(&b, "Metadata: %s\n", v.Metadata)
	}
	switch {
	case !v.HasRawDetail:
		b.WriteString("Raw detail: none stored.")
	case !withRaw:
		b.WriteString("Raw detail: stored; pass include_raw_detail: true to read it.")
	default:
		end := *v.RawDetailOffset + len(v.RawDetail)
		fmt.Fprintf(&b, "Raw detail (bytes %d to %d of %d):\n%s", *v.RawDetailOffset, end, *v.RawDetailBytes, v.RawDetail)
		if *v.RawDetailTruncated {
			fmt.Fprintf(&b, "\n(raw detail continues; call again with raw_detail_offset %d)", v.RawDetailNextOffset)
		}
	}
	if v.Truncated {
		fmt.Fprintf(&b, "\n(cut to fit the 64 KiB response budget: %s)", strings.Join(v.TruncatedFields, ", "))
	}
	return b.String()
}

// shownOf is " (n of total shown)" when a list was cut, else "".
func shownOf(n, total int) string {
	if n >= total {
		return ""
	}
	return fmt.Sprintf(" (%d of %d shown)", n, total)
}

func recordLinksText(links []app.RecordLink) string {
	if len(links) == 0 {
		return "none"
	}
	parts := make([]string, len(links))
	for i, l := range links {
		parts[i] = l.Ref + " " + string(l.Relation)
	}
	return strings.Join(parts, ", ")
}
