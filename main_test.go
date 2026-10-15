package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCurrentReadingUsesLastCompletedHour(t *testing.T) {
	var data openMeteoResponse
	data.Hourly.Time = []string{"2026-10-14T13:00", "2026-10-14T14:00", "2026-10-14T15:00"}
	data.Hourly.Precipitation = []float64{0.1, 2.4, 9.9}
	data.Hourly.Temperature = []float64{50, 55.5, 60}

	now := time.Date(2026, 10, 14, 15, 30, 0, 0, time.UTC)
	precip, temp, err := currentReading(data, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if precip != 2.4 || temp != 55.5 {
		t.Errorf("got precip=%v temp=%v, want 2.4 and 55.5", precip, temp)
	}
}

func TestCurrentReadingMissingHour(t *testing.T) {
	var data openMeteoResponse
	data.Hourly.Time = []string{"2026-10-14T00:00"}
	data.Hourly.Precipitation = []float64{1}
	data.Hourly.Temperature = []float64{1}

	now := time.Date(2026, 10, 14, 15, 30, 0, 0, time.UTC)
	if _, _, err := currentReading(data, now); err == nil {
		t.Error("expected an error when the hour is missing")
	}
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	healthHandler(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
}
