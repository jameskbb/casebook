package contextpack

import (
	"html"
	"strings"
)

// ReportHTML turns the Markdown RenderReport produces into HTML for the
// desktop app's Share summary preview and for saving as a file. It knows
// only the shapes the report emits: "# ", "## " and "### " headings, "- "
// bullets with one nested "  - " level, a leading **bold** run on a bullet,
// a paragraph that is one whole "_..._" line (the footer) as emphasis, and
// paragraphs separated by blank lines, with the backslash mdText puts
// before a leading block marker removed. Every character of stored text is
// escaped, so a claim that contains "<b>" or "## " renders as those
// characters. It is pure and deterministic.
func ReportHTML(markdown string) string {
	var b strings.Builder
	var para []string
	depth := 0 // open <ul> levels: 0, 1 or 2

	flushPara := func() {
		if len(para) == 0 {
			return
		}
		b.WriteString("<p>" + html.EscapeString(strings.Join(para, " ")) + "</p>\n")
		para = nil
	}
	closeLists := func(to int) {
		for depth > to {
			b.WriteString("</li></ul>")
			depth--
		}
		if to == 0 && strings.HasSuffix(b.String(), "</ul>") {
			b.WriteString("\n")
		}
	}
	bullet := func(text string) string {
		if rest, ok := strings.CutPrefix(text, "**"); ok {
			if bold, after, found := strings.Cut(rest, "**"); found {
				return "<strong>" + html.EscapeString(bold) + "</strong>" + html.EscapeString(after)
			}
		}
		return html.EscapeString(text)
	}

	for _, line := range strings.Split(markdown, "\n") {
		switch {
		case strings.HasPrefix(line, "# "):
			flushPara()
			closeLists(0)
			b.WriteString("<h1>" + html.EscapeString(line[2:]) + "</h1>\n")
		case strings.HasPrefix(line, "### "):
			flushPara()
			closeLists(0)
			b.WriteString("<h3>" + html.EscapeString(line[4:]) + "</h3>\n")
		case strings.HasPrefix(line, "## "):
			flushPara()
			closeLists(0)
			b.WriteString("<h2>" + html.EscapeString(line[3:]) + "</h2>\n")
		case strings.HasPrefix(line, "  - "):
			flushPara()
			if depth == 0 {
				b.WriteString("<ul><li>")
				depth = 1
			}
			if depth == 1 {
				b.WriteString("<ul>")
				depth = 2
			} else {
				b.WriteString("</li>")
			}
			b.WriteString("<li>" + bullet(line[4:]))
		case strings.HasPrefix(line, "- "):
			flushPara()
			if depth == 2 {
				b.WriteString("</li></ul></li>")
				depth = 1
			} else if depth == 1 {
				b.WriteString("</li>")
			} else {
				b.WriteString("<ul>")
				depth = 1
			}
			b.WriteString("<li>" + bullet(line[2:]))
		case strings.TrimSpace(line) == "":
			flushPara()
		case len(para) == 0 && len(line) > 2 && line[0] == '_' && line[len(line)-1] == '_':
			closeLists(0)
			b.WriteString("<p><em>" + html.EscapeString(line[1:len(line)-1]) + "</em></p>\n")
		default:
			if depth > 0 {
				closeLists(0)
			}
			para = append(para, unescapeBlockMarker(line))
		}
	}
	flushPara()
	closeLists(0)
	return b.String()
}

// unescapeBlockMarker removes the backslash mdText (report.go) puts before
// a leading Markdown block marker in a paragraph, so the reader sees "#"
// rather than "\#". The text stays escaped literal text in the HTML.
func unescapeBlockMarker(line string) string {
	if len(line) >= 2 && line[0] == '\\' && strings.IndexByte(`#>-+*|`+"`"+`~=_`, line[1]) >= 0 {
		return line[1:]
	}
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(line) && line[i] == '\\' && (line[i+1] == '.' || line[i+1] == ')') {
		return line[:i] + line[i+1:]
	}
	return line
}
