package domain

import (
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	const validUUID = "550e8400-e29b-41d4-a716-446655440000"
	pastTime := time.Now().Add(-time.Hour)

	base := RawEvent{
		EventID:     validUUID,
		DeveloperID: "dev-123",
		MetricType:  "commits",
		Value:       5,
		Repository:  "org/repo",
		Timestamp:   pastTime,
	}

	tests := []struct {
		name    string
		event   RawEvent
		wantErr bool
	}{
		{
			name:    "valid commits event",
			event:   base,
			wantErr: false,
		},
		{
			name:    "valid pull_requests event",
			event:   func() RawEvent { e := base; e.MetricType = "pull_requests"; return e }(),
			wantErr: false,
		},
		{
			name:    "valid review_time_minutes event",
			event:   func() RawEvent { e := base; e.MetricType = "review_time_minutes"; e.Value = 120; return e }(),
			wantErr: false,
		},
		{
			name:    "zero value is valid",
			event:   func() RawEvent { e := base; e.Value = 0; return e }(),
			wantErr: false,
		},
		{
			name:    "review_time_minutes at boundary (1440)",
			event:   func() RawEvent { e := base; e.MetricType = "review_time_minutes"; e.Value = 1440; return e }(),
			wantErr: false,
		},
		{
			name:    "commits above 1440 is valid",
			event:   func() RawEvent { e := base; e.MetricType = "commits"; e.Value = 2000; return e }(),
			wantErr: false,
		},
		// event_id errors
		{
			name:    "empty event_id",
			event:   func() RawEvent { e := base; e.EventID = ""; return e }(),
			wantErr: true,
		},
		{
			name:    "event_id not a UUID",
			event:   func() RawEvent { e := base; e.EventID = "not-a-uuid"; return e }(),
			wantErr: true,
		},
		{
			name:    "UUID v1 is rejected (wrong version digit)",
			event:   func() RawEvent { e := base; e.EventID = "550e8400-e29b-11d4-a716-446655440000"; return e }(),
			wantErr: true,
		},
		{
			name:    "UUID v3 is rejected",
			event:   func() RawEvent { e := base; e.EventID = "550e8400-e29b-31d4-a716-446655440000"; return e }(),
			wantErr: true,
		},
		{
			name:    "UUID with invalid variant byte",
			event:   func() RawEvent { e := base; e.EventID = "550e8400-e29b-41d4-1716-446655440000"; return e }(),
			wantErr: true,
		},
		// developer_id errors
		{
			name:    "empty developer_id",
			event:   func() RawEvent { e := base; e.DeveloperID = ""; return e }(),
			wantErr: true,
		},
		// metric_type errors
		{
			name:    "invalid metric_type",
			event:   func() RawEvent { e := base; e.MetricType = "invalid_metric"; return e }(),
			wantErr: true,
		},
		{
			name:    "empty metric_type",
			event:   func() RawEvent { e := base; e.MetricType = ""; return e }(),
			wantErr: true,
		},
		{
			name:    "metric_type case sensitive (Commits rejected)",
			event:   func() RawEvent { e := base; e.MetricType = "Commits"; return e }(),
			wantErr: true,
		},
		// value errors
		{
			name:    "negative value",
			event:   func() RawEvent { e := base; e.Value = -1; return e }(),
			wantErr: true,
		},
		{
			name:    "review_time_minutes exceeds 1440",
			event:   func() RawEvent { e := base; e.MetricType = "review_time_minutes"; e.Value = 1441; return e }(),
			wantErr: true,
		},
		// timestamp errors
		{
			name:    "zero timestamp",
			event:   func() RawEvent { e := base; e.Timestamp = time.Time{}; return e }(),
			wantErr: true,
		},
		{
			name:    "future timestamp",
			event:   func() RawEvent { e := base; e.Timestamp = time.Now().Add(time.Hour); return e }(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
