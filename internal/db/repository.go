package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type JobRepository struct {
	db *sql.DB
}

func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) Create(ctx context.Context, name string, payload json.RawMessage, scheduledAt time.Time, maxAttempts int, cronExpr string) (*Job, error) {
	const query = `
		INSERT INTO jobs (name, payload, status, scheduled_at, max_attempts, cron_expression)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, payload, status, scheduled_at, started_at, finished_at,
		          attempts, max_attempts, cron_expression, created_at, updated_at`

	if len(payload) == 0 {
		payload = json.RawMessage("{}")
	}

	var cronArg sql.NullString
	if cronExpr != "" {
		cronArg = sql.NullString{String: cronExpr, Valid: true}
	}

	var job Job
	err := r.db.QueryRowContext(ctx, query, name, payload, StatusPending, scheduledAt, maxAttempts, cronArg).Scan(
		&job.ID,
		&job.Name,
		(*[]byte)(&job.Payload),
		&job.Status,
		&job.ScheduledAt,
		&job.StartedAt,
		&job.FinishedAt,
		&job.Attempts,
		&job.MaxAttempts,
		&job.CronExpression,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *JobRepository) List(ctx context.Context, limit, offset int) ([]Job, error) {
	const query = `
		SELECT id, name, payload, status, scheduled_at, started_at, finished_at,
		       attempts, max_attempts, cron_expression, created_at, updated_at
		FROM jobs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]Job, 0)
	for rows.Next() {
		var job Job
		if err := rows.Scan(
			&job.ID,
			&job.Name,
			(*[]byte)(&job.Payload),
			&job.Status,
			&job.ScheduledAt,
			&job.StartedAt,
			&job.FinishedAt,
			&job.Attempts,
			&job.MaxAttempts,
			&job.CronExpression,
			&job.CreatedAt,
			&job.UpdatedAt,
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *JobRepository) GetByID(ctx context.Context, id uuid.UUID) (*Job, error) {
	const query = `
		SELECT id, name, payload, status, scheduled_at, started_at, finished_at,
		       attempts, max_attempts, cron_expression, created_at, updated_at
		FROM jobs WHERE id = $1`

	var job Job
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID,
		&job.Name,
		(*[]byte)(&job.Payload),
		&job.Status,
		&job.ScheduledAt,
		&job.StartedAt,
		&job.FinishedAt,
		&job.Attempts,
		&job.MaxAttempts,
		&job.CronExpression,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *JobRepository) ListRunsByJobID(ctx context.Context, jobID uuid.UUID) ([]JobRun, error) {
	const query = `
		SELECT id, job_id, attempt, started_at, finished_at, status, error_message
		FROM job_runs
		WHERE job_id = $1
		ORDER BY attempt ASC`

	rows, err := r.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := make([]JobRun, 0)
	for rows.Next() {
		var run JobRun
		if err := rows.Scan(
			&run.ID,
			&run.JobID,
			&run.Attempt,
			&run.StartedAt,
			&run.FinishedAt,
			&run.Status,
			&run.ErrorMessage,
		); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *JobRepository) Retry(ctx context.Context, id uuid.UUID) (*Job, error) {
	const query = `
		UPDATE jobs
		SET status = 'pending', scheduled_at = now(), started_at = NULL
		WHERE id = $1
		RETURNING id, name, payload, status, scheduled_at, started_at, finished_at,
		          attempts, max_attempts, cron_expression, created_at, updated_at`

	var job Job
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID,
		&job.Name,
		(*[]byte)(&job.Payload),
		&job.Status,
		&job.ScheduledAt,
		&job.StartedAt,
		&job.FinishedAt,
		&job.Attempts,
		&job.MaxAttempts,
		&job.CronExpression,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *JobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM jobs WHERE id = $1`, id)
	return err
}

func (r *JobRepository) GetStatus(ctx context.Context, id uuid.UUID) (JobStatus, error) {
	var status JobStatus
	err := r.db.QueryRowContext(ctx, `SELECT status FROM jobs WHERE id = $1`, id).Scan(&status)
	return status, err
}
