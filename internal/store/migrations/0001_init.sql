-- Casebook schema version 1: the complete initial schema.
-- Shipped migrations are never edited or dropped; changes go in 0002 and later.
-- Every timestamp column holds domain.TimeLayout text in UTC
-- ("2006-01-02T15:04:05.000Z"), so text order equals time order.
-- Every enum column carries a CHECK list equal to the Go enum in internal/domain.

-- Key/value settings shared by every process. Key active_investigation_id
-- holds the decimal id of the active investigation (ADR-0008).
CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;

-- Global counters. Row 'investigation' allocates INV refs; never reused.
CREATE TABLE counters (
    name  TEXT PRIMARY KEY,
    value INTEGER NOT NULL
) STRICT;

CREATE TABLE investigations (
    id                    INTEGER PRIMARY KEY,
    ref_num               INTEGER NOT NULL UNIQUE CHECK (ref_num >= 1),
    title                 TEXT NOT NULL,
    problem_statement     TEXT NOT NULL DEFAULT '',
    status                TEXT NOT NULL DEFAULT 'new'
        CHECK (status IN ('new', 'investigating', 'blocked', 'resolved', 'archived')),
    pre_archive_status    TEXT
        CHECK (pre_archive_status IS NULL OR pre_archive_status IN ('new', 'investigating', 'blocked', 'resolved')),
    conclusion            TEXT NOT NULL DEFAULT '',
    conclusion_confidence TEXT NOT NULL DEFAULT 'unknown'
        CHECK (conclusion_confidence IN ('unknown', 'low', 'medium', 'high')),
    conclusion_rationale  TEXT NOT NULL DEFAULT '',
    created_at            TEXT NOT NULL,
    updated_at            TEXT NOT NULL,
    archived_at           TEXT,
    created_by_kind       TEXT NOT NULL
        CHECK (created_by_kind IN ('human', 'ai', 'import', 'system')),
    created_by_client     TEXT NOT NULL,
    -- Per-investigation record ref counters (D-06); refs are never reused.
    last_evidence_num     INTEGER NOT NULL DEFAULT 0,
    last_claim_num        INTEGER NOT NULL DEFAULT 0,
    last_hypothesis_num   INTEGER NOT NULL DEFAULT 0,
    last_question_num     INTEGER NOT NULL DEFAULT 0
) STRICT;

CREATE INDEX investigations_updated_at ON investigations (updated_at);

CREATE TABLE evidence (
    id                INTEGER PRIMARY KEY,
    investigation_id  INTEGER NOT NULL REFERENCES investigations (id) ON DELETE CASCADE,
    ref_num           INTEGER NOT NULL CHECK (ref_num >= 1),
    title             TEXT NOT NULL,
    summary           TEXT NOT NULL,
    raw_detail        TEXT NOT NULL DEFAULT '',
    source_type       TEXT NOT NULL
        CHECK (source_type IN ('sql_query', 'code', 'file', 'jira', 'documentation', 'api_response',
                               'user_statement', 'ai_tool_output', 'manual_observation', 'external_url',
                               'casebook_artifact')),
    source_ref        TEXT NOT NULL DEFAULT '',
    source_label      TEXT NOT NULL DEFAULT '',
    captured_at       TEXT NOT NULL,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL,
    created_by_kind   TEXT NOT NULL
        CHECK (created_by_kind IN ('human', 'ai', 'import', 'system')),
    created_by_client TEXT NOT NULL,
    verification      TEXT NOT NULL
        CHECK (verification IN ('ai_generated', 'human_verified', 'source_verified', 'disputed', 'superseded')),
    -- Compact JSON object text, or '' when absent.
    metadata          TEXT NOT NULL DEFAULT '' CHECK (metadata = '' OR json_valid(metadata)),
    UNIQUE (investigation_id, ref_num)
) STRICT;

