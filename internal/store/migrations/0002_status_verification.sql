-- Casebook schema version 2: one coherent status and verification state for
-- claims and hypotheses (ADR-0013).
--   - verification is 'superseded' exactly when the status is 'superseded';
--   - a claim's verification is 'disputed' exactly when its status is
--     'disputed' (hypotheses have no disputed status).
-- Existing rows are repaired first, then triggers refuse any write that
-- breaks the rule. Repairs keep updated_at and write no activity: they
-- restate what the record already said on the other axis.

-- Claims: the status wins for superseded and disputed.
UPDATE claims SET verification = 'superseded'
WHERE status = 'superseded' AND verification != 'superseded';

-- A claim whose verification says disputed while its status is active was
-- disputed through the verification control only: it becomes disputed.
UPDATE claims SET status = 'disputed'
WHERE status = 'active' AND verification = 'disputed';

UPDATE claims SET verification = 'disputed'
WHERE status = 'disputed' AND verification != 'disputed';

-- Any other leftover disputed or superseded verification (active or rejected
-- claims) returns to its author's default verification.
UPDATE claims
SET verification = CASE created_by_kind WHEN 'human' THEN 'human_verified' ELSE 'ai_generated' END
WHERE status IN ('active', 'rejected') AND verification IN ('disputed', 'superseded');

-- Hypotheses: superseded only; disputed stays a free verification state.
UPDATE hypotheses SET verification = 'superseded'
WHERE status = 'superseded' AND verification != 'superseded';

UPDATE hypotheses
SET verification = CASE created_by_kind WHEN 'human' THEN 'human_verified' ELSE 'ai_generated' END
WHERE status != 'superseded' AND verification = 'superseded';

CREATE TRIGGER claims_status_verification_bi BEFORE INSERT ON claims
WHEN (new.status = 'superseded') != (new.verification = 'superseded')
     OR (new.status = 'disputed') != (new.verification = 'disputed')
BEGIN
    SELECT RAISE(ABORT, 'casebook: claim status and verification disagree');
END;

CREATE TRIGGER claims_status_verification_bu BEFORE UPDATE OF status, verification ON claims
WHEN (new.status = 'superseded') != (new.verification = 'superseded')
     OR (new.status = 'disputed') != (new.verification = 'disputed')
BEGIN
    SELECT RAISE(ABORT, 'casebook: claim status and verification disagree');
END;

CREATE TRIGGER hypotheses_status_verification_bi BEFORE INSERT ON hypotheses
WHEN (new.status = 'superseded') != (new.verification = 'superseded')
BEGIN
    SELECT RAISE(ABORT, 'casebook: hypothesis status and verification disagree');
END;

CREATE TRIGGER hypotheses_status_verification_bu BEFORE UPDATE OF status, verification ON hypotheses
WHEN (new.status = 'superseded') != (new.verification = 'superseded')
BEGIN
    SELECT RAISE(ABORT, 'casebook: hypothesis status and verification disagree');
END;
