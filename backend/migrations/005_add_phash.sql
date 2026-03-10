-- 005_add_phash.sql
-- Add perceptual hash columns to media table (v0.5)

ALTER TABLE media ADD COLUMN IF NOT EXISTS phash             BIGINT;
ALTER TABLE media ADD COLUMN IF NOT EXISTS phash_computed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS media_phash_idx ON media (phash) WHERE phash IS NOT NULL;
