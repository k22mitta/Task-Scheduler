package scheduler

import (
	"testing"
	"time"
)

func TestNextRun_EveryMinute(t *testing.T) {
	from := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	next, err := NextRun("* * * * *", from)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := from.Add(time.Minute)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextRun_DailyAt9am(t *testing.T) {
	from := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	next, err := NextRun("0 9 * * *", from)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := time.Date(2024, 1, 16, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextRun_DailyAt9am_BeforeNine(t *testing.T) {
	from := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	next, err := NextRun("0 9 * * *", from)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextRun_InvalidExpression(t *testing.T) {
	_, err := NextRun("not a cron expression", time.Now())
	if err == nil {
		t.Error("expected error for invalid cron expression, got nil")
	}
}

func TestNextRun_AlwaysAfterFrom(t *testing.T) {
	cases := []struct {
		expr string
		from time.Time
	}{
		{"* * * * *", time.Now()},
		{"0 9 * * *", time.Now()},
		{"0 0 1 * *", time.Now()},
		{"30 6 * * 1", time.Now()},
	}

	for _, tc := range cases {
		next, err := NextRun(tc.expr, tc.from)
		if err != nil {
			t.Fatalf("expr %q: unexpected error: %v", tc.expr, err)
		}
		if !next.After(tc.from) {
			t.Errorf("expr %q: next %v is not after from %v", tc.expr, next, tc.from)
		}
	}
}
