package app

// UpdateOutcome is what an update did beyond the record it returns.
// VerificationReset is true when an AI or system edit changed what a check
// covered and so reset a vouched verification to ai_generated (ADR-0014);
// PreviousVerification is the label it replaced, empty otherwise. Surfaces
// use it to tell the person their check is gone.
type UpdateOutcome struct {
	VerificationReset    bool         `json:"verification_reset"`
	PreviousVerification Verification `json:"previous_verification,omitempty"`
}
