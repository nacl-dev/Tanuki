-- 006_add_users.up.sql
-- Add multi-user authentication support (v0.6)

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT NOT NULL UNIQUE,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name  TEXT NOT NULL DEFAULT '',
    role          TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE media ADD COLUMN IF NOT EXISTS owner_id UUID REFERENCES users(id);

ALTER TABLE download_jobs ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id);

ALTER TABLE download_schedules ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id);
