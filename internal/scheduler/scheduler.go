package scheduler

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/khushmittal/task-scheduler/internal/db"
)

type Scheduler struct {
	db       *sql.DB
	jobs     chan db.Job
	interval time.Duration
	done     chan struct{}
}

func New(database *sql.DB, interval time.Duration) *Scheduler {
	return &Scheduler{
		db:       database,
		jobs:     make(chan db.Job, 100),
		interval: interval,
		done:     make(chan struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	defer close(s.done)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.poll(ctx); err != nil {
				log.Printf("scheduler poll error: %v", err)
			}
		}
	}
}

func (s *Scheduler) Jobs() <-chan db.Job {
	return s.jobs
}

func (s *Scheduler) poll(ctx context.Context) error {
	const query = `
		UPDATE jobs
		SET status = 'running', started_at = now()
		WHERE id IN (
			SELECT id FROM jobs
			WHERE status = 'pending' AND scheduled_at <= now()
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, name, payload, status, scheduled_at, started_at, finished_at,
		          attempts, max_attempts, created_at, updated_at`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var job db.Job
		if err := rows.Scan(
			&job.ID,
			&job.Name,
			&job.Payload,
			&job.Status,
			&job.ScheduledAt,
			&job.StartedAt,
			&job.FinishedAt,
			&job.Attempts,
			&job.MaxAttempts,
			&job.CreatedAt,
			&job.UpdatedAt,
		); err != nil {
			return err
		}

		select {
		case s.jobs <- job:
		default:
			log.Printf("scheduler: jobs channel full, dropping job %s", job.ID)
		}
	}

	return rows.Err()
}
