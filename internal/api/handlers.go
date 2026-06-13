package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/khushmittal/task-scheduler/internal/db"
)

type Handler struct {
	db *sql.DB
}

func NewHandler(database *sql.DB) *Handler {
	return &Handler{db: database}
}

func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string          `json:"name"`
		Payload     json.RawMessage `json:"payload"`
		ScheduledAt time.Time       `json:"scheduled_at"`
		MaxAttempts int             `json:"max_attempts"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.ScheduledAt.IsZero() {
		h.writeError(w, http.StatusBadRequest, "scheduled_at is required")
		return
	}
	if req.MaxAttempts == 0 {
		req.MaxAttempts = 3
	}

	const query = `
		INSERT INTO jobs (name, payload, status, scheduled_at, max_attempts)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, payload, status, scheduled_at, started_at, finished_at,
		          attempts, max_attempts, created_at, updated_at`

	var job db.Job
	err := h.db.QueryRowContext(r.Context(), query,
		req.Name,
		req.Payload,
		db.StatusPending,
		req.ScheduledAt,
		req.MaxAttempts,
	).Scan(
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
	)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	h.writeJSON(w, http.StatusCreated, job)
}

func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	limit := parseQueryInt(r, "limit", 20)
	offset := parseQueryInt(r, "offset", 0)
	if limit > 100 {
		limit = 100
	}

	const query = `
		SELECT id, name, payload, status, scheduled_at, started_at, finished_at,
		       attempts, max_attempts, created_at, updated_at
		FROM jobs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := h.db.QueryContext(r.Context(), query, limit, offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to fetch jobs")
		return
	}
	defer rows.Close()

	jobs := make([]db.Job, 0)
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
			h.writeError(w, http.StatusInternalServerError, "failed to scan job")
			return
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to read jobs")
		return
	}

	h.writeJSON(w, http.StatusOK, jobs)
}

func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	const query = `
		SELECT id, name, payload, status, scheduled_at, started_at, finished_at,
		       attempts, max_attempts, created_at, updated_at
		FROM jobs WHERE id = $1`

	var job db.Job
	err = h.db.QueryRowContext(r.Context(), query, id).Scan(
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
	)
	if errors.Is(err, sql.ErrNoRows) {
		h.writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to fetch job")
		return
	}

	h.writeJSON(w, http.StatusOK, job)
}

func (h *Handler) CancelJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	var status db.JobStatus
	err = h.db.QueryRowContext(r.Context(), `SELECT status FROM jobs WHERE id = $1`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		h.writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to fetch job")
		return
	}

	if status != db.StatusPending {
		h.writeError(w, http.StatusConflict, "only pending jobs can be cancelled")
		return
	}

	_, err = h.db.ExecContext(r.Context(), `DELETE FROM jobs WHERE id = $1`, id)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to cancel job")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %dms", r.Method, r.URL.Path, time.Since(start).Milliseconds())
	})
}

func parseQueryInt(r *http.Request, key string, defaultVal int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
