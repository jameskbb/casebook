package contextpack

import (
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// timelineVerbs holds the plain phrase for each activity action, the same
// table the desktop History view uses (desktop/frontend/src/lib/activity.ts,
// VERBS), capitalised to start a line. "{T}" is the target ref. It is only
// ever looked up by key, never iterated, so output order cannot depend on it.
var timelineVerbs = map[string]string{
	"investigation.create":    "Created this case",
	"investigation.update":    "Edited the case details",
	"investigation.archive":   "Archived this case",
	"investigation.unarchive": "Unarchived this case",
	"conclusion.update":       "Updated the conclusion",
	"evidence.add":            "Added evidence {T}",
	"evidence.update":         "Updated evidence {T}",
	"claim.add":               "Added claim {T}",
	"claim.update":            "Updated claim {T}",
	"claim.supersede":         "Superseded claim {T}",
	"hypothesis.add":          "Added hypothesis {T}",
	"hypothesis.update":       "Updated hypothesis {T}",
	"hypothesis.supersede":    "Superseded hypothesis {T}",
	"link.add":                "Linked evidence to {T}",
	"link.update":             "Changed how evidence relates to {T}",
	"link.remove":             "Unlinked evidence from {T}",
	"question.add":            "Asked {T}",
	"question.update":         "Reworded {T}",
	"question.resolve":        "Resolved {T}",
	"question.defer":          "Deferred {T}",
	"question.reopen":         "Reopened {T}",
	"investigation.delete":    "Deleted case {T}",
	"evidence.delete":         "Deleted evidence {T}",
	"claim.delete":            "Deleted claim {T}",
	"hypothesis.delete":       "Deleted hypothesis {T}",
	"question.delete":         "Deleted {T}",
	"active.set":              "Made this your AI's current case",
	"active.clear":            "Cleared your AI's current case",
	"checkpoint":              "Saved a checkpoint",
	"import":                  "Imported this case",
	"restore":                 "Restored all data from a backup",
}

// timelinePhrase renders one activity row for the report timeline in plain
// words, from its action key and target ref: "Created this case", "Marked
// C-001 Human verified", "Ruled out H-001". When the stored summary has the
// shape internal/app writes for that action, the detail it carries (the new
// status, the question text, the linked evidence) is kept, with stored enum
// values written as their labels. An action the table does not know falls
// back to the stored summary.
func timelinePhrase(a domain.Activity) string {
	sum := oneLine(a.Summary)
	ref := oneLine(a.TargetRef)
	base, ok := timelineVerbs[a.Action]
	if !ok {
		if sum == "" {
			return "Recorded " + oneLine(a.Action)
		}
		return sum
	}
	if p := detailPhrase(a.Action, ref, sum); p != "" {
		return p
	}
	if ref == "" {
		return strings.Replace(base, " {T}", "", 1)
	}
	return strings.Replace(base, "{T}", ref, 1)
}

// detailPhrase reads the detail out of a summary written by internal/app
// for action. It returns "" when the summary does not have the expected
// shape (an older row, an imported file), and the caller uses the table.
func detailPhrase(action, ref, sum string) string {
	switch action {
	case "investigation.update":
		return caseUpdatePhrase(ref, sum)
	case "conclusion.update":
		if strings.HasPrefix(sum, "Cleared conclusion") {
			return "Cleared the conclusion"
		}
		if c, ok := cutAffixes(sum, "Updated conclusion of "+ref+" (confidence ", ")"); ok && ref != "" {
			if c == string(domain.ConfidenceUnknown) {
				return "Updated the conclusion (confidence not assessed)"
			}
			return "Updated the conclusion (" + confidenceLabel(c) + " confidence)"
		}
	case "evidence.add":
		if title, ok := strings.CutPrefix(sum, "Added evidence "+ref+" "); ok && ref != "" {
			return "Added evidence " + ref + " " + title
		}
	case "claim.add", "hypothesis.add":
		return addStatementPhrase(recName(action), ref, sum)
	case "claim.update", "hypothesis.update", "evidence.update":
		return updateRecordPhrase(recName(action), ref, sum)
	case "claim.supersede", "hypothesis.supersede":
		head, _, _ := strings.Cut(sum, "; ")
		if by, ok := strings.CutPrefix(head, ref+" superseded by "); ok && ref != "" && by != "" {
			return "Superseded " + recName(action) + " " + ref + " (replaced by " + by + ")"
		}
	case "link.add":
		// "Linked E-003 to H-001 (contradicts)"
		if rest, ok := strings.CutPrefix(sum, "Linked "); ok {
			ev, rest, ok1 := strings.Cut(rest, " to ")
			to, rel, ok2 := strings.Cut(rest, " (")
			rel, ok3 := strings.CutSuffix(rel, ")")
			if ok1 && ok2 && ok3 && ev != "" && to != "" {
				switch rel {
				case string(domain.RelationSupports):
					return "Linked evidence " + ev + " to " + to + ", which it supports"
				case string(domain.RelationContradicts):
					return "Linked evidence " + ev + " to " + to + ", which it contradicts"
				}
			}
		}
	case "link.update":
		// "Changed link E-003 to H-001 from supports to contradicts"
		if rest, ok := strings.CutPrefix(sum, "Changed link "); ok {
			ev, rest, ok1 := strings.Cut(rest, " to ")
			to, rest, ok2 := strings.Cut(rest, " from ")
			from, now, ok3 := strings.Cut(rest, " to ")
			if ok1 && ok2 && ok3 && ev != "" && to != "" && relationGerund(from) != "" && relationGerund(now) != "" {
				return "Changed evidence " + ev + " from " + relationGerund(from) + " to " +
					relationGerund(now) + " " + to
			}
		}
	case "link.remove":
		if rest, ok := strings.CutPrefix(sum, "Unlinked "); ok {
			if ev, from, ok := strings.Cut(rest, " from "); ok && ev != "" && from != "" {
				return "Unlinked evidence " + ev + " from " + from
			}
		}
	case "question.add":
		return questionPhrase("Asked", "Added question ", ref, sum)
	case "question.update":
		return questionPhrase("Reworded", "Updated question ", ref, sum)
	case "question.resolve":
		return questionPhrase("Resolved", "Resolved question ", ref, sum)
	case "question.defer":
		return questionPhrase("Deferred", "Deferred question ", ref, sum)
	}
	return ""
}

// recName is the record word of a "claim.update"-style action.
func recName(action string) string {
	rec, _, _ := strings.Cut(action, ".")
	return rec
}

// cutAffixes returns s without prefix and suffix, and whether it had both.
func cutAffixes(s, prefix, suffix string) (string, bool) {
	rest, ok := strings.CutPrefix(s, prefix)
	if !ok {
		return "", false
	}
	return strings.CutSuffix(rest, suffix)
}

// questionPhrase turns "Added question Q-001: text" into "Asked Q-001: text".
func questionPhrase(verb, prefix, ref, sum string) string {
	rest, ok := strings.CutPrefix(sum, prefix+ref)
	if !ok || ref == "" {
		return ""
	}
	if rest == "" {
		return verb + " " + ref
	}
	if text, ok := strings.CutPrefix(rest, ": "); ok && text != "" {
		return verb + " " + ref + ": " + text
	}
	return ""
}

// addStatementPhrase turns "Added claim C-001 [high] with 3 evidence links"
// into "Added claim C-001 (High confidence, 3 evidence links)".
func addStatementPhrase(rec, ref, sum string) string {
	rest, ok := strings.CutPrefix(sum, "Added "+rec+" "+ref+" [")
	if !ok || ref == "" {
		return ""
	}
	conf, links, ok := strings.Cut(rest, "]")
	if !ok {
		return ""
	}
	detail := confidenceLabel(conf) + " confidence"
	if conf == string(domain.ConfidenceUnknown) {
		detail = "confidence not assessed"
	}
	if links != "" {
		n, ok := strings.CutPrefix(links, " with ")
		if !ok {
			return ""
		}
		detail += ", " + n
	}
	return "Added " + rec + " " + ref + " (" + detail + ")"
}

// updateRecordPhrase renders a claim, hypothesis or evidence update:
// "Set verification of C-001 to human_verified" becomes "Marked C-001 Human
// verified"; "Updated hypothesis H-001: rationale, status disproven" becomes
// "Ruled out H-001; edited the rationale".
func updateRecordPhrase(rec, ref, sum string) string {
	if ref == "" {
		return ""
	}
	if rest, ok := strings.CutPrefix(sum, "Set verification of "+ref+" to "); ok {
		v, status, hasStatus := strings.Cut(rest, "; status ")
		if v == "" {
			return ""
		}
		out := "Marked " + ref + " " + domain.Verification(v).Label()
		if hasStatus && statusLabel(rec, status) != domain.Verification(v).Label() {
			out += "; status now " + statusLabel(rec, status)
		}
		return out
	}
	rest, ok := strings.CutPrefix(sum, "Updated "+rec+" "+ref+": ")
	if !ok {
		return ""
	}
	list, note, hasNote := strings.Cut(rest, "; ")
	var primary string
	var others []string
	for _, item := range strings.Split(list, ", ") {
		field, value, hasValue := strings.Cut(item, " ")
		switch {
		case field == "":
			return ""
		case field == "status" && hasValue:
			if rec == "hypothesis" && value == string(domain.HypothesisStatusDisproven) {
				primary = "Ruled out " + ref
			} else {
				primary = "Marked " + ref + " " + statusLabel(rec, value)
			}
		case field == "verification" && !hasValue && hasNote:
			// a reset: the note below names the new value
		case field == "verification" && hasValue && primary == "":
			primary = "Marked " + ref + " " + domain.Verification(value).Label()
		case hasValue:
			others = append(others, changePhrase(rec, field, value))
		default:
			others = append(others, "edited the "+fieldName(field))
		}
	}
	if hasNote {
		if !strings.HasPrefix(note, "verification reset to ") {
			return ""
		}
		others = append(others, "verification reset to "+domain.VerificationAIGenerated.Label()+" after an edit")
	}
	switch {
	case primary == "" && len(others) == 0:
		return ""
	case primary == "":
		return "Updated " + rec + " " + ref + ": " + strings.Join(others, ", ")
	case len(others) == 0:
		return primary
	}
	return primary + "; " + strings.Join(others, ", ")
}

// caseUpdatePhrase renders "Updated investigation INV-0002: title, status
// (status resolved)" as "Marked the case Resolved; edited the title".
func caseUpdatePhrase(ref, sum string) string {
	rest, ok := strings.CutPrefix(sum, "Updated investigation "+ref+": ")
	if !ok || ref == "" {
		return ""
	}
	status, unarchived := "", false
	if list, st, ok := strings.Cut(rest, " (status "); ok {
		rest, status = list, strings.TrimSuffix(st, ")")
	} else if list, st, ok := strings.Cut(rest, " (unarchived to "); ok {
		rest, status, unarchived = list, strings.TrimSuffix(st, ")"), true
	}
	var edits []string
	for _, field := range strings.Split(rest, ", ") {
		switch field {
		case "status":
		case "title":
			edits = append(edits, "the title")
		case "problem_statement":
			edits = append(edits, "the question")
		default:
			return ""
		}
	}
	var parts []string
	switch {
	case unarchived:
		parts = append(parts, "Unarchived this case as "+domain.InvestigationStatus(status).Label())
	case status != "":
		parts = append(parts, "Marked the case "+domain.InvestigationStatus(status).Label())
	}
	if len(edits) > 0 {
		verb := "Edited "
		if len(parts) > 0 {
			verb = "edited "
		}
		parts = append(parts, verb+strings.Join(edits, " and "))
	}
	return strings.Join(parts, "; ")
}

// changePhrase renders an enum change other than the headline one:
// "confidence now High", "source now SQL query".
func changePhrase(rec, field, value string) string {
	switch field {
	case "confidence":
		return "confidence now " + confidenceLabel(value)
	case "verification":
		return "verification now " + domain.Verification(value).Label()
	case "source_type":
		return "source now " + domain.SourceType(value).Label()
	case "status":
		return "status now " + statusLabel(rec, value)
	}
	return fieldName(field) + " now " + value
}

// fieldName is the plain name of a stored field: "confidence_rationale"
// reads "rationale", "raw_detail" reads "raw detail".
func fieldName(field string) string {
	switch field {
	case "confidence_rationale":
		return "rationale"
	case "superseded_by":
		return "replacement"
	case "problem_statement":
		return "question"
	}
	return strings.ReplaceAll(field, "_", " ")
}

// statusLabel is the label of a stored status for the record kind, matching
// the desktop labels (lib/labels.ts): a claim's "active" reads "Current".
func statusLabel(rec, value string) string {
	switch rec {
	case "claim":
		if value == string(domain.ClaimStatusActive) {
			return "Current"
		}
		return domain.ClaimStatus(value).Label()
	case "hypothesis":
		return domain.HypothesisStatus(value).Label()
	case "question":
		return domain.QuestionStatus(value).Label()
	case "investigation":
		return domain.InvestigationStatus(value).Label()
	}
	return value
}

// confidenceLabel matches the desktop labels: "unknown" reads "Not assessed".
func confidenceLabel(value string) string {
	if value == string(domain.ConfidenceUnknown) {
		return "Not assessed"
	}
	return domain.Confidence(value).Label()
}

// relationGerund is "supporting" or "contradicting", or "" for anything else.
func relationGerund(rel string) string {
	switch rel {
	case string(domain.RelationSupports):
		return "supporting"
	case string(domain.RelationContradicts):
		return "contradicting"
	}
	return ""
}
