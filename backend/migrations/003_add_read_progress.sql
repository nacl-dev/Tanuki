-- 003_add_read_progress.sql
-- Add read_progress and read_total columns for tracking reading/viewing position

ALTER TABLE media ADD COLUMN IF NOT EXISTS read_progress INT NOT NULL DEFAULT 0;
ALTER TABLE media ADD COLUMN IF NOT EXISTS read_total INT NOT NULL DEFAULT 0;
