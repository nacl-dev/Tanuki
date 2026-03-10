-- 004_add_autotag.sql
-- Add auto-tagging status and metadata columns to media table (v0.4)

ALTER TABLE media ADD COLUMN IF NOT EXISTS auto_tag_status  VARCHAR(20)  NOT NULL DEFAULT 'pending';
ALTER TABLE media ADD COLUMN IF NOT EXISTS auto_tag_source  VARCHAR(50)  NOT NULL DEFAULT '';
ALTER TABLE media ADD COLUMN IF NOT EXISTS auto_tag_similarity REAL      NOT NULL DEFAULT 0;
ALTER TABLE media ADD COLUMN IF NOT EXISTS auto_tagged_at   TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS media_auto_tag_status_idx ON media (auto_tag_status);
