package db

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJobStatusConstants(t *testing.T) {
	cases := []struct {
		status   JobStatus
		expected string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusDone, "done"},
		{StatusFailed, "failed"},
	}

	for _, tc := range cases {
		if string(tc.status) != tc.expected {
			t.Errorf("got %q, want %q", tc.status, tc.expected)
		}
	}
}

func TestJobStructCreation(t *testing.T) {
	now := time.Now()
	id := uuid.New()
	payload := json.RawMessage(`{"key":"value"}`)

	job := Job{
		ID:          id,
		Name:        "send_email",
		Payload:     payload,
		Status:      StatusPending,
		ScheduledAt: now,
		Attempts:    0,
		MaxAttempts: 3,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if job.ID != id {
		t.Errorf("ID: got %v, want %v", job.ID, id)
	}
	if job.Name != "send_email" {
		t.Errorf("Name: got %q, want %q", job.Name, "send_email")
	}
	if string(job.Payload) != `{"key":"value"}` {
		t.Errorf("Payload: got %q, want %q", job.Payload, `{"key":"value"}`)
	}
	if job.Status != StatusPending {
		t.Errorf("Status: got %q, want %q", job.Status, StatusPending)
	}
	if job.Attempts != 0 {
		t.Errorf("Attempts: got %d, want 0", job.Attempts)
	}
	if job.MaxAttempts != 3 {
		t.Errorf("MaxAttempts: got %d, want 3", job.MaxAttempts)
	}
}

func TestJobNullableTimestamps(t *testing.T) {
	job := Job{
		ID:          uuid.New(),
		Name:        "resize_image",
		Status:      StatusPending,
		ScheduledAt: time.Now(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if job.StartedAt.Valid {
		t.Error("StartedAt.Valid: got true, want false")
	}
	if job.FinishedAt.Valid {
		t.Error("FinishedAt.Valid: got true, want false")
	}
}
