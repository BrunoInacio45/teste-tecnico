package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"aggregator/internal/domain"
)

type EventReader interface {
	FindByDeveloperID(ctx context.Context, developerID string) ([]domain.ProcessedEvent, error)
}

type SummaryReader interface {
	Get(ctx context.Context, developerID string) (*domain.DeveloperSummary, error)
}

// Handler agrupa os repositórios necessários para os endpoints.
type Handler struct {
	events    EventReader
	summaries SummaryReader
	sqs       *sqs.Client
	dynamo    *dynamodb.Client
}

// NewRouter registra todas as rotas HTTP do aggregator.
func NewRouter(events EventReader, summaries SummaryReader, sqsClient *sqs.Client, dynamoClient *dynamodb.Client) http.Handler {
	h := &Handler{events: events, summaries: summaries, sqs: sqsClient, dynamo: dynamoClient}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /metrics/{developer_id}/summary", h.getSummary)
	mux.HandleFunc("GET /metrics/{developer_id}", h.getEvents)
	return mux
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if _, err := h.dynamo.ListTables(ctx, &dynamodb.ListTablesInput{}); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "detail": "dynamodb unreachable"})
		return
	}
	if _, err := h.sqs.ListQueues(ctx, &sqs.ListQueuesInput{}); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "detail": "sqs unreachable"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// getEvents retorna todos os eventos individuais de um desenvolvedor.
func (h *Handler) getEvents(w http.ResponseWriter, r *http.Request) {
	developerID := r.PathValue("developer_id")

	events, err := h.events.FindByDeveloperID(r.Context(), developerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch events"})
		return
	}

	writeJSON(w, http.StatusOK, events)
}

// getSummary retorna o resumo agregado de um desenvolvedor.
func (h *Handler) getSummary(w http.ResponseWriter, r *http.Request) {
	developerID := r.PathValue("developer_id")

	summary, err := h.summaries.Get(r.Context(), developerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch summary"})
		return
	}
	if summary == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "developer not found"})
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
