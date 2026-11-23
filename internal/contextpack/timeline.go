package contextpack

import (
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// timelineVerbs holds the plain phrase for each activity action, the same
// table the desktop "How we got here" list uses
// (desktop/frontend/src/lib/activity.ts, VERBS). A phrase follows the actor:
// "Claude Code added the source E-005". "{T}" is the target, written by a
// refNamer. The table is only ever looked up by key, never iterated, so
// output order cannot depend on it.
var timelineVerbs = map[string]string{
	"investigation.create":    "created this question",
	"investigation.update":    "edited the question",
	"investigation.archive":   "archived this question",
	"investigation.unarchive": "put this question back in the list",
	"conclusion.update":       "updated the answer",
	"evidence.add":            "added the source {T}",
	"evidence.update":         "updated the source {T}",
	"claim.add":               "added the finding {T}",
	"claim.update":            "updated the finding {T}",
	"claim.supersede":         "replaced the finding {T}",
	"hypothesis.add":          "added the finding {T}",
	"hypothesis.update":       "updated the finding {T}",
	"hypothesis.supersede":    "replaced the finding {T}",
	"link.add":                "added a source to the finding {T}",
	"link.update":             "changed whether a source backs the finding {T}",
	"link.remove":             "removed a source from the finding {T}",
	"question.add":            "added something to find out: {T}",
	"question.update":         "reworded {T}",
	"question.resolve":        "answered {T}",
	"question.defer":          "left {T} for later",
	"question.reopen":         "opened {T} again",
	"investigation.delete":    "deleted a question {T}",
	"evidence.delete":         "deleted a source {T}",
	"claim.delete":            "deleted a finding {T}",
	"hypothesis.delete":       "deleted a finding {T}",
	"question.delete":         "deleted something to find out {T}",
	"active.set":              "made this your AI's current question",
	"active.clear":            "cleared your AI's current question",
	"checkpoint":              "saved its progress",
	"import":                  "added this question from a file",
	"restore":                 "restored all data from a backup",
}

// refNamer writes a record ref the way the reader meets the record: the ref
// itself in a terminal or a pack an AI reads, the record's own text in a
// shared answer (gone when the record has been deleted, "" for no ref).
type refNamer func(ref string) string

func refItself(ref string) string { return ref }

// gone is the name of a deleted record. It is never printed: with and fill
// write "a finding that is no longer here" for it, so a line has no hole.
const gone = "\x00"

// with joins a noun and a name: "the source E-005", "the source" when the
// name is empty, "a source that is no longer here" when the record is gone.
func with(noun, name string) string {
	switch name {
	case "":
		return noun
	case gone:
		return "a " + strings.TrimPrefix(noun, "the ") + " that is no longer here"
	}
	return noun + " " + name
}

// fill puts the target's name into a table phrase. Without a name the
// phrase still reads whole: "added the finding", "deleted a source", "added
// a finding that is no longer here" for a deleted one, and "answered
// something to find out" where the name was the only noun.
func fill(phrase, name string) string {
	if name != "" && name != gone {
		return strings.Replace(phrase, "{T}", name, 1)
	}
	before, after, ok := strings.Cut(phrase, "{T}")
	if !ok {
		return phrase
	}
	before = strings.TrimRight(before, " ")
	if head, ok := strings.CutSuffix(before, ":"); ok {
		return head + after
	}
	for _, noun := range []string{"finding", "source", "question", "find out"} {
		if head, ok := strings.CutSuffix(before, "the "+noun); ok && name == gone {
			return head + with("the "+noun, gone) + after
		}
		if strings.HasSuffix(before, noun) {
			return before + after
		}
	}
	return before + " something to find out" + after
}

// timelinePhrase renders one activity row in plain words, from its action
// key and target ref: "created this question", "checked the finding C-001",
// "ruled out the finding H-001". When the stored summary has a shape
// internal/app writes for that action (today's or an older version's), the
// detail it carries (the new state, the answer to a follow-up question, the
// source added to a finding) is kept, with stored enum values written as the
// labels a person reads. Confidence is never shown. An action the table does
// not know falls back to the stored summary.
func timelinePhrase(a domain.Activity, name refNamer) string {
	sum := oneLine(a.Summary)
	ref := oneLine(a.TargetRef)
	base, ok := timelineVerbs[a.Action]
	if !ok {
		if sum == "" {
			return "made a change"
		}
		return sum
	}
	if p := detailPhrase(a.Action, ref, sum, name); p != "" {
		return p
	}
	return fill(base, name(ref))
}

// resetSentence says an edit lowered a checked record to Not checked.
const resetSentence = "It changed after it was checked, so it needs checking again."

// detailPhrase reads the detail out of a summary written by internal/app
// for action. It returns "" when the summary does not have an expected
// shape (an imported file, a hand-written row), and the caller uses the
// table.
func detailPhrase(action, ref, sum string, name refNamer) string {
	switch action {
	case "investigation.update":
		return questionUpdatePhrase(ref, sum)
	case "conclusion.update":
		if strings.HasPrefix(sum, "Cleared conclusion") || strings.HasPrefix(sum, "Cleared the answer") {
			return "cleared the answer"
		}
		if strings.HasPrefix(sum, "Recorded a review of the answer") {
			return "checked the answer"
		}
		if strings.Contains(sum, "; verification reset to ") {
			return "updated the answer. " + resetSentence
		}
	case "evidence.add":
		// The stored title names a source whose code is all the reader has.
		for _, prefix := range []string{"Added source ", "Added evidence "} {
			if title, ok := strings.CutPrefix(sum, prefix+ref+" "); ok && ref != "" && name(ref) == ref {
				return "added the source " + ref + " " + title
			}
		}
	case "claim.add", "hypothesis.add":
		return addFindingPhrase(recName(action), ref, sum, name)
	case "claim.update", "hypothesis.update", "evidence.update":
		return updateRecordPhrase(recName(action), ref, sum, name)
	case "claim.supersede", "hypothesis.supersede":
		head, _, _ := strings.Cut(sum, "; ")
		for _, mid := range []string{" was replaced by ", " superseded by "} {
			if by, ok := strings.CutPrefix(head, ref+mid); ok && ref != "" && name(by) != "" && name(by) != gone {
				return "replaced " + with("the finding", name(ref)) + " with " + name(by)
			}
		}
	case "link.add":
		// "Linked E-003 to H-001 (contradicts)"
		if rest, ok := strings.CutPrefix(sum, "Linked "); ok {
			ev, rest, ok1 := strings.Cut(rest, " to ")
			to, rel, ok2 := strings.Cut(rest, " (")
			rel, ok3 := strings.CutSuffix(rel, ")")
			if ok1 && ok2 && ok3 && ev != "" && to != "" && relationWords(rel) != "" {
				return "added " + with("the source", name(ev)) + " to " + with("the finding", name(to)) + ", which it " + relationWords(rel)
			}
		}
	case "link.update":
		// "Changed link E-003 to H-001 from supports to contradicts"
		if rest, ok := strings.CutPrefix(sum, "Changed link "); ok {
			ev, rest, ok1 := strings.Cut(rest, " to ")
			to, rest, ok2 := strings.Cut(rest, " from ")
			from, now, ok3 := strings.Cut(rest, " to ")
			if ok1 && ok2 && ok3 && ev != "" && to != "" && relationWords(from) != "" && relationWords(now) != "" {
				return "changed " + with("the source", name(ev)) + ": it now " + relationWords(now) + " " + with("the finding", name(to))
			}
		}
	case "link.remove":
		if rest, ok := strings.CutPrefix(sum, "Unlinked "); ok {
			if ev, from, ok := strings.Cut(rest, " from "); ok && ev != "" && from != "" {
				return "removed " + with("the source", name(ev)) + " from " + with("the finding", name(from))
			}
		}
	case "question.add":
		return questionPhrase(action, "Added question ", ref, sum, name, false)
	case "question.update":
		return questionPhrase(action, "Updated question ", ref, sum, name, false)
	case "question.resolve":
		return questionPhrase(action, "Resolved question ", ref, sum, name, true)
	case "question.defer":
		return questionPhrase(action, "Deferred question ", ref, sum, name, true)
	case "checkpoint":
		// The stored summary is the note the AI saved with its progress.
		if sum != "" {
			return "saved its progress: " + sum
		}
	}
	return ""
}

// recName is the record word of a "claim.update"-style action.
func recName(action string) string {
	rec, _, _ := strings.Cut(action, ".")
	return rec
}

// questionPhrase turns "Added question Q-001: text" into "added something
// to find out: Q-001 "text"". For a new or reworded question the text
// is the question itself, so it is left out when the name already is that
// text; for an answered question it is the answer (detail true) and is
// always kept.
func questionPhrase(action, prefix, ref, sum string, name refNamer, detail bool) string {
	rest, ok := strings.CutPrefix(sum, prefix+ref)
	if !ok || ref == "" {
		return ""
	}
	phrase := fill(timelineVerbs[action], name(ref))
	if rest == "" {
		return phrase
	}
	text, ok := strings.CutPrefix(rest, ": ")
	if !ok || text == "" {
		return ""
	}
	if n := name(ref); !detail && n != ref && n != "" && n != gone {
		return phrase
	}
	if strings.Contains(phrase, ": ") {
		// "added something to find out: Q-001" already has its colon.
		return phrase + ` "` + text + `"`
	}
	return phrase + ": " + text
}

// addFindingPhrase turns "Added finding C-001 with 3 sources", or the older
// "Added claim C-001 [high] with 3 evidence links", into "added the finding
// C-001 with 3 sources".
func addFindingPhrase(rec, ref, sum string, name refNamer) string {
	rest, ok := strings.CutPrefix(sum, "Added finding "+ref)
	if !ok {
		if rest, ok = strings.CutPrefix(sum, "Added "+rec+" "+ref+" ["); ok {
			_, rest, ok = strings.Cut(rest, "]")
		}
	}
	if !ok || ref == "" {
		return ""
	}
	out := "added " + with("the finding", name(ref))
	if rest == "" {
		return out
	}
	links, ok := strings.CutPrefix(rest, " with ")
	n, _, found := strings.Cut(links, " ")
	if !ok || !found {
		return ""
	}
	if n == "1" {
		return out + " with 1 source"
	}
	return out + " with " + n + " sources"
}

// checkPhrase says a record was given a check label: "checked the source
// E-002", "decided the finding C-001 holds" (a person settles a finding, they
// do not check it), "said sources disagree about the finding C-001", "marked
// the source E-002: AI checked the source". The actor is written before it,
// so a person's check needs neither "You" nor "A person".
func checkPhrase(rec, what string, v domain.Verification) string {
	switch v {
	case domain.VerificationHumanVerified:
		if rec != "evidence" {
			return "decided " + what + " holds"
		}
		return "checked " + what
	case domain.VerificationDisputed:
		return "said sources disagree about " + what
	}
	return "marked " + what + ": " + v.PlainLabel()
}

// updateRecordPhrase renders a claim, hypothesis or evidence update. These
// summaries keep the engine's field list: "Set verification of C-001 to
// human_verified" becomes "decided the finding C-001 holds"; "Updated
// hypothesis H-001: rationale, status disproven" becomes "ruled out the
// finding H-001 (also changed: why)" in a terminal, where the reader works
// with the fields, and "ruled out the finding ..." in a shared answer, where
// a list of field names is noise.
func updateRecordPhrase(rec, ref, sum string, name refNamer) string {
	if ref == "" {
		return ""
	}
	shared := name(ref) != ref
	what := with("the "+domain.RecordType(rec).PlainWord(), name(ref))
	if rest, ok := strings.CutPrefix(sum, "Set verification of "+ref+" to "); ok {
		v, status, hasStatus := strings.Cut(rest, "; status ")
		if v == "" {
			return ""
		}
		out := checkPhrase(rec, what, domain.Verification(v))
		if s := statusLabel(rec, status); hasStatus && s != domain.Verification(v).PlainLabel() && s != "Current" {
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
	statusSet := false
	for _, item := range strings.Split(list, ", ") {
		field, value, hasValue := strings.Cut(item, " ")
		switch {
		case field == "":
			return ""
		case field == "confidence":
			// stored, never shown
		case field == "status" && hasValue:
			statusSet = true
			switch s := statusLabel(rec, value); s {
			case "Ruled out":
				primary = "ruled out " + what
			case "Current", "Not tested yet":
				primary = "brought back " + what
			case "Looks right so far":
				primary = "said " + what + " turned out right"
			case "Sources disagree":
				primary = checkPhrase(rec, what, domain.VerificationDisputed)
			default:
				primary = "marked " + what + ": " + s
			}
		case field == "verification" && !hasValue && hasNote:
			// a reset: the note below names the new value
		case field == "verification" && hasValue && primary == "":
			primary = checkPhrase(rec, what, domain.Verification(value))
		case field == "verification" && hasValue && domain.Verification(value) == domain.VerificationDisputed && statusSet:
			// the status already said so
		case field == "verification" && hasValue:
			if domain.Verification(value) == domain.VerificationHumanVerified {
				others = append(others, "checked")
			} else {
				others = append(others, domain.Verification(value).PlainLabel())
			}
		default:
			others = append(others, fieldName(field))
		}
	}
	if shared {
		others = nil
	}
	out := ""
	switch {
	case primary == "" && len(others) == 0:
		out = "updated " + what
	case primary == "":
		out = "updated " + what + " (" + strings.Join(others, ", ") + ")"
	case len(others) == 0:
		out = primary
	default:
		out = primary + " (also changed: " + strings.Join(others, ", ") + ")"
	}
	if hasNote {
		if !strings.HasPrefix(note, "verification reset to ") {
			return ""
		}
		out += ". " + resetSentence
	}
	return out
}

// questionUpdatePhrase renders "Updated question INV-0002: title, question
// text", or the older "Updated investigation ...", as "edited the title and
// the background". A status is the engine's bookkeeping and is not shown.
func questionUpdatePhrase(ref, sum string) string {
	rest, ok := strings.CutPrefix(sum, "Updated question "+ref+": ")
	if !ok {
		rest, ok = strings.CutPrefix(sum, "Updated investigation "+ref+": ")
	}
	if !ok || ref == "" {
		return ""
	}
	if list, _, ok := strings.Cut(rest, " (status "); ok {
		rest = list
	} else if _, _, ok := strings.Cut(rest, " (unarchived to "); ok {
		return timelineVerbs["investigation.unarchive"]
	}
	var edits []string
	for _, field := range strings.Split(rest, ", ") {
		switch field {
		case "status":
		case "title":
			edits = append(edits, "the title")
		case "question text", "problem_statement":
			edits = append(edits, "the background")
		default:
			return ""
		}
	}
	if len(edits) == 0 {
		return ""
	}
	return "edited " + strings.Join(edits, " and ")
}

// fieldName is the plain name of a stored field, in the words the desktop
// uses for it.
func fieldName(field string) string {
	switch field {
	case "statement":
		return "wording"
	case "confidence_rationale", "rationale":
		return "why"
	case "superseded_by":
		return "replaced by"
	case "source_type":
		return "kind of source"
	case "source_label":
		return "name of the source"
	case "source_ref":
		return "where to find it"
	case "captured_at":
		return "date of the source"
	case "raw_detail":
		return "exact words"
	case "metadata":
		return "extra details"
	case "title":
		return "name"
	case "summary":
		return "what it shows"
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

// relationWords is "backs up" or "goes against", or "" for anything else.
func relationWords(rel string) string {
	switch rel {
	case string(domain.RelationSupports):
		return "backs up"
	case string(domain.RelationContradicts):
		return "goes against"
	}
	return ""
}

// TimelinePhrase is timelinePhrase for a terminal: a line of its own, so it
// starts with a capital, and records are named by their codes.
func TimelinePhrase(a domain.Activity) string {
	return upperFirst(timelinePhrase(a, refItself))
}

// upperFirst capitalises a leading ASCII letter.
func upperFirst(s string) string {
	if s != "" && s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-'a'+'A') + s[1:]
	}
	return s
}
