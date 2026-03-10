-- 002_add_thumbnail_path.up.sql
-- Add thumbnail_path column to media table

ALTER TABLE media ADD COLUMN IF NOT EXISTS thumbnail_path TEXT NOT NULL DEFAULT '';
