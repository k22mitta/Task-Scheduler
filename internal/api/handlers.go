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
	repo *db.JobRepository
}

func NewHandler(repo *db.JobRepository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string          `json:"name"`
		Payload        json.RawMessage `json:"payload"`
		ScheduledAt    time.Time       `json:"scheduled_at"`
		MaxAttempts    int             `json:"max_attempts"`
		CronExpression string          `json:"cron_expression"`
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

	job, err := h.repo.Create(r.Context(), req.Name, req.Payload, req.ScheduledAt, req.MaxAttempts, req.CronExpression)
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

	jobs, err := h.repo.List(r.Context(), limit, offset)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to fetch jobs")
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

	job, err := h.repo.GetByID(r.Context(), id)
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

func (h *Handler) GetJobRuns(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	runs, err := h.repo.ListRunsByJobID(r.Context(), id)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to fetch job runs")
		return
	}

	h.writeJSON(w, http.StatusOK, runs)
}

func (h *Handler) CancelJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	status, err := h.repo.GetStatus(r.Context(), id)
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

	if err := h.repo.Delete(r.Context(), id); err != nil {
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
