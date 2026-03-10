-- v1.0: Community plugin registry
CREATE TABLE IF NOT EXISTS plugins (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    name        TEXT        NOT NULL,
    source_name TEXT        NOT NULL UNIQUE,
    source_url  TEXT        NOT NULL DEFAULT '',
    file_path   TEXT        NOT NULL UNIQUE,
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    version     TEXT        NOT NULL DEFAULT '0.0.0',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_plugins_source_name ON plugins(source_name);
