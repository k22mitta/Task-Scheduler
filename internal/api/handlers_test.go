package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/khushmittal/task-scheduler/internal/db"

	gosql "database/sql"
)

func setupHandler(t *testing.T) *Handler {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}

	database, err := gosql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	return NewHandler(db.NewJobRepository(database))
}

func TestCreateJob_Returns201(t *testing.T) {
	h := setupHandler(t)

	body, _ := json.Marshal(map[string]any{
		"name":         "test_job",
		"scheduled_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		"payload":      map[string]string{"key": "value"},
	})

	r := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateJob(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d — body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["id"] == "" {
		t.Error("expected non-empty id in response")
	}
	if resp["name"] != "test_job" {
		t.Errorf("name: got %q, want %q", resp["name"], "test_job")
	}
	if resp["status"] != "pending" {
		t.Errorf("status: got %q, want %q", resp["status"], "pending")
	}
}

func TestListJobs_Returns200Array(t *testing.T) {
	h := setupHandler(t)

	r := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	w := httptest.NewRecorder()

	h.ListJobs(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d — body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp []any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func TestGetJob_Returns404ForUnknownID(t *testing.T) {
	h := setupHandler(t)

	r := httptest.NewRequest(http.MethodGet, "/jobs/00000000-0000-0000-0000-000000000000", nil)
	r.SetPathValue("id", "00000000-0000-0000-0000-000000000000")
	w := httptest.NewRecorder()

	h.GetJob(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d — body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

func TestCreateJob_Returns400WhenNameMissing(t *testing.T) {
	h := setupHandler(t)

	body, _ := json.Marshal(map[string]any{
		"scheduled_at": time.Now().Add(time.Hour).Format(time.RFC3339),
	})

	r := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateJob(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d — body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] == "" {
		t.Error("expected non-empty error in response")
	}
}
