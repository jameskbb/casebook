package contextpack

import (
	"testing"

	"github.com/jameskbb/casebook/internal/domain"
)

func TestTimelinePhrase(t *testing.T) {
	for _, tc := range []struct {
		name, action, target, summary, want string
	}{
		{"create", "investigation.create", "INV-0002", `Created investigation INV-0002 "Margin"`, "Created this case"},
		{"case status", "investigation.update", "INV-0002", "Updated investigation INV-0002: title, status (status resolved)",
			"Marked the case Resolved; edited the title"},
		{"case text", "investigation.update", "INV-0002", "Updated investigation INV-0002: title, problem_statement",
			"Edited the title and the question"},
		{"unarchive", "investigation.update", "INV-0002", "Updated investigation INV-0002: status (unarchived to investigating)",
			"Unarchived this case as Investigating"},
		{"archive", "investigation.archive", "INV-0002", "Archived investigation INV-0002", "Archived this case"},
		{"conclusion", "conclusion.update", "INV-0002", "Updated conclusion of INV-0002 (confidence high)",
			"Updated the conclusion (High confidence)"},
		{"conclusion unknown", "conclusion.update", "INV-0002", "Updated conclusion of INV-0002 (confidence unknown)",
			"Updated the conclusion (confidence not assessed)"},
		{"conclusion cleared", "conclusion.update", "INV-0002", "Cleared conclusion of INV-0002", "Cleared the conclusion"},
		{"evidence add", "evidence.add", "E-001", `Added evidence E-001 "CI runs"`, `Added evidence E-001 "CI runs"`},
		{"claim add", "claim.add", "C-001", "Added claim C-001 [high] with 3 evidence links",
			"Added claim C-001 (High confidence, 3 evidence links)"},
		{"hypothesis add bare", "hypothesis.add", "H-003", "Added hypothesis H-003 [low]", "Added hypothesis H-003 (Low confidence)"},
		{"verify claim", "claim.update", "C-001", "Set verification of C-001 to human_verified", "Marked C-001 Human verified"},
		{"verify evidence", "evidence.update", "E-002", "Set verification of E-002 to source_verified", "Marked E-002 Source verified"},
		{"dispute", "claim.update", "C-001", "Set verification of C-001 to disputed; status disputed", "Marked C-001 Disputed"},
		{"undispute", "claim.update", "C-001", "Set verification of C-001 to ai_generated; status active",
			"Marked C-001 AI generated; status now Current"},
		{"ruled out", "hypothesis.update", "H-001", "Updated hypothesis H-001: rationale, status disproven",
			"Ruled out H-001; edited the rationale"},
		{"supported", "hypothesis.update", "H-002", "Updated hypothesis H-002: confidence high, status supported",
			"Marked H-002 Supported; confidence now High"},
		{"rejected", "claim.update", "C-002", "Updated claim C-002: status rejected", "Marked C-002 Rejected"},
		{"text edit", "claim.update", "C-001", "Updated claim C-001: statement, confidence_rationale",
			"Updated claim C-001: edited the statement, edited the rationale"},
		{"reset", "claim.update", "C-001",
			"Updated claim C-001: statement, verification; verification reset to ai_generated after AI edit",
			"Updated claim C-001: edited the statement, verification reset to AI generated after an edit"},
		{"evidence source", "evidence.update", "E-001", "Updated evidence E-001: title, source_type sql_query",
			"Updated evidence E-001: edited the title, source now SQL query"},
		{"supersede", "claim.supersede", "C-001", "C-001 superseded by C-004; also updated statement",
			"Superseded claim C-001 (replaced by C-004)"},
		{"link", "link.add", "H-001", "Linked E-003 to H-001 (contradicts)", "Linked evidence E-003 to H-001, which it contradicts"},
		{"relink", "link.update", "C-001", "Changed link E-002 to C-001 from supports to contradicts",
			"Changed evidence E-002 from supporting to contradicting C-001"},
		{"unlink", "link.remove", "C-001", "Unlinked E-002 from C-001", "Unlinked evidence E-002 from C-001"},
		{"ask", "question.add", "Q-001", "Added question Q-001: Is it seasonal?", "Asked Q-001: Is it seasonal?"},
		{"resolve", "question.resolve", "Q-001", "Resolved question Q-001: No.", "Resolved Q-001: No."},
		{"defer bare", "question.defer", "Q-002", "Deferred question Q-002", "Deferred Q-002"},
		{"reopen", "question.reopen", "Q-002", "Reopened question Q-002", "Reopened Q-002"},
		{"import", "import", "INV-0003", "Imported INV-0003 (2 evidence, 1 claims, 0 hypotheses, 0 questions)", "Imported this case"},
		// A known action whose summary has another shape uses the table.
		{"odd summary", "claim.update", "C-001", "something an imported file said", "Updated claim C-001"},
		{"odd status", "hypothesis.update", "H-001", "Updated hypothesis H-001: status maybe", "Marked H-001 maybe"},
		// An action the table does not know keeps the stored summary.
		{"fallback", "evernote.post", "INV-0002", "Posted the case block to an Evernote note",
			"Posted the case block to an Evernote note"},
		{"fallback empty", "future.thing", "", "", "Recorded future.thing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := timelinePhrase(domain.Activity{Action: tc.action, TargetRef: tc.target, Summary: tc.summary})
			if got != tc.want {
				t.Errorf("timelinePhrase(%s, %q) = %q, want %q", tc.action, tc.summary, got, tc.want)
			}
		})
	}
}
