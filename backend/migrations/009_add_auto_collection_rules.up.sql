ALTER TABLE collections
  ADD COLUMN auto_type TEXT,
  ADD COLUMN auto_tag TEXT NOT NULL DEFAULT '',
  ADD COLUMN auto_favorite BOOLEAN,
  ADD COLUMN auto_min_rating INTEGER;
