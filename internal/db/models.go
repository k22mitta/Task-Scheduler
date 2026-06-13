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

type NullTime struct {
	sql.NullTime
}

func (nt NullTime) MarshalJSON() ([]byte, error) {
	if !nt.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(nt.Time.Format(time.RFC3339))
}

type Job struct {
	ID          uuid.UUID       `db:"id"           json:"id"`
	Name        string          `db:"name"         json:"name"`
	Payload     json.RawMessage `db:"payload"      json:"payload"`
	Status      JobStatus       `db:"status"       json:"status"`
	ScheduledAt time.Time       `db:"scheduled_at" json:"scheduled_at"`
	StartedAt   NullTime        `db:"started_at"   json:"started_at"`
	FinishedAt  NullTime        `db:"finished_at"  json:"finished_at"`
	Attempts    int             `db:"attempts"     json:"attempts"`
	MaxAttempts int             `db:"max_attempts" json:"max_attempts"`
	CreatedAt   time.Time       `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at"   json:"updated_at"`
}
