//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"aggregator/internal/domain"
	"aggregator/internal/infra/repository"
	"aggregator/internal/usecase"
)

func newProcessedEvent(devID, metricType string, value float64) domain.ProcessedEvent {
	return domain.ProcessedEvent{
		EventID:     nextEventID(),
		DeveloperID: devID,
		MetricType:  metricType,
		Value:       value,
		Repository:  "test-repo",
		Timestamp:   time.Now().Add(-1 * time.Hour).UTC(),
		ProcessedAt: time.Now().UTC(),
		ProcessorID: "processor-test",
	}
}

func newRepos() (*repository.EventRepository, *repository.SummaryRepository) {
	return repository.NewEventRepository(dynamoClient, eventsTable),
		repository.NewSummaryRepository(dynamoClient, summaryTable)
}

// TestAggregateUseCase_ProcessCreatesEventAndSummary verifica que processar um
// evento persiste o evento individual e cria o summary do desenvolvedor.
func TestAggregateUseCase_ProcessCreatesEventAndSummary(t *testing.T) {
	ctx := context.Background()
	eventRepo, summaryRepo := newRepos()
	uc := usecase.NewAggregateUseCase(eventRepo, summaryRepo)

	devID := "dev-" + t.Name()
	event := newProcessedEvent(devID, "commits", 5)

	if err := uc.Process(ctx, event); err != nil {
		t.Fatalf("Process: %v", err)
	}

	exists, err := eventRepo.Exists(ctx, event.EventID)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatal("evento deve existir no DynamoDB após processamento")
	}

	summary, err := summaryRepo.Get(ctx, devID)
	if err != nil {
		t.Fatalf("Get summary: %v", err)
	}
	if summary == nil {
		t.Fatal("summary deve ser criado após processamento")
	}
	if summary.TotalCommits != 5 {
		t.Errorf("TotalCommits = %d, want 5", summary.TotalCommits)
	}
	if summary.EventsProcessed != 1 {
		t.Errorf("EventsProcessed = %d, want 1", summary.EventsProcessed)
	}
}

// TestAggregateUseCase_Idempotency garante que o mesmo evento processado duas
// vezes não duplica os totais no summary.
func TestAggregateUseCase_Idempotency(t *testing.T) {
	ctx := context.Background()
	eventRepo, summaryRepo := newRepos()
	uc := usecase.NewAggregateUseCase(eventRepo, summaryRepo)

	devID := "dev-" + t.Name()
	event := newProcessedEvent(devID, "commits", 10)

	if err := uc.Process(ctx, event); err != nil {
		t.Fatalf("primeira chamada: %v", err)
	}
	if err := uc.Process(ctx, event); err != nil {
		t.Fatalf("segunda chamada (duplicate): %v", err)
	}

	summary, err := summaryRepo.Get(ctx, devID)
	if err != nil {
		t.Fatalf("Get summary: %v", err)
	}
	if summary.TotalCommits != 10 {
		t.Errorf("TotalCommits = %d, want 10 (duplicate não deve ser contado)", summary.TotalCommits)
	}
	if summary.EventsProcessed != 1 {
		t.Errorf("EventsProcessed = %d, want 1", summary.EventsProcessed)
	}
}

// TestAggregateUseCase_AccumulatesMultipleMetrics verifica a agregação incremental
// de commits, pull_requests e cálculo correto da média de review_time_minutes.
func TestAggregateUseCase_AccumulatesMultipleMetrics(t *testing.T) {
	ctx := context.Background()
	eventRepo, summaryRepo := newRepos()
	uc := usecase.NewAggregateUseCase(eventRepo, summaryRepo)

	devID := "dev-" + t.Name()

	events := []struct {
		metricType string
		value      float64
	}{
		{"commits", 3},
		{"commits", 7},
		{"pull_requests", 2},
		{"review_time_minutes", 60},
		{"review_time_minutes", 120},
	}

	for _, e := range events {
		ev := newProcessedEvent(devID, e.metricType, e.value)
		if err := uc.Process(ctx, ev); err != nil {
			t.Fatalf("Process(%s, %.0f): %v", e.metricType, e.value, err)
		}
	}

	summary, err := summaryRepo.Get(ctx, devID)
	if err != nil {
		t.Fatalf("Get summary: %v", err)
	}

	if summary.TotalCommits != 10 {
		t.Errorf("TotalCommits = %d, want 10", summary.TotalCommits)
	}
	if summary.TotalPullRequests != 2 {
		t.Errorf("TotalPullRequests = %d, want 2", summary.TotalPullRequests)
	}
	// (60 + 120) / 2 = 90
	if summary.AvgReviewTimeMinutes != 90 {
		t.Errorf("AvgReviewTimeMinutes = %.2f, want 90", summary.AvgReviewTimeMinutes)
	}
	if summary.EventsProcessed != 5 {
		t.Errorf("EventsProcessed = %d, want 5", summary.EventsProcessed)
	}
}
