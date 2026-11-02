package app

import (
	"fmt"

	"github.com/jameskbb/casebook/internal/domain"
)

// checkVerifier refuses verification human_verified from any actor that is
// not a person (ADR-0014). An AI client may mark a record source_verified
// (it checked the record against its source), ai_generated or disputed, and
// may move a human_verified record to any of those, but only a person can
// vouch for a record. Import and system actors are refused as well: import
// lowers verification on its own path (untrustedVerification) and no system
// write sets human_verified.
func checkVerifier(actor domain.ActorKind, v domain.Verification) error {
	if v == domain.VerificationHumanVerified && actor != domain.ActorHuman {
		return domain.NewValidationError("verification",
			"only a person can mark a record human_verified; use source_verified when you checked it against the source, or leave it for a person to verify")
	}
	return nil
}

// vouched reports whether v says someone checked the record's substance:
// a person (human_verified) or a check against the source
// (source_verified).
func vouched(v domain.Verification) bool {
	return v == domain.VerificationHumanVerified || v == domain.VerificationSourceVerified
}

// resetsVerification reports whether an edit by actor that changes a
// record's substance, without naming a verification, lowers a vouched
// verification cur to ai_generated (ADR-0014): the check covered the old
// text, not the new one. A person's edit keeps it.
func resetsVerification(actor domain.ActorKind, cur domain.Verification, substanceChanged, verificationGiven bool) bool {
	return actor != domain.ActorHuman && substanceChanged && !verificationGiven && vouched(cur)
}

// resetNote is the activity summary suffix for such a reset, for example
// "; verification reset to ai_generated after AI edit".
func resetNote(actor domain.ActorKind) string {
	who := string(actor)
	if actor == domain.ActorAI {
		who = "AI"
	}
	return fmt.Sprintf("; verification reset to %s after %s edit", domain.VerificationAIGenerated, who)
}
