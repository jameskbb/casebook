-- A conclusion's verification covers its statement; review records the selected basis.
-- Existing conclusions have no attributable review and start unverified.
ALTER TABLE investigations ADD COLUMN conclusion_verification TEXT NOT NULL DEFAULT 'ai_generated'
    CHECK (conclusion_verification IN ('ai_generated', 'human_verified', 'source_verified', 'disputed'));
ALTER TABLE investigations ADD COLUMN conclusion_supporting_claims TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(conclusion_supporting_claims) AND json_type(conclusion_supporting_claims) = 'array');
ALTER TABLE investigations ADD COLUMN conclusion_contradicting_claims TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(conclusion_contradicting_claims) AND json_type(conclusion_contradicting_claims) = 'array');
ALTER TABLE investigations ADD COLUMN conclusion_reviewed_at TEXT NOT NULL DEFAULT '';
ALTER TABLE investigations ADD COLUMN conclusion_review_basis TEXT NOT NULL DEFAULT '';
ALTER TABLE investigations ADD COLUMN conclusion_review_snapshot TEXT NOT NULL DEFAULT '';
