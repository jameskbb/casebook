package contextpack

import (
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/domain"
)

func TestTimelinePhrase(t *testing.T) {
	for _, tc := range []struct {
		name, action, target, summary, want string
	}{
		// Rows written by this version.
		{"create", "investigation.create", "INV-0002", `Created question INV-0002 "Calls"`, "created this question"},
		{"question status", "investigation.update", "INV-0002", "Updated question INV-0002: title, status (status resolved)",
			"edited the title"},
		{"question status only", "investigation.update", "INV-0002", "Updated question INV-0002: status (status investigating)",
			"edited the question"},
		{"question text", "investigation.update", "INV-0002", "Updated question INV-0002: title, question text",
			"edited the title and the background"},
		{"unarchive", "investigation.update", "INV-0002", "Updated question INV-0002: status (unarchived to investigating)",
			"put this question back in the list"},
		{"archive", "investigation.archive", "INV-0002", "Archived question INV-0002", "archived this question"},
		{"answer", "conclusion.update", "INV-0002", "Updated conclusion of INV-0002 (confidence high)", "updated the answer"},
		{"answer reset", "conclusion.update", "INV-0002",
			"Updated conclusion of INV-0002 (confidence high); verification reset to ai_generated after AI edit",
			"updated the answer. It changed after it was checked, so it needs checking again."},
		{"answer cleared", "conclusion.update", "INV-0002", "Cleared the answer", "cleared the answer"},
		{"answer checked", "conclusion.update", "INV-0002", "Recorded a review of the answer", "checked the answer"},
		{"source add", "evidence.add", "E-001", `Added source E-001 "Call log"`, `added the source E-001 "Call log"`},
		{"source add checked", "evidence.add", "E-001", `Added source E-001 "Call log"; checked by a person`, `added the source E-001 "Call log" and checked it`},
		{"first answer", "conclusion.update", "INV-0002", "Wrote conclusion of INV-0002 (confidence high)", "wrote the answer"},
		{"finding add", "claim.add", "C-001", "Added finding C-001 with 3 sources", "added the finding C-001 with 3 sources"},
		{"finding add one", "hypothesis.add", "H-002", "Added finding H-002 with 1 source", "added the finding H-002 with 1 source"},
		{"finding add bare", "hypothesis.add", "H-003", "Added finding H-003", "added the finding H-003"},
		{"check finding", "claim.update", "C-001", "Set verification of C-001 to human_verified", "decided the finding C-001 holds"},
		{"check source", "evidence.update", "E-002", "Set verification of E-002 to source_verified", "marked the source E-002: AI checked the source"},
		{"sources disagree", "claim.update", "C-001", "Set verification of C-001 to disputed; status disputed", "said sources disagree about the finding C-001"},
		{"settled", "claim.update", "C-001", "Set verification of C-001 to ai_generated; status active", "marked the finding C-001: Not checked"},
		{"ruled out", "hypothesis.update", "H-001", "Updated hypothesis H-001: rationale, status disproven",
			"ruled out the finding H-001 (also changed: why)"},
		{"looks right", "hypothesis.update", "H-002", "Updated hypothesis H-002: confidence high, status supported",
			"said the finding H-002 turned out right"},
		{"confidence only", "claim.update", "C-002", "Updated claim C-002: confidence low", "updated the finding C-002"},
		{"claim ruled out", "claim.update", "C-002", "Updated claim C-002: status rejected", "ruled out the finding C-002"},
		{"claim brought back", "claim.update", "C-002", "Updated claim C-002: status active", "brought back the finding C-002"},
		{"text edit", "claim.update", "C-001", "Updated claim C-001: statement, confidence_rationale",
			"updated the finding C-001 (wording, why)"},
		{"reset", "claim.update", "C-001",
			"Updated claim C-001: statement, verification; verification reset to ai_generated after AI edit",
			"updated the finding C-001 (wording). It changed after it was checked, so it needs checking again."},
		{"source kind", "evidence.update", "E-001", "Updated evidence E-001: title, source_type file",
			"updated the source E-001 (name, kind of source)"},
		{"replace", "claim.supersede", "C-001", "C-001 was replaced by C-004; also updated statement",
			"replaced the finding C-001 with C-004"},
		{"link", "link.add", "H-001", "Linked E-003 to H-001 (contradicts)", "added the source E-003 to the finding H-001, which it goes against"},
		{"relink", "link.update", "C-001", "Changed link E-002 to C-001 from supports to contradicts",
			"changed the source E-002: it now goes against the finding C-001"},
		{"unlink", "link.remove", "C-001", "Unlinked E-002 from C-001", "removed the source E-002 from the finding C-001"},
		{"ask", "question.add", "Q-001", "Added question Q-001: Is it seasonal?", `added something to find out: Q-001 "Is it seasonal?"`},
		{"answer follow-up", "question.resolve", "Q-001", "Resolved question Q-001: No.", "answered Q-001: No."},
		{"later", "question.defer", "Q-002", "Deferred question Q-002", "left Q-002 for later"},
		{"reopen", "question.reopen", "Q-002", "Reopened question Q-002", "opened Q-002 again"},
		{"import", "import", "INV-0003", "Imported question INV-0003 (2 sources, 1 finding, 0 follow-up questions)", "added this question from a file"},
		// A checkpoint keeps the note the AI saved with it.
		{"checkpoint", "checkpoint", "", "Read the call log.", "saved its progress: Read the call log."},
		{"checkpoint bare", "checkpoint", "", "", "saved its progress"},
		// Rows an older version wrote stay readable in the new words.
		{"old create", "investigation.create", "INV-0002", `Created investigation INV-0002 "Margin"`, "created this question"},
		{"old question text", "investigation.update", "INV-0002", "Updated investigation INV-0002: title, problem_statement",
			"edited the title and the background"},
		{"old archive", "investigation.archive", "INV-0002", "Archived investigation INV-0002", "archived this question"},
		{"old answer cleared", "conclusion.update", "INV-0002", "Cleared conclusion of INV-0002", "cleared the answer"},
		{"old source add", "evidence.add", "E-001", `Added evidence E-001 "CI runs"`, `added the source E-001 "CI runs"`},
		{"old finding add", "claim.add", "C-001", "Added claim C-001 [high] with 3 evidence links", "added the finding C-001 with 3 sources"},
		{"old finding add one", "hypothesis.add", "H-002", "Added hypothesis H-002 [low] with 1 evidence link", "added the finding H-002 with 1 source"},
		{"old finding add bare", "hypothesis.add", "H-003", "Added hypothesis H-003 [low]", "added the finding H-003"},
		{"old replace", "claim.supersede", "C-001", "C-001 superseded by C-004; also updated statement",
			"replaced the finding C-001 with C-004"},
		{"old import", "import", "INV-0003", "Imported INV-0003 (2 evidence, 1 claims, 0 hypotheses, 0 questions)", "added this question from a file"},
		// A known action whose summary has another shape uses the table.
		{"odd summary", "claim.update", "C-001", "something an imported file said", "updated the finding C-001"},
		{"odd status", "hypothesis.update", "H-001", "Updated hypothesis H-001: status maybe", "marked the finding H-001: maybe"},
		// An action the table does not know keeps the stored summary.
		{"fallback", "evernote.post", "INV-0002", "Posted the case block to an Evernote note",
			"Posted the case block to an Evernote note"},
		{"fallback empty", "future.thing", "", "", "made a change"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := timelinePhrase(domain.Activity{Action: tc.action, TargetRef: tc.target, Summary: tc.summary}, refItself)
			if got != tc.want {
				t.Errorf("timelinePhrase(%s, %q) = %q, want %q", tc.action, tc.summary, got, tc.want)
			}
		})
	}
}

