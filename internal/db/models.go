package db

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type JobStatus string

const (
	StatusPending JobStatus = "pending"
	StatusRunning JobStatus = "running"
	StatusDone    JobStatus = "done"
	StatusFailed  JobStatus = "failed"
)

type Job struct {
	ID          uuid.UUID       `db:"id"`
	Name        string          `db:"name"`
	Payload     json.RawMessage `db:"payload"`
	Status      JobStatus       `db:"status"`
	ScheduledAt time.Time       `db:"scheduled_at"`
	StartedAt   sql.NullTime    `db:"started_at"`
	FinishedAt  sql.NullTime    `db:"finished_at"`
	Attempts    int             `db:"attempts"`
	MaxAttempts int             `db:"max_attempts"`
	CreatedAt   time.Time       `db:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at"`
}
