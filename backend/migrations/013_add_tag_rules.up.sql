CREATE TABLE IF NOT EXISTS tag_aliases (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    alias_name TEXT        NOT NULL UNIQUE,
    tag_id     UUID        NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS tag_aliases_tag_id_idx ON tag_aliases (tag_id);

CREATE TABLE IF NOT EXISTS tag_implications (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tag_id         UUID        NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    implied_tag_id UUID        NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tag_id, implied_tag_id),
    CHECK (tag_id <> implied_tag_id)
);

CREATE INDEX IF NOT EXISTS tag_implications_tag_id_idx ON tag_implications (tag_id);
CREATE INDEX IF NOT EXISTS tag_implications_implied_tag_id_idx ON tag_implications (implied_tag_id);
