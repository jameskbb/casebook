package contextpack

import (
	"html"
	"strings"
)

// ReportHTML turns the Markdown of a shared answer into HTML for the
// desktop preview and, inside AnswerPage, for a saved file. It knows
// only the shapes a shared answer has: "# ", "## " and "### " headings, "- "
// bullets with one nested "  - " level,
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
			b.WriteString("<li>" + html.EscapeString(line[4:]))
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
			b.WriteString("<li>" + html.EscapeString(line[2:]))
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

// AnswerPage wraps the HTML of a shared answer (ReportHTML) in a whole page,
// so the saved file opens in a browser on a double-click and reads well when
// printed or pasted into a document. The title is escaped; nothing is loaded
// from the network.
func AnswerPage(title, body string) string {
	return "<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n" +
		"<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n" +
		"<title>" + html.EscapeString(title) + "</title>\n<style>\n" + answerPageCSS + "</style>\n</head>\n<body>\n" +
		body + "</body>\n</html>\n"
}

const answerPageCSS = `body { font: 16px/1.5 "Segoe UI", system-ui, sans-serif; color: #1f2328; max-width: 44em; margin: 2em auto; padding: 0 1.25em; }
h1 { font-size: 1.6em; line-height: 1.25; }
h2 { font-size: 1.15em; margin-top: 1.8em; padding-bottom: 0.2em; border-bottom: 1px solid #d8dee4; }
h3 { font-size: 1em; margin-top: 1.4em; }
li { margin: 0.3em 0; }
li li { color: #474d54; }
em { color: #59636e; font-size: 0.9em; }
`
