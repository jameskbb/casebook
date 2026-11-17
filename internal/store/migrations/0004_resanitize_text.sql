-- Casebook schema version 4: re-sanitize stored text (no schema change).
-- Text sanitizing on write (domain.SanitizeText and SanitizeLine) now also
-- drops C1 controls, invisible format characters such as bidi overrides and
-- zero-width spaces, and noncharacters. Rows written before that change still
-- hold them. The Go step registered for this file (goSteps in migrate.go,
-- resanitizeText in resanitize.go) runs in this migration's transaction and
-- applies the same per-field sanitizing to every stored text column. It keeps
-- updated_at, verification and status, writes no activity, and the search
-- index follows through the existing UPDATE triggers.
SELECT 1;
