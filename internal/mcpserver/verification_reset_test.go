package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

const resetHumanSentence = "This claim was Human verified; the edit reset it to AI generated. Tell the person and ask them to review it again."

// TestUpdateNamesVerificationReset pins B-4: when an AI edit removes a
// person's check, the result text says so in words the client can relay and
// the structured content carries verification_reset and
// previous_verification; edits that keep the check (an AI confidence edit,
// a person's edit) carry neither.
func TestUpdateNamesVerificationReset(t *testing.T) {
	ctx := context.Background()
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	if _, err := a.SetClaimVerification(ctx, apptest.Human, "", "C-001", app.VerificationHumanVerified); err != nil {
		t.Fatal(err)
	}

	// An AI confidence edit keeps the check.
	res := callTool(t, cs, "casebook_update_claim", map[string]any{"claim_ref": "C-001", "confidence": "high"})
	keys := structuredKeys(t, res.StructuredContent)
	if _, ok := keys["verification_reset"]; ok {
		t.Fatalf("confidence edit has verification_reset: %v", keys)
	}
	if txt := resultText(res); strings.Contains(txt, "reset") {
		t.Fatalf("confidence edit text = %q", txt)
	}

	// An AI statement edit removes it, and says so.
	res = callTool(t, cs, "casebook_update_claim", map[string]any{"claim_ref": "C-001", "statement": "AI wording."})
	if txt, want := resultText(res), "Updated C-001: statement, verification. "+resetHumanSentence; txt != want {
		t.Fatalf("text = %q, want %q", txt, want)
	}
	keys = structuredKeys(t, res.StructuredContent)
	if keys["verification_reset"] != true || keys["previous_verification"] != "human_verified" {
		t.Fatalf("structured = %v", keys)
	}

	// A hypothesis says hypothesis.
	if _, err := a.SetHypothesisVerification(ctx, apptest.AI, "", "H-001", app.VerificationSourceVerified); err != nil {
		t.Fatal(err)
	}
	res = callTool(t, cs, "casebook_update_hypothesis", map[string]any{"hypothesis_ref": "H-001", "statement": "AI H."})
	if txt := resultText(res); !strings.HasSuffix(txt, " This hypothesis was Source verified; the edit reset it to AI generated. Tell the person and ask them to review it again.") {
		t.Fatalf("hypothesis text = %q", txt)
	}

	// A person's edit keeps the check: no flag, no sentence.
	if _, err := a.SetClaimVerification(ctx, apptest.Human, "", "C-001", app.VerificationHumanVerified); err != nil {
		t.Fatal(err)
	}
	out, err := applyClaimUpdate(ctx, a, apptest.Human, "", "C-001", claimPatchFields{Statement: ptrTo("Person wording.")})
	if err != nil {
		t.Fatal(err)
	}
	if out.VerificationReset || out.PreviousVerification != "" || strings.Contains(updateText(out), "reset") {
		t.Fatalf("human edit = %+v, text %q", out.resetFields, updateText(out))
	}
	keys = structuredKeys(t, out)
	if _, ok := keys["verification_reset"]; ok {
		t.Fatalf("human edit has verification_reset: %v", keys)
	}
}

// TestCheckpointNamesVerificationReset pins the checkpoint side: the text
// names every reset ref with its earlier label and the structured content
// lists them.
func TestCheckpointNamesVerificationReset(t *testing.T) {
	ctx := context.Background()
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	if _, err := a.SetClaimVerification(ctx, apptest.Human, "", "C-001", app.VerificationHumanVerified); err != nil {
		t.Fatal(err)
	}
	res := callTool(t, cs, "casebook_checkpoint", map[string]any{
		"note":          "Reworded C-001.",
		"claim_updates": []any{map[string]any{"ref": "C-001", "statement": "Checkpoint wording."}},
	})
	out := decode[checkpointOut](t, res)
	if len(out.VerificationResets) != 1 || out.VerificationResets[0].Ref != "C-001" ||
		out.VerificationResets[0].PreviousVerification != app.VerificationHumanVerified {
		t.Fatalf("resets = %+v", out.VerificationResets)
	}
	want := "Updated: C-001. Claim C-001 was Human verified; the edit reset it to AI generated. Tell the person and ask them to review it again."
	if txt := resultText(res); !strings.Contains(txt, want) {
		t.Fatalf("text = %q, want it to contain %q", txt, want)
	}

	res = callTool(t, cs, "casebook_checkpoint", map[string]any{"note": "Nothing reworded."})
	if keys := structuredKeys(t, res.StructuredContent); keys["verification_resets"] != nil {
		t.Fatalf("plain checkpoint has verification_resets: %v", keys)
	}
}

func ptrTo[T any](v T) *T { return &v }
