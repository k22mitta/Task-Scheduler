package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {}

func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {}

func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {}

func (h *Handler) CancelJob(w http.ResponseWriter, r *http.Request) {}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
