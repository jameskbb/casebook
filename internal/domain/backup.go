package domain

// BackupInfo describes a backup file. Path is absolute. CreatedAt is the time
// Backup wrote it, or the file's modification time for ValidateBackup.
// Counts are the row counts inside the backup.
type BackupInfo struct {
	Path          string `json:"path"`
	SizeBytes     int64  `json:"size_bytes"`
	SchemaVersion int    `json:"schema_version"`
	CreatedAt     string `json:"created_at"`
	Counts        Counts `json:"counts"`
}

// RestoreResult reports a Restore. SourcePath is the restored backup,
// SafetyCopyPath the automatic copy of the database taken just before the
// restore, SourceSchemaVersion the backup's schema version before it was
// migrated, and Counts the database row counts after the restore.
type RestoreResult struct {
	SourcePath          string `json:"source_path"`
	SafetyCopyPath      string `json:"safety_copy_path"`
	SourceSchemaVersion int    `json:"source_schema_version"`
	Counts              Counts `json:"counts"`
}
