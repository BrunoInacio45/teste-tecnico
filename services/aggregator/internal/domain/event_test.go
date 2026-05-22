package domain

import (
	"math"
	"testing"
	"time"
)

func newEvent(metricType string, value float64, ts time.Time) ProcessedEvent {
	return ProcessedEvent{
		EventID:     "550e8400-e29b-41d4-a716-446655440000",
		DeveloperID: "dev-1",
		MetricType:  metricType,
		Value:       value,
		Timestamp:   ts,
	}
}

func TestApply_Commits_IncrementsTotal(t *testing.T) {
	s := &DeveloperSummary{DeveloperID: "dev-1"}
	ts := time.Now()

	s.Apply(newEvent("commits", 5, ts))
	s.Apply(newEvent("commits", 3, ts.Add(time.Minute)))

	if s.TotalCommits != 8 {
		t.Errorf("TotalCommits: got %d, want 8", s.TotalCommits)
	}
	if s.EventsProcessed != 2 {
		t.Errorf("EventsProcessed: got %d, want 2", s.EventsProcessed)
	}
	if s.TotalPullRequests != 0 {
		t.Errorf("TotalPullRequests should be 0, got %d", s.TotalPullRequests)
	}
}

func TestApply_PullRequests_IncrementsTotal(t *testing.T) {
	s := &DeveloperSummary{DeveloperID: "dev-1"}
	ts := time.Now()

	s.Apply(newEvent("pull_requests", 2, ts))
	s.Apply(newEvent("pull_requests", 4, ts.Add(time.Minute)))

	if s.TotalPullRequests != 6 {
		t.Errorf("TotalPullRequests: got %d, want 6", s.TotalPullRequests)
	}
	if s.TotalCommits != 0 {
		t.Errorf("TotalCommits should be 0, got %d", s.TotalCommits)
	}
}

func TestApply_ReviewTimeMinutes_ComputesRunningAverage(t *testing.T) {
	s := &DeveloperSummary{DeveloperID: "dev-1"}
	ts := time.Now()

	s.Apply(newEvent("review_time_minutes", 60, ts))
	if s.AvgReviewTimeMinutes != 60 {
		t.Errorf("after first event: avg = %v, want 60", s.AvgReviewTimeMinutes)
	}

	s.Apply(newEvent("review_time_minutes", 120, ts.Add(time.Minute)))
	// avg(60, 120) = 90
	if s.AvgReviewTimeMinutes != 90 {
		t.Errorf("after second event: avg = %v, want 90", s.AvgReviewTimeMinutes)
	}

	s.Apply(newEvent("review_time_minutes", 30, ts.Add(2*time.Minute)))
	// avg(60, 120, 30) = 70
	const wantAvg = 70.0
	if math.Abs(s.AvgReviewTimeMinutes-wantAvg) > 1e-9 {
		t.Errorf("after third event: avg = %v, want %v", s.AvgReviewTimeMinutes, wantAvg)
	}
}

func TestApply_ReviewTimeMinutes_AuxFieldsUpdated(t *testing.T) {
	s := &DeveloperSummary{DeveloperID: "dev-1"}
	ts := time.Now()

	s.Apply(newEvent("review_time_minutes", 60, ts))
	s.Apply(newEvent("review_time_minutes", 40, ts.Add(time.Minute)))

	if s.ReviewTimeCount != 2 {
		t.Errorf("ReviewTimeCount: got %d, want 2", s.ReviewTimeCount)
	}
	if s.ReviewTimeTotal != 100 {
		t.Errorf("ReviewTimeTotal: got %v, want 100", s.ReviewTimeTotal)
	}
}

func TestApply_LastActivity_TracksLatestTimestamp(t *testing.T) {
	s := &DeveloperSummary{DeveloperID: "dev-1"}
	earlier := time.Now().Add(-2 * time.Hour)
	later := time.Now().Add(-time.Hour)

	// Apply later first, then earlier — last activity must stay as "later"
	s.Apply(newEvent("commits", 1, later))
	s.Apply(newEvent("commits", 1, earlier))

	if !s.LastActivity.Equal(later) {
		t.Errorf("LastActivity: got %v, want %v", s.LastActivity, later)
	}
}

func TestApply_LastActivity_InitiallyZeroThenSet(t *testing.T) {
	s := &DeveloperSummary{DeveloperID: "dev-1"}
	ts := time.Now().Add(-time.Hour)

	if !s.LastActivity.IsZero() {
		t.Error("LastActivity should be zero before any event")
	}

	s.Apply(newEvent("commits", 1, ts))

	if !s.LastActivity.Equal(ts) {
		t.Errorf("LastActivity: got %v, want %v", s.LastActivity, ts)
	}
}

func TestApply_MixedMetrics_IndependentTotals(t *testing.T) {
	s := &DeveloperSummary{DeveloperID: "dev-1"}
	ts := time.Now()

	s.Apply(newEvent("commits", 10, ts))
	s.Apply(newEvent("pull_requests", 3, ts.Add(time.Minute)))
	s.Apply(newEvent("review_time_minutes", 45, ts.Add(2*time.Minute)))
	s.Apply(newEvent("commits", 5, ts.Add(3*time.Minute)))

	if s.TotalCommits != 15 {
		t.Errorf("TotalCommits: got %d, want 15", s.TotalCommits)
	}
	if s.TotalPullRequests != 3 {
		t.Errorf("TotalPullRequests: got %d, want 3", s.TotalPullRequests)
	}
	if s.AvgReviewTimeMinutes != 45 {
		t.Errorf("AvgReviewTimeMinutes: got %v, want 45", s.AvgReviewTimeMinutes)
	}
	if s.EventsProcessed != 4 {
		t.Errorf("EventsProcessed: got %d, want 4", s.EventsProcessed)
	}
}
