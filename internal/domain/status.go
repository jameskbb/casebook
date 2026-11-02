package domain

// Counts is the number of rows of each kind in the database.
type Counts struct {
	Investigations  int `json:"investigations"`
	Evidence        int `json:"evidence"`
	Claims          int `json:"claims"`
	Hypotheses      int `json:"hypotheses"`
	Questions       int `json:"questions"`
	ClaimLinks      int `json:"claim_links"`
	HypothesisLinks int `json:"hypothesis_links"`
	Activity        int `json:"activity"`
}

// StatusInfo describes the open database for CLI status and diagnostics.
// ActiveInvestigationRef is "" when none is set.
type StatusInfo struct {
	DBPath                 string `json:"db_path"`
	DataDir                string `json:"data_dir"`
	SchemaVersion          int    `json:"schema_version"`
	ActiveInvestigationRef string `json:"active_investigation_ref"`
	// ActiveDir is the bound directory the active investigation comes
	// from (ADR-0015), "" when it is the global one or none.
	ActiveDir string `json:"active_dir"`
	Counts    Counts `json:"counts"`
}
