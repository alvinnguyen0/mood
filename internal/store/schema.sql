-- Schema for the mood tracker. Applied on startup (idempotent).

CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL   PRIMARY KEY,
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    timezone      TEXT        NOT NULL DEFAULT 'UTC',
    created_at    TEXT        NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT    PRIMARY KEY,
    user_id    BIGINT  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS mood_entries (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_date TEXT      NOT NULL,
    mood_level INTEGER   NOT NULL CHECK (mood_level BETWEEN 1 AND 5),
    created_at TEXT      NOT NULL,
    updated_at TEXT      NOT NULL,
    UNIQUE (user_id, entry_date)
);

ALTER TABLE mood_entries ADD COLUMN IF NOT EXISTS note TEXT;

CREATE INDEX IF NOT EXISTS idx_mood_user_date ON mood_entries(user_id, entry_date);
CREATE INDEX IF NOT EXISTS idx_mood_date      ON mood_entries(entry_date);
