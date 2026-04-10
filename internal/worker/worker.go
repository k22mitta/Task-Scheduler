package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/khushmittal/task-scheduler/internal/db"
)

type Worker struct {
	id   int
	jobs <-chan db.Job
	db   *sql.DB
}

type Pool struct {
	workers []*Worker
	jobs    chan db.Job
}

func NewPool(database *sql.DB, concurrency int, jobs <-chan db.Job) *Pool {
	workers := make([]*Worker, concurrency)
	for i := range workers {
		workers[i] = &Worker{
			id:   i + 1,
			jobs: jobs,
			db:   database,
		}
	}
	return &Pool{workers: workers}
}

func (p *Pool) Start(ctx context.Context) {
	for _, w := range p.workers {
		go w.run(ctx)
	}
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
			if err := w.runJob(ctx, job); err != nil {
				log.Printf("worker %d: job %s failed: %v", w.id, job.ID, err)
				if err := w.markFailed(ctx, job); err != nil {
					log.Printf("worker %d: failed to mark job %s as failed: %v", w.id, job.ID, err)
				}
			}
		}
	}
}

func (w *Worker) runJob(ctx context.Context, job db.Job) error {
	_, err := w.db.ExecContext(ctx,
		`UPDATE jobs SET attempts = attempts + 1 WHERE id = $1`,
		job.ID,
	)
	if err != nil {
		return fmt.Errorf("increment attempts: %w", err)
	}

	log.Printf("worker %d: starting job %s (%s)", w.id, job.ID, job.Name)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
	}

	_, err = w.db.ExecContext(ctx,
		`UPDATE jobs SET status = 'done', finished_at = now() WHERE id = $1`,
		job.ID,
	)
	if err != nil {
		return fmt.Errorf("mark done: %w", err)
	}

	log.Printf("worker %d: finished job %s (%s)", w.id, job.ID, job.Name)
	return nil
}

func (w *Worker) markFailed(ctx context.Context, job db.Job) error {
	_, err := w.db.ExecContext(
		context.WithoutCancel(ctx),
		`UPDATE jobs SET status = 'failed', finished_at = now() WHERE id = $1`,
		job.ID,
	)
	return err
}
