-- Schema for the whole API. Applied on start-up when DATABASE_URL is set; the
-- statements are all IF NOT EXISTS so running it repeatedly is harmless.
--
-- Two conventions run through it:
--   * IDs are text, matching the snowflake-ish strings the app already hands out.
--   * Rows people edit carry a version column, incremented on every write, so a
--     stale client gets a conflict rather than silently clobbering someone.

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    display_name  TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    id           TEXT PRIMARY KEY,
    user_id      TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   BYTEA       NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions(user_id);

CREATE TABLE IF NOT EXISTS events (
    id          TEXT PRIMARY KEY,
    user_id     TEXT        NOT NULL,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    start_at    TIMESTAMPTZ NOT NULL,
    end_at      TIMESTAMPTZ NOT NULL,
    all_day     BOOLEAN     NOT NULL DEFAULT FALSE,
    -- The recurrence rule, or NULL for a one-off. Expanding the series is the
    -- caller's job; what lives here is the rule and the days dropped from it.
    repeat_freq  TEXT,
    repeat_until TEXT,
    exdates      TEXT[]     NOT NULL DEFAULT '{}',
    version     INTEGER     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS events_user_start_idx ON events(user_id, start_at);

CREATE TABLE IF NOT EXISTS timetable_entries (
    id          TEXT PRIMARY KEY,
    user_id     TEXT        NOT NULL,
    day_of_week INTEGER     NOT NULL,
    period      INTEGER     NOT NULL,
    subject     TEXT        NOT NULL,
    room        TEXT        NOT NULL DEFAULT '',
    teacher     TEXT        NOT NULL DEFAULT '',
    version     INTEGER     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS timetable_user_idx ON timetable_entries(user_id);

CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    user_id     TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    version     INTEGER     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS projects_user_idx ON projects(user_id);

CREATE TABLE IF NOT EXISTS tasks (
    id          TEXT PRIMARY KEY,
    user_id     TEXT        NOT NULL,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    -- A calendar day, not an instant: a task is due on a day, and pinning it to
    -- a time zone only invites off-by-one bugs.
    due_at      TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL DEFAULT 'OPEN',
    -- Dropping a project unfiles its tasks rather than deleting them.
    project_id  TEXT        REFERENCES projects(id) ON DELETE SET NULL,
    version     INTEGER     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS tasks_user_due_idx ON tasks(user_id, due_at);
-- 締切リマインドメールを送った日付（YYYY-MM-DD）。空なら未送信。同じ日に
-- 二重送信しないためだけの列で、既存DBにも ALTER で追加できるようにしてある
-- （CREATE TABLE IF NOT EXISTS は既存テーブルには列を足してくれないので）。
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS reminded_at TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS colonies (
    id            TEXT PRIMARY KEY,
    name          TEXT        NOT NULL,
    description   TEXT        NOT NULL DEFAULT '',
    owner_user_id TEXT        NOT NULL,
    invite_code   TEXT        NOT NULL UNIQUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS colony_members (
    colony_id TEXT        NOT NULL REFERENCES colonies(id) ON DELETE CASCADE,
    user_id   TEXT        NOT NULL,
    role      TEXT        NOT NULL DEFAULT 'MEMBER',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (colony_id, user_id)
);
CREATE INDEX IF NOT EXISTS colony_members_user_idx ON colony_members(user_id);

CREATE TABLE IF NOT EXISTS shared_items (
    id             TEXT PRIMARY KEY,
    colony_id      TEXT        NOT NULL REFERENCES colonies(id) ON DELETE CASCADE,
    source_type    TEXT        NOT NULL,
    source_id      TEXT        NOT NULL,
    created_by     TEXT        NOT NULL,
    title_snapshot TEXT        NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Sharing the same thing twice into one colony is a conflict, not a second row.
    UNIQUE (colony_id, source_type, source_id)
);

CREATE TABLE IF NOT EXISTS prints (
    id           TEXT PRIMARY KEY,
    user_id      TEXT        NOT NULL,
    object_key   TEXT        NOT NULL DEFAULT '',
    content_type TEXT        NOT NULL DEFAULT '',
    filename     TEXT        NOT NULL DEFAULT '',
    status       TEXT        NOT NULL DEFAULT 'UPLOADED',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS prints_user_idx ON prints(user_id);

CREATE TABLE IF NOT EXISTS analysis_jobs (
    id             TEXT PRIMARY KEY,
    user_id        TEXT        NOT NULL,
    print_id       TEXT,
    content_type   TEXT        NOT NULL DEFAULT '',
    filename       TEXT        NOT NULL DEFAULT '',
    status         TEXT        NOT NULL DEFAULT 'QUEUED',
    attempt_count  INTEGER     NOT NULL DEFAULT 0,
    error_code     TEXT        NOT NULL DEFAULT '',
    error_message  TEXT        NOT NULL DEFAULT '',
    result_summary TEXT        NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS analysis_jobs_user_idx ON analysis_jobs(user_id);
CREATE INDEX IF NOT EXISTS analysis_jobs_status_idx ON analysis_jobs(status);

-- Candidates the analyser pulled out of one image, before a human confirms them.
CREATE TABLE IF NOT EXISTS analysis_results (
    id            TEXT PRIMARY KEY,
    job_id        TEXT        NOT NULL REFERENCES analysis_jobs(id) ON DELETE CASCADE,
    candidate_type TEXT       NOT NULL,
    title         TEXT        NOT NULL DEFAULT '',
    date          TEXT        NOT NULL DEFAULT '',
    time          TEXT        NOT NULL DEFAULT '',
    committed     BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS analysis_results_job_idx ON analysis_results(job_id);

-- Password resets. The token is stored hashed, like a session token, and can be
-- spent once: used_at is set when it is redeemed so a leaked mailbox cannot be
-- replayed later.
CREATE TABLE IF NOT EXISTS password_resets (
    id         TEXT PRIMARY KEY,
    user_id    TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA       NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS password_resets_user_idx ON password_resets(user_id);