CREATE TABLE claims (
    id                   INTEGER PRIMARY KEY,
    investigation_id     INTEGER NOT NULL REFERENCES investigations (id) ON DELETE CASCADE,
    ref_num              INTEGER NOT NULL CHECK (ref_num >= 1),
    statement            TEXT NOT NULL,
    confidence           TEXT NOT NULL DEFAULT 'unknown'
        CHECK (confidence IN ('unknown', 'low', 'medium', 'high')),
    confidence_rationale TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disputed', 'superseded', 'rejected')),
    verification         TEXT NOT NULL
        CHECK (verification IN ('ai_generated', 'human_verified', 'source_verified', 'disputed', 'superseded')),
    superseded_by_id     INTEGER REFERENCES claims (id) ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL,
    created_by_kind      TEXT NOT NULL
        CHECK (created_by_kind IN ('human', 'ai', 'import', 'system')),
    created_by_client    TEXT NOT NULL,
    UNIQUE (investigation_id, ref_num)
) STRICT;

CREATE INDEX claims_superseded_by ON claims (superseded_by_id);

CREATE TABLE hypotheses (
    id                INTEGER PRIMARY KEY,
    investigation_id  INTEGER NOT NULL REFERENCES investigations (id) ON DELETE CASCADE,
    ref_num           INTEGER NOT NULL CHECK (ref_num >= 1),
    statement         TEXT NOT NULL,
    confidence        TEXT NOT NULL DEFAULT 'unknown'
        CHECK (confidence IN ('unknown', 'low', 'medium', 'high')),
    rationale         TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'supported', 'disproven', 'superseded')),
    verification      TEXT NOT NULL
        CHECK (verification IN ('ai_generated', 'human_verified', 'source_verified', 'disputed', 'superseded')),
    superseded_by_id  INTEGER REFERENCES hypotheses (id) ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL,
    created_by_kind   TEXT NOT NULL
        CHECK (created_by_kind IN ('human', 'ai', 'import', 'system')),
    created_by_client TEXT NOT NULL,
    UNIQUE (investigation_id, ref_num)
) STRICT;

CREATE INDEX hypotheses_superseded_by ON hypotheses (superseded_by_id);

CREATE TABLE questions (
    id                INTEGER PRIMARY KEY,
    investigation_id  INTEGER NOT NULL REFERENCES investigations (id) ON DELETE CASCADE,
    ref_num           INTEGER NOT NULL CHECK (ref_num >= 1),
    question          TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'resolved', 'deferred')),
    resolution        TEXT NOT NULL DEFAULT '',
    defer_note        TEXT NOT NULL DEFAULT '',
    resolved_at       TEXT,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL,
    created_by_kind   TEXT NOT NULL
        CHECK (created_by_kind IN ('human', 'ai', 'import', 'system')),
    created_by_client TEXT NOT NULL,
    UNIQUE (investigation_id, ref_num)
) STRICT;

CREATE TABLE claim_evidence (
    claim_id          INTEGER NOT NULL REFERENCES claims (id) ON DELETE CASCADE,
    evidence_id       INTEGER NOT NULL REFERENCES evidence (id) ON DELETE CASCADE,
    relation          TEXT NOT NULL CHECK (relation IN ('supports', 'contradicts')),
    created_at        TEXT NOT NULL,
    created_by_kind   TEXT NOT NULL
        CHECK (created_by_kind IN ('human', 'ai', 'import', 'system')),
    created_by_client TEXT NOT NULL,
    PRIMARY KEY (claim_id, evidence_id)
) STRICT;

CREATE INDEX claim_evidence_evidence ON claim_evidence (evidence_id);

CREATE TABLE hypothesis_evidence (
    hypothesis_id     INTEGER NOT NULL REFERENCES hypotheses (id) ON DELETE CASCADE,
    evidence_id       INTEGER NOT NULL REFERENCES evidence (id) ON DELETE CASCADE,
    relation          TEXT NOT NULL CHECK (relation IN ('supports', 'contradicts')),
    created_at        TEXT NOT NULL,
    created_by_kind   TEXT NOT NULL
        CHECK (created_by_kind IN ('human', 'ai', 'import', 'system')),
    created_by_client TEXT NOT NULL,
    PRIMARY KEY (hypothesis_id, evidence_id)
) STRICT;

