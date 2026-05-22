package usecase_test

import (
	"testing"
	"time"

	"processor/internal/domain"
	"processor/internal/usecase"
)

func TestProcess_ValidEvent(t *testing.T) {
	raw := domain.RawEvent{
		EventID:     "550e8400-e29b-41d4-a716-446655440000",
		DeveloperID: "dev-123",
		MetricType:  "commits",
		Value:       10,
		Repository:  "org/repo",
		Timestamp:   time.Now().Add(-time.Hour),
	}

	processed, err := usecase.Process(raw, "proc-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if processed.EventID != raw.EventID {
		t.Errorf("EventID: got %q, want %q", processed.EventID, raw.EventID)
	}
	if processed.DeveloperID != raw.DeveloperID {
		t.Errorf("DeveloperID: got %q, want %q", processed.DeveloperID, raw.DeveloperID)
	}
	if processed.MetricType != raw.MetricType {
		t.Errorf("MetricType: got %q, want %q", processed.MetricType, raw.MetricType)
	}
	if processed.Value != raw.Value {
		t.Errorf("Value: got %v, want %v", processed.Value, raw.Value)
	}
	if processed.Repository != raw.Repository {
		t.Errorf("Repository: got %q, want %q", processed.Repository, raw.Repository)
	}
	if !processed.Timestamp.Equal(raw.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", processed.Timestamp, raw.Timestamp)
	}
	if processed.ProcessorID != "proc-1" {
		t.Errorf("ProcessorID: got %q, want %q", processed.ProcessorID, "proc-1")
	}
	if processed.ProcessedAt.IsZero() {
		t.Error("ProcessedAt must not be zero")
	}
	if processed.ProcessedAt.Location() != time.UTC {
		t.Errorf("ProcessedAt must be UTC, got %v", processed.ProcessedAt.Location())
	}
}

func TestProcess_ProcessedAtIsRecent(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	raw := domain.RawEvent{
		EventID:     "550e8400-e29b-41d4-a716-446655440000",
		DeveloperID: "dev-1",
		MetricType:  "pull_requests",
		Value:       3,
		Repository:  "org/repo",
		Timestamp:   time.Now().Add(-time.Hour),
	}

	processed, _ := usecase.Process(raw, "proc-x")
	after := time.Now().UTC().Add(time.Second)

	if processed.ProcessedAt.Before(before) || processed.ProcessedAt.After(after) {
		t.Errorf("ProcessedAt %v is not within the expected window [%v, %v]",
			processed.ProcessedAt, before, after)
	}
}

func TestProcess_InvalidEvent_ReturnsError(t *testing.T) {
	tests := []struct {
		name  string
		event domain.RawEvent
	}{
		{
			name: "invalid UUID",
			event: domain.RawEvent{
				EventID:     "not-a-uuid",
				DeveloperID: "dev-1",
				MetricType:  "commits",
				Value:       1,
				Timestamp:   time.Now().Add(-time.Hour),
			},
		},
		{
			name: "empty developer_id",
			event: domain.RawEvent{
				EventID:     "550e8400-e29b-41d4-a716-446655440000",
				DeveloperID: "",
				MetricType:  "commits",
				Value:       1,
				Timestamp:   time.Now().Add(-time.Hour),
			},
		},
		{
			name: "invalid metric_type",
			event: domain.RawEvent{
				EventID:     "550e8400-e29b-41d4-a716-446655440000",
				DeveloperID: "dev-1",
				MetricType:  "invalid",
				Value:       1,
				Timestamp:   time.Now().Add(-time.Hour),
			},
		},
		{
			name: "review_time_minutes over 1440",
			event: domain.RawEvent{
				EventID:     "550e8400-e29b-41d4-a716-446655440000",
				DeveloperID: "dev-1",
				MetricType:  "review_time_minutes",
				Value:       1500,
				Timestamp:   time.Now().Add(-time.Hour),
			},
		},
		{
			name: "future timestamp",
			event: domain.RawEvent{
				EventID:     "550e8400-e29b-41d4-a716-446655440000",
				DeveloperID: "dev-1",
				MetricType:  "commits",
				Value:       1,
				Timestamp:   time.Now().Add(time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := usecase.Process(tt.event, "proc-1")
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
