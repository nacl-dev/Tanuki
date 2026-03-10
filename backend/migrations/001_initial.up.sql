-- 001_initial.up.sql
-- Initial schema for Tanuki media vault

-- ─── Media ────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS media (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    title       TEXT        NOT NULL DEFAULT '',
    type        TEXT        NOT NULL CHECK (type IN ('video','image','manga','comic','doujinshi')),
    file_path   TEXT        NOT NULL UNIQUE,
    file_size   BIGINT      NOT NULL DEFAULT 0,
    checksum    TEXT        NOT NULL DEFAULT '',
    rating      SMALLINT    NOT NULL DEFAULT 0 CHECK (rating BETWEEN 0 AND 5),
    favorite    BOOLEAN     NOT NULL DEFAULT FALSE,
    view_count  INTEGER     NOT NULL DEFAULT 0,
    language    TEXT        NOT NULL DEFAULT '',
    source_url  TEXT        NOT NULL DEFAULT '',
    thumbnail_url TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS media_type_idx     ON media (type);
CREATE INDEX IF NOT EXISTS media_favorite_idx ON media (favorite);
CREATE INDEX IF NOT EXISTS media_title_idx    ON media USING GIN (to_tsvector('simple', title));

-- ─── Tags ─────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tags (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL UNIQUE,
    category    TEXT        NOT NULL DEFAULT 'general'
                            CHECK (category IN ('general','artist','character','parody','genre','meta')),
    usage_count INTEGER     NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS tags_name_idx     ON tags (name);
CREATE INDEX IF NOT EXISTS tags_category_idx ON tags (category);

-- ─── Media ↔ Tags ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS media_tags (
    media_id UUID NOT NULL REFERENCES media (id) ON DELETE CASCADE,
    tag_id   UUID NOT NULL REFERENCES tags  (id) ON DELETE CASCADE,
    PRIMARY KEY (media_id, tag_id)
);

-- ─── Collections ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS collections (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    cover_image_path TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Media ↔ Collections ──────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS media_collections (
    media_id      UUID NOT NULL REFERENCES media       (id) ON DELETE CASCADE,
    collection_id UUID NOT NULL REFERENCES collections (id) ON DELETE CASCADE,
    position      INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (media_id, collection_id)
);

-- ─── Performers ───────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS performers (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL UNIQUE,
    image_path TEXT NOT NULL DEFAULT '',
    metadata   JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Media ↔ Performers ───────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS media_performers (
    media_id     UUID NOT NULL REFERENCES media      (id) ON DELETE CASCADE,
    performer_id UUID NOT NULL REFERENCES performers (id) ON DELETE CASCADE,
    PRIMARY KEY (media_id, performer_id)
);

-- ─── Download jobs ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS download_jobs (
    id               UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    url              TEXT           NOT NULL,
    source_type      TEXT           NOT NULL DEFAULT 'auto',
    status           TEXT           NOT NULL DEFAULT 'queued'
                                    CHECK (status IN ('queued','downloading','processing','completed','failed','paused')),
    progress         NUMERIC(5,2)   NOT NULL DEFAULT 0,
    total_files      INTEGER        NOT NULL DEFAULT 0,
    downloaded_files INTEGER        NOT NULL DEFAULT 0,
    total_bytes      BIGINT         NOT NULL DEFAULT 0,
    downloaded_bytes BIGINT         NOT NULL DEFAULT 0,
    target_directory TEXT           NOT NULL DEFAULT '',
    source_metadata  JSONB,
    auto_tags        JSONB,
    error_message    TEXT           NOT NULL DEFAULT '',
    retry_count      SMALLINT       NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS download_jobs_status_idx ON download_jobs (status);

-- ─── Download schedules ───────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS download_schedules (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT        NOT NULL,
    url_pattern      TEXT        NOT NULL,
    source_type      TEXT        NOT NULL DEFAULT 'auto',
    cron_expression  TEXT        NOT NULL,
    enabled          BOOLEAN     NOT NULL DEFAULT TRUE,
    default_tags     JSONB,
    target_directory TEXT        NOT NULL DEFAULT '',
    last_run         TIMESTAMPTZ,
    next_run         TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
