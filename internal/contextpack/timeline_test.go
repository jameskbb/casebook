package contextpack

import (
	"testing"

	"github.com/jameskbb/casebook/internal/domain"
)

func TestTimelinePhrase(t *testing.T) {
	for _, tc := range []struct {
		name, action, target, summary, want string
	}{
		// Rows written by this version.
		{"create", "investigation.create", "INV-0002", `Created question INV-0002 "Calls"`, "Created this question"},
		{"question status", "investigation.update", "INV-0002", "Updated question INV-0002: title, status (status resolved)",
			"Marked the question Resolved; edited the title"},
		{"question in progress", "investigation.update", "INV-0002", "Updated question INV-0002: status (status investigating)",
			"Marked the question In progress"},
		{"question text", "investigation.update", "INV-0002", "Updated question INV-0002: title, question text",
			"Edited the title and the question"},
		{"unarchive", "investigation.update", "INV-0002", "Updated question INV-0002: status (unarchived to investigating)",
			"Unarchived this question"},
		{"archive", "investigation.archive", "INV-0002", "Archived question INV-0002", "Archived this question"},
		{"answer", "conclusion.update", "INV-0002", "Updated conclusion of INV-0002 (confidence high)", "Updated the answer"},
		{"answer reset", "conclusion.update", "INV-0002",
			"Updated conclusion of INV-0002 (confidence high); verification reset to ai_generated after AI edit",
			"Updated the answer. It changed after it was checked, so it reads Not checked again."},
		{"answer cleared", "conclusion.update", "INV-0002", "Cleared the answer", "Cleared the answer"},
		{"answer reviewed", "conclusion.update", "INV-0002", "Recorded a review of the answer", "Recorded a review of the answer"},
		{"source add", "evidence.add", "E-001", `Added source E-001 "Call log"`, `Added source E-001 "Call log"`},
		{"finding add", "claim.add", "C-001", "Added finding C-001 with 3 sources", "Added finding C-001 (3 sources)"},
		{"finding add one", "hypothesis.add", "H-002", "Added finding H-002 with 1 source", "Added finding H-002 (1 source)"},
		{"finding add bare", "hypothesis.add", "H-003", "Added finding H-003", "Added finding H-003"},
		{"check finding", "claim.update", "C-001", "Set verification of C-001 to human_verified", "Marked C-001: You checked this"},
		{"check source", "evidence.update", "E-002", "Set verification of E-002 to source_verified", "Marked E-002: AI checked the source"},
		{"sources disagree", "claim.update", "C-001", "Set verification of C-001 to disputed; status disputed", "Marked C-001: Sources disagree"},
		{"settled", "claim.update", "C-001", "Set verification of C-001 to ai_generated; status active", "Marked C-001: Not checked"},
		{"ruled out", "hypothesis.update", "H-001", "Updated hypothesis H-001: rationale, status disproven",
			"Ruled out H-001; edited the why"},
		{"supported", "hypothesis.update", "H-002", "Updated hypothesis H-002: confidence high, status supported",
			"Marked H-002: Supported"},
		{"confidence only", "claim.update", "C-002", "Updated claim C-002: confidence low", "Updated finding C-002"},
		{"claim ruled out", "claim.update", "C-002", "Updated claim C-002: status rejected", "Ruled out C-002"},
		{"text edit", "claim.update", "C-001", "Updated claim C-001: statement, confidence_rationale",
			"Updated finding C-001: edited the statement, edited the why"},
		{"reset", "claim.update", "C-001",
			"Updated claim C-001: statement, verification; verification reset to ai_generated after AI edit",
			"Updated finding C-001: edited the statement. It changed after it was checked, so it reads Not checked again."},
		{"source type", "evidence.update", "E-001", "Updated evidence E-001: title, source_type file",
			"Updated source E-001: edited the title, source type now File"},
		{"replace", "claim.supersede", "C-001", "C-001 was replaced by C-004; also updated statement",
			"Replaced finding C-001 with C-004"},
		{"link", "link.add", "H-001", "Linked E-003 to H-001 (contradicts)", "Linked source E-003 to H-001, which it contradicts"},
		{"relink", "link.update", "C-001", "Changed link E-002 to C-001 from supports to contradicts",
			"Changed source E-002 from supporting to contradicting C-001"},
		{"unlink", "link.remove", "C-001", "Unlinked E-002 from C-001", "Unlinked source E-002 from C-001"},
		{"ask", "question.add", "Q-001", "Added question Q-001: Is it seasonal?", "Asked Q-001: Is it seasonal?"},
		{"resolve", "question.resolve", "Q-001", "Resolved question Q-001: No.", "Resolved Q-001: No."},
		{"defer bare", "question.defer", "Q-002", "Deferred question Q-002", "Deferred Q-002"},
		{"reopen", "question.reopen", "Q-002", "Reopened question Q-002", "Reopened Q-002"},
		{"import", "import", "INV-0003", "Imported question INV-0003 (2 sources, 1 finding, 0 follow-up questions)", "Imported this question"},
		// Rows an older version wrote stay readable in the new words.
		{"old create", "investigation.create", "INV-0002", `Created investigation INV-0002 "Margin"`, "Created this question"},
		{"old question status", "investigation.update", "INV-0002", "Updated investigation INV-0002: title, status (status resolved)",
			"Marked the question Resolved; edited the title"},
		{"old question text", "investigation.update", "INV-0002", "Updated investigation INV-0002: title, problem_statement",
			"Edited the title and the question"},
		{"old archive", "investigation.archive", "INV-0002", "Archived investigation INV-0002", "Archived this question"},
		{"old answer cleared", "conclusion.update", "INV-0002", "Cleared conclusion of INV-0002", "Cleared the answer"},
		{"old source add", "evidence.add", "E-001", `Added evidence E-001 "CI runs"`, `Added source E-001 "CI runs"`},
		{"old finding add", "claim.add", "C-001", "Added claim C-001 [high] with 3 evidence links", "Added finding C-001 (3 sources)"},
		{"old finding add one", "hypothesis.add", "H-002", "Added hypothesis H-002 [low] with 1 evidence link", "Added finding H-002 (1 source)"},
		{"old finding add bare", "hypothesis.add", "H-003", "Added hypothesis H-003 [low]", "Added finding H-003"},
		{"old replace", "claim.supersede", "C-001", "C-001 superseded by C-004; also updated statement",
			"Replaced finding C-001 with C-004"},
		{"old import", "import", "INV-0003", "Imported INV-0003 (2 evidence, 1 claims, 0 hypotheses, 0 questions)", "Imported this question"},
		// A known action whose summary has another shape uses the table.
		{"odd summary", "claim.update", "C-001", "something an imported file said", "Updated finding C-001"},
		{"odd status", "hypothesis.update", "H-001", "Updated hypothesis H-001: status maybe", "Marked H-001: maybe"},
		// An action the table does not know keeps the stored summary.
		{"fallback", "evernote.post", "INV-0002", "Posted the case block to an Evernote note",
			"Posted the case block to an Evernote note"},
		{"fallback empty", "future.thing", "", "", "Recorded future.thing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := timelinePhrase(domain.Activity{Action: tc.action, TargetRef: tc.target, Summary: tc.summary}, verificationLabel)
			if got != tc.want {
				t.Errorf("timelinePhrase(%s, %q) = %q, want %q", tc.action, tc.summary, got, tc.want)
			}
		})
	}
}
