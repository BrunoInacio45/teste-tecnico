package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"aggregator/internal/domain"
	"aggregator/internal/usecase"
)

// --- mock repositories ---

type mockEventRepo struct {
	existing map[string]bool
	saved    []domain.ProcessedEvent
	existErr error
	saveErr  error
}

func (m *mockEventRepo) Exists(_ context.Context, eventID string) (bool, error) {
	if m.existErr != nil {
		return false, m.existErr
	}
	return m.existing[eventID], nil
}

func (m *mockEventRepo) Save(_ context.Context, e domain.ProcessedEvent) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saved = append(m.saved, e)
	return nil
}

type mockSummaryRepo struct {
	data    map[string]*domain.DeveloperSummary
	saved   []domain.DeveloperSummary
	getErr  error
	saveErr error
}

func (m *mockSummaryRepo) Get(_ context.Context, developerID string) (*domain.DeveloperSummary, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.data[developerID], nil
}

func (m *mockSummaryRepo) Save(_ context.Context, s domain.DeveloperSummary) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saved = append(m.saved, s)
	if m.data == nil {
		m.data = make(map[string]*domain.DeveloperSummary)
	}
	cp := s
	m.data[s.DeveloperID] = &cp
	return nil
}

// --- helpers ---

func newTestEvent(id, developerID, metricType string, value float64) domain.ProcessedEvent {
	return domain.ProcessedEvent{
		EventID:     id,
		DeveloperID: developerID,
		MetricType:  metricType,
		Value:       value,
		Timestamp:   time.Now().Add(-time.Hour),
		ProcessedAt: time.Now(),
		ProcessorID: "proc-1",
	}
}

func newUseCase(events *mockEventRepo, summaries *mockSummaryRepo) *usecase.AggregateUseCase {
	return usecase.NewAggregateUseCase(events, summaries)
}

// --- tests ---

func TestProcess_NewEvent_IsSavedAndSummaryCreated(t *testing.T) {
	events := &mockEventRepo{existing: map[string]bool{}}
	summaries := &mockSummaryRepo{data: map[string]*domain.DeveloperSummary{}}
	uc := newUseCase(events, summaries)

	event := newTestEvent("evt-1", "dev-1", "commits", 10)
	if err := uc.Process(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events.saved) != 1 {
		t.Errorf("expected 1 saved event, got %d", len(events.saved))
	}
	if len(summaries.saved) != 1 {
		t.Errorf("expected 1 saved summary, got %d", len(summaries.saved))
	}

	summary := summaries.saved[0]
	if summary.DeveloperID != "dev-1" {
		t.Errorf("summary DeveloperID: got %q, want %q", summary.DeveloperID, "dev-1")
	}
	if summary.TotalCommits != 10 {
		t.Errorf("TotalCommits: got %d, want 10", summary.TotalCommits)
	}
	if summary.EventsProcessed != 1 {
		t.Errorf("EventsProcessed: got %d, want 1", summary.EventsProcessed)
	}
}

func TestProcess_DuplicateEvent_Ignored(t *testing.T) {
	const eventID = "evt-duplicate"
	events := &mockEventRepo{existing: map[string]bool{eventID: true}}
	summaries := &mockSummaryRepo{data: map[string]*domain.DeveloperSummary{}}
	uc := newUseCase(events, summaries)

	event := newTestEvent(eventID, "dev-1", "commits", 10)
	if err := uc.Process(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events.saved) != 0 {
		t.Errorf("duplicate event must not be saved; got %d saves", len(events.saved))
	}
	if len(summaries.saved) != 0 {
		t.Errorf("duplicate event must not update summary; got %d saves", len(summaries.saved))
	}
}

func TestProcess_ExistingDeveloper_SummaryUpdated(t *testing.T) {
	existing := &domain.DeveloperSummary{
		DeveloperID:     "dev-1",
		TotalCommits:    5,
		EventsProcessed: 1,
	}
	events := &mockEventRepo{existing: map[string]bool{}}
	summaries := &mockSummaryRepo{data: map[string]*domain.DeveloperSummary{"dev-1": existing}}
	uc := newUseCase(events, summaries)

	event := newTestEvent("evt-new", "dev-1", "commits", 3)
	if err := uc.Process(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(summaries.saved) != 1 {
		t.Fatalf("expected 1 summary save, got %d", len(summaries.saved))
	}
	summary := summaries.saved[0]
	if summary.TotalCommits != 8 {
		t.Errorf("TotalCommits: got %d, want 8 (5 + 3)", summary.TotalCommits)
	}
	if summary.EventsProcessed != 2 {
		t.Errorf("EventsProcessed: got %d, want 2", summary.EventsProcessed)
	}
}

func TestProcess_ExistsError_PropagatesError(t *testing.T) {
	sentinel := errors.New("dynamo unavailable")
	events := &mockEventRepo{existErr: sentinel}
	summaries := &mockSummaryRepo{}
	uc := newUseCase(events, summaries)

	err := uc.Process(context.Background(), newTestEvent("evt-1", "dev-1", "commits", 1))
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

func TestProcess_SaveEventError_PropagatesError(t *testing.T) {
	sentinel := errors.New("save failed")
	events := &mockEventRepo{existing: map[string]bool{}, saveErr: sentinel}
	summaries := &mockSummaryRepo{}
	uc := newUseCase(events, summaries)

	err := uc.Process(context.Background(), newTestEvent("evt-1", "dev-1", "commits", 1))
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
	if len(summaries.saved) != 0 {
		t.Error("summary must not be updated when event save fails")
	}
}

func TestProcess_GetSummaryError_PropagatesError(t *testing.T) {
	sentinel := errors.New("get failed")
	events := &mockEventRepo{existing: map[string]bool{}}
	summaries := &mockSummaryRepo{getErr: sentinel}
	uc := newUseCase(events, summaries)

	err := uc.Process(context.Background(), newTestEvent("evt-1", "dev-1", "commits", 1))
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

func TestProcess_SaveSummaryError_PropagatesError(t *testing.T) {
	sentinel := errors.New("summary save failed")
	events := &mockEventRepo{existing: map[string]bool{}}
	summaries := &mockSummaryRepo{
		data:    map[string]*domain.DeveloperSummary{},
		saveErr: sentinel,
	}
	uc := newUseCase(events, summaries)

	err := uc.Process(context.Background(), newTestEvent("evt-1", "dev-1", "commits", 1))
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

func TestProcess_MultipleEvents_SamePlayer_AccumulatesCorrectly(t *testing.T) {
	events := &mockEventRepo{existing: map[string]bool{}}
	summaries := &mockSummaryRepo{data: map[string]*domain.DeveloperSummary{}}
	uc := newUseCase(events, summaries)

	ctx := context.Background()
	_ = uc.Process(ctx, newTestEvent("e1", "dev-1", "commits", 10))
	_ = uc.Process(ctx, newTestEvent("e2", "dev-1", "pull_requests", 2))
	_ = uc.Process(ctx, newTestEvent("e3", "dev-1", "commits", 5))

	final := summaries.data["dev-1"]
	if final == nil {
		t.Fatal("expected summary for dev-1, got nil")
	}
	if final.TotalCommits != 15 {
		t.Errorf("TotalCommits: got %d, want 15", final.TotalCommits)
	}
	if final.TotalPullRequests != 2 {
		t.Errorf("TotalPullRequests: got %d, want 2", final.TotalPullRequests)
	}
	if final.EventsProcessed != 3 {
		t.Errorf("EventsProcessed: got %d, want 3", final.EventsProcessed)
	}
}
