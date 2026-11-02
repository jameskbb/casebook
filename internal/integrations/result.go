package integrations

// Action is what a setup or remove run did to one config file.
type Action string

// Actions reported per file (D-04).
const (
	ActionAdded     Action = "added"
	ActionUpdated   Action = "updated"
	ActionRemoved   Action = "removed"
	ActionUnchanged Action = "unchanged"
)

// FileChange reports one config file a run wrote (or would have written).
type FileChange struct {
	// File is the resolved path that was (or would be) written; for a
	// symlinked config it is the link's target.
	File   string `json:"file"`
	Action Action `json:"action"`
	// Backup is the backup of the previous bytes, "" when none was written.
	Backup string `json:"backup"`
	// Entries are the Casebook-owned keys touched, for example
	// "mcpServers.casebook". Never nil.
	Entries []string `json:"entries"`
	// Note is an optional one-line remark.
	Note string `json:"note,omitempty"`
}

// RefusedError reports a config file Casebook would not change, because it
// does not parse or a Casebook-owned key has an unexpected shape. Reason
// never contains file contents.
type RefusedError struct {
	File   string
	Reason string
}

func (e *RefusedError) Error() string {
	return "refusing to change " + e.File + ": " + e.Reason + "; nothing was written"
}
