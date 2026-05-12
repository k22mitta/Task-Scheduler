CREATE TABLE IF NOT EXISTS job_runs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id        UUID        NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    attempt       INTEGER     NOT NULL,
    started_at    TIMESTAMPTZ NOT NULL,
    finished_at   TIMESTAMPTZ,
    status        TEXT        NOT NULL,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_job_runs_job_id ON job_runs (job_id);
