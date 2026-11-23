package contextpack

import (
	"fmt"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// BriefOmitted counts and names the records outside the person's selected
// short version. The ref lists are in ref order; the notice a reader sees
// gives only the counts.
type BriefOmitted struct {
	Claims         int      `json:"claims"`
	Hypotheses     int      `json:"hypotheses"`
	Questions      int      `json:"questions"`
	Evidence       int      `json:"evidence"`
	ClaimRefs      []string `json:"claim_refs"`
	HypothesisRefs []string `json:"hypothesis_refs"`
	QuestionRefs   []string `json:"question_refs"`
	EvidenceRefs   []string `json:"evidence_refs"`
}

// RenderBrief renders the short version of a shared answer: only the
// supplied records, with no summary or recommendation of its own. The app
// selects the records and always keeps findings where sources disagree,
// sources that go against a finding and the findings behind the answer.
// allEvidence is every source of the question, selected or not: the "N of M
// sources checked by a person" line counts them all, so leaving a source out
// of the selection cannot improve the count.
func RenderBrief(s Snapshot, omitted BriefOmitted, allEvidence []domain.Evidence) string {
	var d answerDoc
	d.body(s, allEvidence)
	d.para(briefOmissionNotice(omitted))
	d.para(answerFooter)
	return d.b.String()
}

// BriefLeftOut opens the last paragraph of the short version before the
// footer, which says what was left out. A surface that cuts a long answer
// keeps everything from these words on.
const BriefLeftOut = "This short version leaves out "

// BriefStateLabel writes a record's state in the list a person picks from
// before sharing: the status (left out when it is Current), then the check
// label unless it repeats the status. status is the record's PlainLabel.
func BriefStateLabel(status string, v domain.Verification) string {
	label := verificationLabel(v)
	if status == "" || status == "Current" || status == label {
		return label
	}
	return status + "; " + label
}

// briefOmissionNotice says what the short version leaves out: how many
// findings, things still to find out and sources, and always the history.
func briefOmissionNotice(o BriefOmitted) string {
	var items []string
	for _, n := range []struct {
		count            int
		singular, plural string
	}{
		{max(o.Claims, len(o.ClaimRefs)) + max(o.Hypotheses, len(o.HypothesisRefs)), "finding", "findings"},
		{max(o.Questions, len(o.QuestionRefs)), "thing still to find out", "things still to find out"},
		{max(o.Evidence, len(o.EvidenceRefs)), "source", "sources"},
	} {
		switch {
		case n.count == 1:
			items = append(items, "1 "+n.singular)
		case n.count > 1:
			items = append(items, fmt.Sprintf("%d %s", n.count, n.plural))
		}
	}
	return BriefLeftOut + joinList(append(items, "the history")) + ". Ask the person who shared this for the version with everything."
}

// joinList joins items as "a", "a and b" or "a, b and c".
func joinList(items []string) string {
	if len(items) <= 1 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
