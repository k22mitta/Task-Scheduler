package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/khushmittal/task-scheduler/internal/db"
	"github.com/khushmittal/task-scheduler/internal/scheduler"
)

type Worker struct {
	id   int
	jobs <-chan db.Job
	db   *sql.DB
	repo *db.JobRepository
}

type Pool struct {
	workers []*Worker
	jobs    chan db.Job
	wg      sync.WaitGroup
}

func NewPool(database *sql.DB, concurrency int, jobs <-chan db.Job) *Pool {
	workers := make([]*Worker, concurrency)
	for i := range workers {
		workers[i] = &Worker{
			id:   i + 1,
			jobs: jobs,
			db:   database,
			repo: db.NewJobRepository(database),
		}
	}
	return &Pool{workers: workers}
}

func (p *Pool) Start(ctx context.Context) {
	for _, w := range p.workers {
		p.wg.Add(1)
		go func(w *Worker) {
			defer p.wg.Done()
			w.run(ctx)
		}(w)
	}
}

func (p *Pool) Wait() {
	p.wg.Wait()
}

func (w *Worker) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-w.jobs:
			if !ok {
				return
			}
			var runID uuid.UUID
			var runErr error
			if runID, runErr = w.runJob(ctx, job); runErr != nil {
				log.Printf("worker %d: job %s failed: %v", w.id, job.ID, runErr)
				if err := w.markFailed(ctx, job, runID, runErr); err != nil {
					log.Printf("worker %d: failed to mark job %s as failed: %v", w.id, job.ID, err)
				}
			}
		}
	}
}

func (w *Worker) runJob(ctx context.Context, job db.Job) (uuid.UUID, error) {
	_, err := w.db.ExecContext(ctx,
		`UPDATE jobs SET attempts = attempts + 1 WHERE id = $1`,
		job.ID,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("increment attempts: %w", err)
	}

	var runID uuid.UUID
	err = w.db.QueryRowContext(ctx,
		`INSERT INTO job_runs (job_id, attempt, started_at, status)
		 VALUES ($1, $2, now(), 'running') RETURNING id`,
		job.ID, job.Attempts+1,
	).Scan(&runID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert job run: %w", err)
	}

	log.Printf("worker %d: starting job %s (%s)", w.id, job.ID, job.Name)

	select {
	case <-ctx.Done():
		return runID, ctx.Err()
	case <-time.After(2 * time.Second):
	}

	_, err = w.db.ExecContext(ctx,
		`UPDATE jobs SET status = 'done', finished_at = now() WHERE id = $1`,
		job.ID,
	)
	if err != nil {
		return runID, fmt.Errorf("mark done: %w", err)
	}

	_, err = w.db.ExecContext(ctx,
		`UPDATE job_runs SET status = 'done', finished_at = now() WHERE id = $1`,
		runID,
	)
	if err != nil {
		return runID, fmt.Errorf("update job run done: %w", err)
	}

	log.Printf("worker %d: finished job %s (%s)", w.id, job.ID, job.Name)

	if job.CronExpression.Valid {
		next, err := scheduler.NextRun(job.CronExpression.String, time.Now())
		if err != nil {
			log.Printf("worker %d: invalid cron expression for job %s: %v", w.id, job.ID, err)
			return runID, nil
		}
		if _, err := w.repo.Create(ctx, job.Name, job.Payload, next, job.MaxAttempts, job.CronExpression.String); err != nil {
			log.Printf("worker %d: failed to reschedule job %s: %v", w.id, job.ID, err)
		}
	}

	return runID, nil
}

func (w *Worker) markFailed(ctx context.Context, job db.Job, runID uuid.UUID, jobErr error) error {
	safeCtx := context.WithoutCancel(ctx)

	errMsg := jobErr.Error()
	if runID != uuid.Nil {
		w.db.ExecContext(safeCtx,
			`UPDATE job_runs SET status = 'failed', finished_at = now(), error_message = $1 WHERE id = $2`,
			errMsg, runID,
		)
	}

	attempts := job.Attempts + 1
	if attempts >= job.MaxAttempts {
		_, err := w.db.ExecContext(safeCtx,
			`UPDATE jobs SET status = 'dead', finished_at = now() WHERE id = $1`,
			job.ID,
		)
		return err
	}

	backoff := time.Duration(30*math.Pow(2, float64(attempts))) * time.Second
	_, err := w.db.ExecContext(safeCtx,
		`UPDATE jobs SET status = 'pending', started_at = NULL, scheduled_at = now() + $1 WHERE id = $2`,
		backoff, job.ID,
	)
	return err
}
