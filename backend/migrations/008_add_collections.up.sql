ALTER TABLE collections
    ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id);

ALTER TABLE media_collections
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE UNIQUE INDEX IF NOT EXISTS collections_user_name_idx ON collections(user_id, name);
CREATE INDEX IF NOT EXISTS collections_user_id_idx ON collections(user_id);
CREATE INDEX IF NOT EXISTS media_collections_collection_id_idx ON media_collections(collection_id);