CREATE INDEX hypothesis_evidence_evidence ON hypothesis_evidence (evidence_id);

-- Simple activity log (ADR-0007): one or more rows per mutation, written in the
-- same transaction. Not event sourcing; never replayed. investigation_id is
-- NULL for global actions (backup, restore, active.clear).
CREATE TABLE activity (
    id               INTEGER PRIMARY KEY,
    investigation_id INTEGER REFERENCES investigations (id) ON DELETE CASCADE,
    actor_kind       TEXT NOT NULL CHECK (actor_kind IN ('human', 'ai', 'import', 'system')),
    actor_client     TEXT NOT NULL,
    action           TEXT NOT NULL,
    target_ref       TEXT NOT NULL DEFAULT '',
    summary          TEXT NOT NULL,
    note             TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL
) STRICT;

CREATE INDEX activity_investigation ON activity (investigation_id, id);

-- Full-text index kept in sync by the triggers below (D-20).
-- rowid = record_id * 8 + type code: investigation 1, evidence 2, claim 3,
-- hypothesis 4, question 5. Evidence raw_detail is never indexed.
CREATE VIRTUAL TABLE search_index USING fts5(
    title,
    body,
    record_type UNINDEXED,
    investigation_id UNINDEXED,
    record_id UNINDEXED,
    tokenize = 'unicode61 remove_diacritics 2'
);

-- investigations: title in title; problem_statement and conclusion in body.
CREATE TRIGGER search_investigations_ai AFTER INSERT ON investigations BEGIN
    INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
    VALUES (new.id * 8 + 1, new.title, new.problem_statement || char(10) || new.conclusion,
            'investigation', new.id, new.id);
END;

CREATE TRIGGER search_investigations_au AFTER UPDATE OF title, problem_statement, conclusion ON investigations BEGIN
    DELETE FROM search_index WHERE rowid = old.id * 8 + 1;
    INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
    VALUES (new.id * 8 + 1, new.title, new.problem_statement || char(10) || new.conclusion,
            'investigation', new.id, new.id);
END;

CREATE TRIGGER search_investigations_ad AFTER DELETE ON investigations BEGIN
    DELETE FROM search_index WHERE rowid = old.id * 8 + 1;
END;

-- evidence: title in title; summary in body (never raw_detail).
CREATE TRIGGER search_evidence_ai AFTER INSERT ON evidence BEGIN
    INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
    VALUES (new.id * 8 + 2, new.title, new.summary, 'evidence', new.investigation_id, new.id);
END;

CREATE TRIGGER search_evidence_au AFTER UPDATE OF title, summary ON evidence BEGIN
    DELETE FROM search_index WHERE rowid = old.id * 8 + 2;
    INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
    VALUES (new.id * 8 + 2, new.title, new.summary, 'evidence', new.investigation_id, new.id);
END;

CREATE TRIGGER search_evidence_ad AFTER DELETE ON evidence BEGIN
    DELETE FROM search_index WHERE rowid = old.id * 8 + 2;
END;

-- claims: statement in body.
CREATE TRIGGER search_claims_ai AFTER INSERT ON claims BEGIN
    INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
    VALUES (new.id * 8 + 3, '', new.statement, 'claim', new.investigation_id, new.id);
END;

CREATE TRIGGER search_claims_au AFTER UPDATE OF statement ON claims BEGIN
    DELETE FROM search_index WHERE rowid = old.id * 8 + 3;
    INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
    VALUES (new.id * 8 + 3, '', new.statement, 'claim', new.investigation_id, new.id);
END;

CREATE TRIGGER search_claims_ad AFTER DELETE ON claims BEGIN
    DELETE FROM search_index WHERE rowid = old.id * 8 + 3;
END;

-- hypotheses: statement in body.
CREATE TRIGGER search_hypotheses_ai AFTER INSERT ON hypotheses BEGIN
    INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
    VALUES (new.id * 8 + 4, '', new.statement, 'hypothesis', new.investigation_id, new.id);
END;

