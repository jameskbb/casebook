-- Casebook schema version 5: per-directory active investigations
-- (ADR-0015). A row binds a directory (usually a git repository root) to an
-- investigation. A surface that knows its working directory uses the binding
-- of that directory or its nearest bound ancestor before the global
-- active_investigation_id setting. dir is an absolute, cleaned path as the
-- binding machine spells it (lowercased on Windows). Deleting the
-- investigation deletes its bindings.
CREATE TABLE directory_investigations (
    dir              TEXT PRIMARY KEY CHECK (length(dir) BETWEEN 1 AND 4096),
    investigation_id INTEGER NOT NULL REFERENCES investigations (id) ON DELETE CASCADE,
    updated_at       TEXT NOT NULL
);
CREATE INDEX directory_investigations_investigation ON directory_investigations (investigation_id);
