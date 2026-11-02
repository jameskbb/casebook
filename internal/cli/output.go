package cli

import (
	"encoding/json"
	"io"
	"text/tabwriter"
)

// writeJSON writes v as indented JSON (two spaces, no HTML escaping)
// followed by a newline.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// newTabWriter returns the aligned plain-text writer every human output uses.
// Callers must Flush it.
func newTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
}
