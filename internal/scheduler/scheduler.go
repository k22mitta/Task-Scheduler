package scheduler

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/khushmittal/task-scheduler/internal/db"
	"github.com/khushmittal/task-scheduler/internal/redisdb"
)

type Scheduler struct {
	db          *sql.DB
	redisClient *redis.Client
	ownerID     string
	jobs        chan db.Job
	interval    time.Duration
	done        chan struct{}
}

func New(database *sql.DB, redisClient *redis.Client, interval time.Duration) *Scheduler {
	return &Scheduler{
		db:          database,
		redisClient: redisClient,
		ownerID:     uuid.New().String(),
		jobs:        make(chan db.Job, 100),
		interval:    interval,
		done:        make(chan struct{}),
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
			lock := redisdb.NewLock(s.redisClient, "scheduler:lock", s.ownerID, 10*time.Second)
			acquired, err := lock.Acquire(ctx)
			if err != nil {
				log.Printf("scheduler: lock acquire error: %v", err)
				continue
			}
			if !acquired {
				log.Println("scheduler: lock held by another instance, skipping")
				continue
			}
			if err := s.recoverOrphanedJobs(ctx); err != nil {
				log.Printf("scheduler: orphan recovery error: %v", err)
			}
			if err := s.poll(ctx); err != nil {
				log.Printf("scheduler poll error: %v", err)
			}
			if err := lock.Release(ctx); err != nil {
				log.Printf("scheduler: lock release error: %v", err)
			}
		}
	}
}

func (s *Scheduler) Jobs() <-chan db.Job {
	return s.jobs
}

func (s *Scheduler) recoverOrphanedJobs(ctx context.Context) error {
	const query = `
		UPDATE jobs
		SET status = 'pending', started_at = NULL
		WHERE status = 'running' AND started_at < now() - interval '5 minutes'`

	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		log.Printf("scheduler: recovered %d orphaned jobs", n)
	}
	return nil
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
		          attempts, max_attempts, cron_expression, created_at, updated_at`

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
