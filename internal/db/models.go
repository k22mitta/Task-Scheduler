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
	StatusDead    JobStatus = "dead"
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

type NullString struct {
	sql.NullString
}

func (ns NullString) MarshalJSON() ([]byte, error) {
	if !ns.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(ns.String)
}

type JobRun struct {
	ID           uuid.UUID  `db:"id"            json:"id"`
	JobID        uuid.UUID  `db:"job_id"        json:"job_id"`
	Attempt      int        `db:"attempt"       json:"attempt"`
	StartedAt    time.Time  `db:"started_at"    json:"started_at"`
	FinishedAt   NullTime   `db:"finished_at"   json:"finished_at"`
	Status       JobStatus  `db:"status"        json:"status"`
	ErrorMessage NullString `db:"error_message" json:"error_message"`
}

type Job struct {
	ID             uuid.UUID       `db:"id"             json:"id"`
	Name           string          `db:"name"           json:"name"`
	Payload        json.RawMessage `db:"payload"        json:"payload"`
	Status         JobStatus       `db:"status"         json:"status"`
	ScheduledAt    time.Time       `db:"scheduled_at"   json:"scheduled_at"`
	StartedAt      NullTime        `db:"started_at"     json:"started_at"`
	FinishedAt     NullTime        `db:"finished_at"    json:"finished_at"`
	Attempts       int             `db:"attempts"       json:"attempts"`
	MaxAttempts    int             `db:"max_attempts"   json:"max_attempts"`
	CronExpression NullString      `db:"cron_expression" json:"cron_expression"`
	CreatedAt      time.Time       `db:"created_at"     json:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"     json:"updated_at"`
}
