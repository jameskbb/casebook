package domain

// CheckpointItem is an open item a checkpoint asks the caller to revisit.
// Text is the statement or question cut to one line of at most 200 bytes.
// Status is the record's status; Confidence and Verification are empty for
// questions.
type CheckpointItem struct {
	Type         RecordType   `json:"type"`
	Ref          string       `json:"ref"`
	Text         string       `json:"text"`
	Status       string       `json:"status"`
	Confidence   Confidence   `json:"confidence"`
	Verification Verification `json:"verification"`
}

// CheckpointResult is what Checkpoint returns: the checkpoint activity row
// id, the activity recorded since the previous checkpoint of the same
// investigation (newest first, at most 50; ChangesTruncated when more
// exist), and the open items to revisit, each ordered by ref.
// PreviousCheckpointAt is "" on the first checkpoint.
//
// Created lists the refs a CheckpointBatch created and Updated the refs it
// actually changed (no-op items are left out), each in the order applied;
// both are [] for a plain Checkpoint. VerificationResets names the updated
// claims and hypotheses whose vouched verification an AI edit reset to
// ai_generated (ADR-0014), in the order applied; nil when none.
type CheckpointResult struct {
	ActivityID           int64               `json:"activity_id"`
	Created              []string            `json:"created"`
	Updated              []string            `json:"updated"`
	VerificationResets   []VerificationReset `json:"verification_resets,omitempty"`
	InvestigationRef     string              `json:"investigation_ref"`
	Note                 string              `json:"note"`
	PreviousCheckpointAt string              `json:"previous_checkpoint_at"`
	Changes              []Activity          `json:"changes"`
	ChangesTruncated     bool                `json:"changes_truncated"`
	OpenQuestions        []CheckpointItem    `json:"open_questions"`
	OpenHypotheses       []CheckpointItem    `json:"open_hypotheses"`
	ClaimsToReview       []CheckpointItem    `json:"claims_to_review"`
}

// VerificationReset names a record whose vouched verification an edit reset
// to ai_generated, with the label it had before the edit.
type VerificationReset struct {
	Type                 RecordType   `json:"type"`
	Ref                  string       `json:"ref"`
	PreviousVerification Verification `json:"previous_verification"`
}

// MaxCheckpointItems bounds the items of one CheckpointBatch: every list
// element plus one for the conclusion.
const MaxCheckpointItems = 50

// CheckpointInput is one atomic checkpoint: the note plus optional batches,
// applied in field order (evidence, claims, claim updates, hypotheses,
// hypothesis updates, questions, question updates, conclusion) in one write
// transaction together with the checkpoint row. Claims and hypotheses link
// evidence created in the same batch with evidence_ref "#<key>".
type CheckpointInput struct {
	Note              string               `json:"note"`
	Evidence          []CheckpointEvidence `json:"evidence"`
	Claims            []ClaimInput         `json:"claims"`
	ClaimUpdates      []ClaimUpdate        `json:"claim_updates"`
	Hypotheses        []HypothesisInput    `json:"hypotheses"`
	HypothesisUpdates []HypothesisUpdate   `json:"hypothesis_updates"`
	Questions         []QuestionInput      `json:"questions"`
	QuestionUpdates   []QuestionUpdate     `json:"question_updates"`
	Conclusion        *ConclusionInput     `json:"conclusion,omitempty"`
}

// CheckpointEvidence is new evidence in a checkpoint batch. Key is optional
// (1 to 32 of A-Z, a-z, 0-9, _ and -, unique in the batch); claims and
// hypotheses of the same batch link the evidence as "#<key>".
type CheckpointEvidence struct {
	Key      string        `json:"key"`
	Evidence EvidenceInput `json:"evidence"`
}

// ClaimUpdate patches an existing claim in a checkpoint batch.
type ClaimUpdate struct {
	Ref   string     `json:"ref"`
	Patch ClaimPatch `json:"patch"`
}

// HypothesisUpdate patches an existing hypothesis in a checkpoint batch.
type HypothesisUpdate struct {
	Ref   string          `json:"ref"`
	Patch HypothesisPatch `json:"patch"`
}

// QuestionUpdate changes an existing question in a checkpoint batch: an
// optional new Question text, then at most one of Resolution (resolve),
// Defer (defer, with an optional Note) or Reopen. At least one change is
// required; Note without Defer is a ValidationError.
type QuestionUpdate struct {
	Ref        string  `json:"ref"`
	Question   *string `json:"question,omitempty"`
	Resolution *string `json:"resolution,omitempty"`
	Defer      bool    `json:"defer,omitempty"`
	Note       string  `json:"note,omitempty"`
	Reopen     bool    `json:"reopen,omitempty"`
}
