package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/khushmittal/task-scheduler/internal/api"
	"github.com/khushmittal/task-scheduler/internal/config"
	"github.com/khushmittal/task-scheduler/internal/db"
	"github.com/khushmittal/task-scheduler/internal/scheduler"
	"github.com/khushmittal/task-scheduler/internal/worker"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	database, err := db.New()
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	sched := scheduler.New(database, 5*time.Second)
	go sched.Start(ctx)

	pool := worker.NewPool(database, 10, sched.Jobs())
	pool.Start(ctx)

	repo := db.NewJobRepository(database)
	h := api.NewHandler(repo)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /jobs", h.CreateJob)
	mux.HandleFunc("GET /jobs", h.ListJobs)
	mux.HandleFunc("GET /jobs/{id}", h.GetJob)
	mux.HandleFunc("DELETE /jobs/{id}", h.CancelJob)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: api.LoggingMiddleware(mux),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}()

	log.Printf("server listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}

	pool.Wait()
	log.Println("all workers done, shutting down")
}
