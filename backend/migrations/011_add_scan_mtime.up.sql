ALTER TABLE media
ADD COLUMN IF NOT EXISTS scan_mtime TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS media_scan_mtime_idx
ON media (scan_mtime);
