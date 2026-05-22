//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aggregator/internal/domain"
	"aggregator/internal/infra/api"
	"aggregator/internal/usecase"
)

func newTestServer() (*httptest.Server, *usecase.AggregateUseCase) {
	eventRepo, summaryRepo := newRepos()
	uc := usecase.NewAggregateUseCase(eventRepo, summaryRepo)
	router := api.NewRouter(eventRepo, summaryRepo, sqsClient, dynamoClient)
	return httptest.NewServer(router), uc
}

// TestHealthEndpoint verifica que o health check retorna 200 e status "ok"
// quando DynamoDB e SQS estão acessíveis.
func TestHealthEndpoint(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want \"ok\"", body["status"])
	}
}

// TestGetEventsEndpoint verifica que GET /metrics/{developer_id} retorna todos
// os eventos processados de um desenvolvedor.
func TestGetEventsEndpoint(t *testing.T) {
	srv, uc := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	devID := "dev-" + t.Name()

	for i := 0; i < 3; i++ {
		ev := newProcessedEvent(devID, "commits", float64(i+1))
		if err := uc.Process(ctx, ev); err != nil {
			t.Fatalf("seed event %d: %v", i, err)
		}
	}

	resp, err := http.Get(srv.URL + "/metrics/" + devID)
	if err != nil {
		t.Fatalf("GET /metrics/%s: %v", devID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var events []domain.ProcessedEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("got %d events, want 3", len(events))
	}
	for _, ev := range events {
		if ev.DeveloperID != devID {
			t.Errorf("developer_id = %q, want %q", ev.DeveloperID, devID)
		}
	}
}

// TestGetSummaryEndpoint verifica que GET /metrics/{developer_id}/summary retorna
// o resumo agregado correto após o processamento de eventos.
func TestGetSummaryEndpoint(t *testing.T) {
	srv, uc := newTestServer()
	defer srv.Close()

	ctx := context.Background()
	devID := "dev-" + t.Name()

	if err := uc.Process(ctx, newProcessedEvent(devID, "commits", 4)); err != nil {
		t.Fatalf("seed commits: %v", err)
	}
	if err := uc.Process(ctx, newProcessedEvent(devID, "pull_requests", 2)); err != nil {
		t.Fatalf("seed pull_requests: %v", err)
	}

	resp, err := http.Get(srv.URL + "/metrics/" + devID + "/summary")
	if err != nil {
		t.Fatalf("GET /metrics/%s/summary: %v", devID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var summary domain.DeveloperSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if summary.DeveloperID != devID {
		t.Errorf("developer_id = %q, want %q", summary.DeveloperID, devID)
	}
	if summary.TotalCommits != 4 {
		t.Errorf("TotalCommits = %d, want 4", summary.TotalCommits)
	}
	if summary.TotalPullRequests != 2 {
		t.Errorf("TotalPullRequests = %d, want 2", summary.TotalPullRequests)
	}
	if summary.EventsProcessed != 2 {
		t.Errorf("EventsProcessed = %d, want 2", summary.EventsProcessed)
	}
}

// TestGetSummaryEndpoint_NotFound verifica que GET /metrics/{developer_id}/summary
// retorna 404 para um desenvolvedor sem dados.
func TestGetSummaryEndpoint_NotFound(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics/developer-que-nao-existe-xyz/summary")
	if err != nil {
		t.Fatalf("GET summary: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
