CREATE TABLE IF NOT EXISTS jobs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT        NOT NULL,
    payload       JSONB,
    status        TEXT        NOT NULL DEFAULT 'pending',
    scheduled_at  TIMESTAMPTZ NOT NULL,
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ,
    attempts      INTEGER     NOT NULL DEFAULT 0,
    max_attempts  INTEGER     NOT NULL DEFAULT 3,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_jobs_status          ON jobs (status);
CREATE INDEX IF NOT EXISTS idx_jobs_scheduled_at    ON jobs (scheduled_at);
