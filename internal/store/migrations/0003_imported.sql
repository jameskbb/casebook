-- Casebook schema version 3: provenance marker for imported rows (AR-09).
-- imported = 1 marks evidence, claims, hypotheses, questions and activity
-- rows that ImportInvestigation copied from a file: their authors,
-- timestamps and activity summaries are what the file said, not something
-- this database recorded. Existing rows were all written locally.
ALTER TABLE evidence ADD COLUMN imported INTEGER NOT NULL DEFAULT 0 CHECK (imported IN (0, 1));
ALTER TABLE claims ADD COLUMN imported INTEGER NOT NULL DEFAULT 0 CHECK (imported IN (0, 1));
ALTER TABLE hypotheses ADD COLUMN imported INTEGER NOT NULL DEFAULT 0 CHECK (imported IN (0, 1));
ALTER TABLE questions ADD COLUMN imported INTEGER NOT NULL DEFAULT 0 CHECK (imported IN (0, 1));
ALTER TABLE activity ADD COLUMN imported INTEGER NOT NULL DEFAULT 0 CHECK (imported IN (0, 1));