// In a shared answer a record is named by its own text, never by its code;
// a record that is gone is said to be gone, so no line has a hole.
func TestTimelinePhraseNamesRecordsByText(t *testing.T) {
	name := Snapshot{
		Evidence:   []domain.Evidence{{Ref: "E-003", Title: "Wait times"}},
		Claims:     []domain.Claim{{Ref: "C-001", Statement: "Callers waited"}, {Ref: "C-004", Statement: "Callers asked about a bill"}},
		Hypotheses: []domain.Hypothesis{{Ref: "H-001", Statement: "A new ad"}},
		Questions:  []domain.Question{{Ref: "Q-001", Question: "Is it seasonal?"}},
	}.names()
	for _, tc := range []struct {
		action, target, summary, want string
	}{
		{"evidence.add", "E-003", `Added source E-003 "Wait times, first name"`, `added the source "Wait times"`},
		{"evidence.add", "E-009", `Added source E-009 "Gone"`, "added a source that is no longer here"},
		{"claim.update", "C-001", "Set verification of C-001 to human_verified", `decided the finding "Callers waited" holds`},
		{"claim.update", "C-009", "Set verification of C-009 to human_verified", "decided a finding that is no longer here holds"},
		{"claim.supersede", "C-001", "C-001 was replaced by C-004", `replaced the finding "Callers waited" with "Callers asked about a bill"`},
		{"claim.supersede", "C-001", "C-001 was replaced by C-009", `replaced the finding "Callers waited"`},
		{"link.add", "H-001", "Linked E-003 to H-001 (contradicts)", `added the source "Wait times" to the finding "A new ad", which it goes against`},
		{"question.add", "Q-001", "Added question Q-001: Is it seasonal?", `added something to find out: "Is it seasonal?"`},
		{"question.add", "Q-009", "Added question Q-009: Deleted since?", "added something to find out: Deleted since?"},
		{"question.resolve", "Q-001", "Resolved question Q-001: No.", `answered "Is it seasonal?": No.`},
		{"evidence.delete", "E-009", `Deleted source E-009 "Gone"`, "deleted a source"},
	} {
		got := timelinePhrase(domain.Activity{Action: tc.action, TargetRef: tc.target, Summary: tc.summary}, name)
		if got != tc.want {
			t.Errorf("timelinePhrase(%s, %q) = %q, want %q", tc.action, tc.summary, got, tc.want)
		}
		if codes := refCode.FindAllString(got, -1); codes != nil {
			t.Errorf("%q prints codes %v", got, codes)
		}
	}
}