CREATE TRIGGER search_hypotheses_au AFTER UPDATE OF statement ON hypotheses BEGIN
    DELETE FROM search_index WHERE rowid = old.id * 8 + 4;
    INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
    VALUES (new.id * 8 + 4, '', new.statement, 'hypothesis', new.investigation_id, new.id);
END;

CREATE TRIGGER search_hypotheses_ad AFTER DELETE ON hypotheses BEGIN
    DELETE FROM search_index WHERE rowid = old.id * 8 + 4;
END;

-- questions: question and resolution in body.
CREATE TRIGGER search_questions_ai AFTER INSERT ON questions BEGIN
    INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
    VALUES (new.id * 8 + 5, '', new.question || char(10) || new.resolution, 'question',
            new.investigation_id, new.id);
END;

CREATE TRIGGER search_questions_au AFTER UPDATE OF question, resolution ON questions BEGIN
    DELETE FROM search_index WHERE rowid = old.id * 8 + 5;
    INSERT INTO search_index (rowid, title, body, record_type, investigation_id, record_id)
    VALUES (new.id * 8 + 5, '', new.question || char(10) || new.resolution, 'question',
            new.investigation_id, new.id);
END;

CREATE TRIGGER search_questions_ad AFTER DELETE ON questions BEGIN
    DELETE FROM search_index WHERE rowid = old.id * 8 + 5;
END;

-- Link guards (D-14): a claim or hypothesis may only link evidence from its
-- own investigation. Missing rows fall through to the foreign key check.
CREATE TRIGGER claim_evidence_guard_bi BEFORE INSERT ON claim_evidence
WHEN (SELECT investigation_id FROM claims WHERE id = new.claim_id)
     != (SELECT investigation_id FROM evidence WHERE id = new.evidence_id)
BEGIN
    SELECT RAISE(ABORT, 'casebook: cross-investigation link');
END;

CREATE TRIGGER claim_evidence_guard_bu BEFORE UPDATE ON claim_evidence
WHEN (SELECT investigation_id FROM claims WHERE id = new.claim_id)
     != (SELECT investigation_id FROM evidence WHERE id = new.evidence_id)
BEGIN
    SELECT RAISE(ABORT, 'casebook: cross-investigation link');
END;

CREATE TRIGGER hypothesis_evidence_guard_bi BEFORE INSERT ON hypothesis_evidence
WHEN (SELECT investigation_id FROM hypotheses WHERE id = new.hypothesis_id)
     != (SELECT investigation_id FROM evidence WHERE id = new.evidence_id)
BEGIN
    SELECT RAISE(ABORT, 'casebook: cross-investigation link');
END;

CREATE TRIGGER hypothesis_evidence_guard_bu BEFORE UPDATE ON hypothesis_evidence
WHEN (SELECT investigation_id FROM hypotheses WHERE id = new.hypothesis_id)
     != (SELECT investigation_id FROM evidence WHERE id = new.evidence_id)
BEGIN
    SELECT RAISE(ABORT, 'casebook: cross-investigation link');
END;

-- Supersede guards (D-13): the replacement must be another row in the same
-- investigation.
CREATE TRIGGER claims_supersede_guard BEFORE UPDATE OF superseded_by_id ON claims
WHEN new.superseded_by_id IS NOT NULL
     AND (new.superseded_by_id = new.id
          OR (SELECT investigation_id FROM claims WHERE id = new.superseded_by_id) IS NOT new.investigation_id)
BEGIN
    SELECT RAISE(ABORT, 'casebook: invalid supersede: replacement must be another claim in the same investigation');
END;

CREATE TRIGGER hypotheses_supersede_guard BEFORE UPDATE OF superseded_by_id ON hypotheses
WHEN new.superseded_by_id IS NOT NULL
     AND (new.superseded_by_id = new.id
          OR (SELECT investigation_id FROM hypotheses WHERE id = new.superseded_by_id) IS NOT new.investigation_id)
BEGIN
    SELECT RAISE(ABORT, 'casebook: invalid supersede: replacement must be another hypothesis in the same investigation');
END;
