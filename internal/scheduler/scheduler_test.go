package scheduler

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/khushmittal/task-scheduler/internal/db"
	"github.com/khushmittal/task-scheduler/internal/redisdb"

	gosql "database/sql"
)

func setupTest(t *testing.T) (*gosql.DB, *redis.Client) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set")
	}

	database, err := gosql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	redisClient, err := redisdb.New(redisURL)
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	t.Cleanup(func() { redisClient.Close() })

	return database, redisClient
}

func TestScheduler_PicksUpDueJob(t *testing.T) {
	database, redisClient := setupTest(t)

	repo := db.NewJobRepository(database)
	jobName := "scheduler_test_job_" + uuid.New().String()
	scheduledAt := time.Now().Add(-time.Minute)
	job, err := repo.Create(context.Background(), jobName, json.RawMessage("{}"), scheduledAt, 3, "")
	if err != nil {
		t.Fatalf("insert test job: %v", err)
	}
	t.Cleanup(func() {
		database.Exec("DELETE FROM jobs WHERE id = $1", job.ID)
	})

	lockKey := "scheduler:lock:test:" + uuid.New().String()
	sched := NewWithLockKey(database, redisClient, time.Second, lockKey)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go sched.Start(ctx)

	timeout := time.After(4 * time.Second)
	for {
		select {
		case got := <-sched.Jobs():
			if got.ID == job.ID {
				return
			}
		case <-timeout:
			t.Fatal("timed out waiting for job from scheduler")
		}
	}
}

func TestScheduler_NoDuplicateClaims(t *testing.T) {
	database, redisClient := setupTest(t)

	repo := db.NewJobRepository(database)
	scheduledAt := time.Now().Add(-time.Minute)
	job, err := repo.Create(context.Background(), "duplicate_claim_test_"+uuid.New().String(), json.RawMessage("{}"), scheduledAt, 3, "")
	if err != nil {
		t.Fatalf("insert test job: %v", err)
	}
	t.Cleanup(func() {
		database.Exec("DELETE FROM jobs WHERE id = $1", job.ID)
	})

	lockKey := "scheduler:lock:test:" + uuid.New().String()
	sched1 := NewWithLockKey(database, redisClient, time.Second, lockKey)
	sched2 := NewWithLockKey(database, redisClient, time.Second, lockKey)

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	go sched1.Start(ctx)
	go sched2.Start(ctx)

	var (
		mu       sync.Mutex
		received []uuid.UUID
	)

	collect := func(jobs <-chan db.Job) {
		for j := range jobs {
			mu.Lock()
			received = append(received, j.ID)
			mu.Unlock()
		}
	}

	go collect(sched1.Jobs())
	go collect(sched2.Jobs())

	<-ctx.Done()

	mu.Lock()
	defer mu.Unlock()

	count := 0
	for _, id := range received {
		if id == job.ID {
			count++
		}
	}
	if count != 1 {
		t.Errorf("job claimed %d times, want exactly 1", count)
	}
}