// The history lines the tester found broken: each reads whole in a shared
// answer, with no hole where a deleted record was and no list of field names.
func TestTimelinePhraseSharedHasNoHoles(t *testing.T) {
	name := Snapshot{
		Evidence: []domain.Evidence{{Ref: "E-002", Title: "Returns desk email"}},
		Claims:   []domain.Claim{{Ref: "C-001", Statement: "Kettles"}},
	}.names()
	for _, tc := range []struct {
		action, target, summary, want string
	}{
		{"evidence.update", "E-002", "Updated evidence E-002: title", `updated the source "Returns desk email"`},
		{"claim.update", "C-001", "Updated claim C-001: status disputed, verification disputed", `said sources disagree about the finding "Kettles"`},
		{"claim.update", "C-001", "Updated claim C-001: verification disputed, status disputed", `said sources disagree about the finding "Kettles"`},
		{"claim.update", "C-001", "Updated claim C-001: status rejected, verification human_verified", `ruled out the finding "Kettles"`},
		{"claim.update", "C-001", "Updated claim C-001: status active, verification ai_generated", `brought back the finding "Kettles"`},
		{"claim.add", "C-003", "Added finding C-003", "added a finding that is no longer here"},
		{"claim.delete", "C-003", "Deleted finding C-003", "deleted a finding"},
		{"question.add", "Q-002", "Added question Q-002", "added something to find out"},
		{"question.resolve", "Q-002", "Resolved question Q-002: Acme", "answered something to find out: Acme"},
		{"question.defer", "Q-002", "Deferred question Q-002", "left something to find out for later"},
		{"question.delete", "Q-002", "Deleted question Q-002", "deleted something to find out"},
		{"link.add", "C-001", "Linked E-009 to C-001 (supports)", `added a source that is no longer here to the finding "Kettles", which it backs up`},
	} {
		got := timelinePhrase(domain.Activity{Action: tc.action, TargetRef: tc.target, Summary: tc.summary}, name)
		if got != tc.want {
			t.Errorf("timelinePhrase(%s, %q) = %q, want %q", tc.action, tc.summary, got, tc.want)
		}
		if strings.Contains(got, gone) || strings.Contains(got, "also changed") || strings.Contains(got, "follow-up") {
			t.Errorf("%q is not a whole line", got)
		}
	}
}
