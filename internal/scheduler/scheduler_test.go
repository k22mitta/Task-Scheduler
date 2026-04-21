package scheduler

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/khushmittal/task-scheduler/internal/db"

	gosql "database/sql"
)

func TestScheduler_PicksUpDueJob(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}

	database, err := gosql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	if err := database.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	repo := db.NewJobRepository(database)

	scheduledAt := time.Now().Add(-time.Minute)
	job, err := repo.Create(context.Background(), "scheduler_test_job", json.RawMessage("{}"), scheduledAt, 3, "")
	if err != nil {
		t.Fatalf("insert test job: %v", err)
	}
	t.Cleanup(func() {
		database.Exec("DELETE FROM jobs WHERE id = $1", job.ID)
	})

	sched := New(database, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go sched.Start(ctx)

	select {
	case got := <-sched.Jobs():
		if got.ID != job.ID {
			t.Errorf("job ID: got %v, want %v", got.ID, job.ID)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("timed out waiting for job from scheduler")
	}
}
