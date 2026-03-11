ALTER TABLE users
ADD COLUMN IF NOT EXISTS library_pinned_collection_ids TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[];
