package domain

// QuestionInput is the input to AddQuestion. Question is required.
type QuestionInput struct {
	Question string `json:"question"`
}

// Question is an open question in an investigation. Resolution is set when
// resolved, DeferNote when deferred; ResolvedAt is "" unless resolved.
type Question struct {
	ID               int64          `json:"id"`
	Ref              string         `json:"ref"`
	InvestigationRef string         `json:"investigation_ref"`
	Question         string         `json:"question"`
	Status           QuestionStatus `json:"status"`
	Resolution       string         `json:"resolution"`
	DeferNote        string         `json:"defer_note"`
	ResolvedAt       string         `json:"resolved_at"`
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
	CreatedBy        Actor          `json:"created_by"`
	// Imported: the row came from ImportInvestigation, so its author,
	// timestamps and text are what the file said (AR-09).
	Imported bool `json:"imported"`
}
