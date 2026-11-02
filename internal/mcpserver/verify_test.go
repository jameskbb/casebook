package mcpserver

import (
	"context"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

// TestAICannotMarkHumanVerified pins ADR-0014 at the MCP surface: no tool
// accepts human_verified (the input schema does not offer it, so the call
// is a tool error and nothing is written), while source_verified and
// disputing a human_verified claim still work.
func TestAICannotMarkHumanVerified(t *testing.T) {
	ctx := context.Background()
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	before, err := a.ChangeToken(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		tool string
		args map[string]any
	}{
		{"casebook_update_claim", map[string]any{"claim_ref": "C-002", "verification": "human_verified"}},
		{"casebook_update_hypothesis", map[string]any{"hypothesis_ref": "H-001", "verification": "human_verified"}},
		{"casebook_add_claim", map[string]any{"statement": "x", "verification": "human_verified"}},
		{"casebook_add_hypothesis", map[string]any{"statement": "x", "verification": "human_verified"}},
		{"casebook_add_evidence", map[string]any{"title": "t", "summary": "s", "source_type": "file", "verification": "human_verified"}},
		{"casebook_checkpoint", map[string]any{"note": "n",
			"evidence": []any{map[string]any{"title": "t", "summary": "s", "source_type": "file"}},
			"claims":   []any{map[string]any{"statement": "x", "verification": "human_verified"}}}},
	} {
		t.Run(c.tool, func(t *testing.T) {
			wantToolError(t, callTool(t, cs, c.tool, c.args), "verification")
		})
	}
	if after, _ := a.ChangeToken(ctx); after != before {
		t.Fatal("a refused call wrote something")
	}

	// An AI may mark evidence source_verified and dispute a claim a person
	// verified.
	ev := decode[mutationResult[evidenceRecord]](t, callTool(t, cs, "casebook_add_evidence",
		map[string]any{"title": "t", "summary": "s", "source_type": "file", "verification": "source_verified"}))
	if ev.Record.Verification != app.VerificationSourceVerified {
		t.Fatalf("evidence verification = %s", ev.Record.Verification)
	}
	c, err := a.GetClaim(ctx, "", "C-001")
	if err != nil || c.Verification != app.VerificationHumanVerified {
		t.Fatalf("fixture C-001 = %s, %v", c.Verification, err)
	}
	out := decode[mutationResult[statementRecord]](t, callTool(t, cs, "casebook_update_claim",
		map[string]any{"claim_ref": "C-001", "status": "disputed"}))
	if out.Record.Status != string(app.ClaimStatusDisputed) || out.Record.Verification != app.VerificationDisputed {
		t.Fatalf("disputed C-001 = %s/%s", out.Record.Status, out.Record.Verification)
	}
}
