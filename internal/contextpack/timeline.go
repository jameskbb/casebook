package contextpack

import (
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// timelineVerbs holds the plain phrase for each activity action, the same
// table the desktop "How we got here" list uses
// (desktop/frontend/src/lib/activity.ts, VERBS), capitalised to start a line.
// "{T}" is the target ref. It is only ever looked up by key, never iterated,
// so output order cannot depend on it.
var timelineVerbs = map[string]string{
	"investigation.create":    "Created this question",
	"investigation.update":    "Edited the question",
	"investigation.archive":   "Archived this question",
	"investigation.unarchive": "Unarchived this question",
	"conclusion.update":       "Updated the answer",
	"evidence.add":            "Added source {T}",
	"evidence.update":         "Updated source {T}",
	"claim.add":               "Added finding {T}",
	"claim.update":            "Updated finding {T}",
	"claim.supersede":         "Replaced finding {T}",
	"hypothesis.add":          "Added finding {T}",
	"hypothesis.update":       "Updated finding {T}",
	"hypothesis.supersede":    "Replaced finding {T}",
	"link.add":                "Linked a source to {T}",
	"link.update":             "Changed how a source relates to {T}",
	"link.remove":             "Unlinked a source from {T}",
	"question.add":            "Asked {T}",
	"question.update":         "Reworded {T}",
	"question.resolve":        "Resolved {T}",
	"question.defer":          "Deferred {T}",
	"question.reopen":         "Reopened {T}",
	"investigation.delete":    "Deleted question {T}",
	"evidence.delete":         "Deleted source {T}",
	"claim.delete":            "Deleted finding {T}",
	"hypothesis.delete":       "Deleted finding {T}",
	"question.delete":         "Deleted {T}",
	"active.set":              "Made this your AI's current question",
	"active.clear":            "Cleared your AI's current question",
	"checkpoint":              "Saved a checkpoint",
	"import":                  "Imported this question",
	"restore":                 "Restored all data from a backup",
}

// timelinePhrase renders one activity row for the report's "How we got
// here" list in plain words, from its action key and target ref: "Created
// this question", "Marked C-001: You checked this", "Ruled out H-001". When
// the stored summary has a shape internal/app writes for that action (today's
// or an older version's), the detail it carries (the new status, the
// question text, the linked source) is kept, with stored enum values written
// as the labels a person reads. Confidence is never shown. An action the
// table does not know falls back to the stored summary. label writes a check
// label: verificationLabel for a person, packLabel for an AI client.
func timelinePhrase(a domain.Activity, label func(domain.Verification) string) string {
	sum := oneLine(a.Summary)
	ref := oneLine(a.TargetRef)
	base, ok := timelineVerbs[a.Action]
	if !ok {
		if sum == "" {
			return "Recorded " + oneLine(a.Action)
		}
		return sum
	}
	if p := detailPhrase(a.Action, ref, sum, label); p != "" {
		return p
	}
	if ref == "" {
		return strings.Replace(base, " {T}", "", 1)
	}
	return strings.Replace(base, "{T}", ref, 1)
}

// resetSentence says an edit lowered a checked record to Not checked.
const resetSentence = "It changed after it was checked, so it reads Not checked again."

// detailPhrase reads the detail out of a summary written by internal/app
// for action. It returns "" when the summary does not have an expected
// shape (an imported file, a hand-written row), and the caller uses the
// table.
func detailPhrase(action, ref, sum string, label func(domain.Verification) string) string {
	switch action {
	case "investigation.update":
		return questionUpdatePhrase(ref, sum)
	case "conclusion.update":
		if strings.HasPrefix(sum, "Cleared conclusion") || strings.HasPrefix(sum, "Cleared the answer") {
			return "Cleared the answer"
		}
		if strings.HasPrefix(sum, "Recorded a review of the answer") {
			return "Recorded a review of the answer"
		}
		if strings.Contains(sum, "; verification reset to ") {
			return "Updated the answer. " + resetSentence
		}
	case "evidence.add":
		for _, prefix := range []string{"Added source ", "Added evidence "} {
			if title, ok := strings.CutPrefix(sum, prefix+ref+" "); ok && ref != "" {
				return "Added source " + ref + " " + title
			}
		}
	case "claim.add", "hypothesis.add":
		return addFindingPhrase(recName(action), ref, sum)
	case "claim.update", "hypothesis.update", "evidence.update":
		return updateRecordPhrase(recName(action), ref, sum, label)
	case "claim.supersede", "hypothesis.supersede":
		head, _, _ := strings.Cut(sum, "; ")
		for _, mid := range []string{" was replaced by ", " superseded by "} {
			if by, ok := strings.CutPrefix(head, ref+mid); ok && ref != "" && by != "" {
				return "Replaced finding " + ref + " with " + by
			}
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
					return "Linked source " + ev + " to " + to + ", which it supports"
				case string(domain.RelationContradicts):
					return "Linked source " + ev + " to " + to + ", which it contradicts"
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
				return "Changed source " + ev + " from " + relationGerund(from) + " to " +
					relationGerund(now) + " " + to
			}
		}
	case "link.remove":
		if rest, ok := strings.CutPrefix(sum, "Unlinked "); ok {
			if ev, from, ok := strings.Cut(rest, " from "); ok && ev != "" && from != "" {
				return "Unlinked source " + ev + " from " + from
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

// addFindingPhrase turns "Added finding C-001 with 3 sources", or the older
// "Added claim C-001 [high] with 3 evidence links", into "Added finding
// C-001 (3 sources)".
func addFindingPhrase(rec, ref, sum string) string {
	rest, ok := strings.CutPrefix(sum, "Added finding "+ref)
	if !ok {
		if rest, ok = strings.CutPrefix(sum, "Added "+rec+" "+ref+" ["); ok {
			_, rest, ok = strings.Cut(rest, "]")
		}
	}
	if !ok || ref == "" {
		return ""
	}
	if rest == "" {
		return "Added finding " + ref
	}
	links, ok := strings.CutPrefix(rest, " with ")
	n, _, found := strings.Cut(links, " ")
	if !ok || !found {
		return ""
	}
	if n == "1" {
		return "Added finding " + ref + " (1 source)"
	}
	return "Added finding " + ref + " (" + n + " sources)"
}

// updateRecordPhrase renders a claim, hypothesis or evidence update. These
// summaries keep the engine's field list: "Set verification of C-001 to
// human_verified" becomes "Marked C-001: You checked this"; "Updated
// hypothesis H-001: rationale, status disproven" becomes "Ruled out H-001;
// edited the why".
func updateRecordPhrase(rec, ref, sum string, check func(domain.Verification) string) string {
	if ref == "" {
		return ""
	}
	if rest, ok := strings.CutPrefix(sum, "Set verification of "+ref+" to "); ok {
		v, status, hasStatus := strings.Cut(rest, "; status ")
		if v == "" {
			return ""
		}
		label := check(domain.Verification(v))
		out := "Marked " + ref + ": " + label
		if s := statusLabel(rec, status); hasStatus && s != label && s != "Current" {
			out += "; now " + s
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
		case field == "confidence":
			// stored, never shown
		case field == "status" && hasValue:
			if s := statusLabel(rec, value); s == "Ruled out" {
				primary = "Ruled out " + ref
			} else {
				primary = "Marked " + ref + ": " + s
			}
		case field == "verification" && !hasValue && hasNote:
			// a reset: the note below names the new value
		case field == "verification" && hasValue && primary == "":
			primary = "Marked " + ref + ": " + check(domain.Verification(value))
		case field == "verification" && hasValue:
			others = append(others, "now marked "+check(domain.Verification(value)))
		case field == "source_type" && hasValue:
			others = append(others, "source type now "+domain.SourceType(value).Label())
		case hasValue:
			others = append(others, fieldName(field)+" now "+value)
		default:
			others = append(others, "edited the "+fieldName(field))
		}
	}
	noun := domain.RecordType(rec).PlainWord()
	out := ""
	switch {
	case primary == "" && len(others) == 0:
		out = "Updated " + noun + " " + ref
	case primary == "":
		out = "Updated " + noun + " " + ref + ": " + strings.Join(others, ", ")
	case len(others) == 0:
		out = primary
	default:
		out = primary + "; " + strings.Join(others, ", ")
	}
	if hasNote {
		if !strings.HasPrefix(note, "verification reset to ") {
			return ""
		}
		out += ". " + resetSentence
	}
	return out
}

// questionUpdatePhrase renders "Updated question INV-0002: title, status
// (status resolved)", or the older "Updated investigation ...", as "Marked
// the question Resolved; edited the title".
func questionUpdatePhrase(ref, sum string) string {
	rest, ok := strings.CutPrefix(sum, "Updated question "+ref+": ")
	if !ok {
		rest, ok = strings.CutPrefix(sum, "Updated investigation "+ref+": ")
	}
	if !ok || ref == "" {
		return ""
	}
	status, unarchived := "", false
	if list, st, ok := strings.Cut(rest, " (status "); ok {
		rest, status = list, strings.TrimSuffix(st, ")")
	} else if list, _, ok := strings.Cut(rest, " (unarchived to "); ok {
		rest, unarchived = list, true
	}
	var edits []string
	for _, field := range strings.Split(rest, ", ") {
		switch field {
		case "status":
		case "title":
			edits = append(edits, "the title")
		case "question text", "problem_statement":
			edits = append(edits, "the question")
		default:
			return ""
		}
	}
	var parts []string
	switch {
	case unarchived:
		parts = append(parts, "Unarchived this question")
	case status != "":
		parts = append(parts, "Marked the question "+questionStatusWord(domain.InvestigationStatus(status)))
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

// questionStatusWord is the stored status of a question (an investigation)
// in plain words.
func questionStatusWord(s domain.InvestigationStatus) string {
	if s == domain.InvestigationStatusInvestigating {
		return "In progress"
	}
	return s.Label()
}

// fieldName is the plain name of a stored field: "confidence_rationale"
// reads "why", "raw_detail" reads "raw detail".
func fieldName(field string) string {
	switch field {
	case "confidence_rationale", "rationale":
		return "why"
	case "superseded_by":
		return "replacement"
	}
	return strings.ReplaceAll(field, "_", " ")
}

// statusLabel is the label a person reads for a stored status of the record
// kind, matching the desktop labels (lib/labels.ts).
func statusLabel(rec, value string) string {
	switch rec {
	case "claim":
		return domain.ClaimStatus(value).PlainLabel()
	case "hypothesis":
		return domain.HypothesisStatus(value).PlainLabel()
	}
	return value
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
