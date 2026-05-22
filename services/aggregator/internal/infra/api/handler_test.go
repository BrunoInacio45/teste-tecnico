package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"aggregator/internal/domain"
	"aggregator/internal/infra/api"
)

// --- mock repositories ---

type mockEventReader struct {
	events []domain.ProcessedEvent
	err    error
}

func (m *mockEventReader) FindByDeveloperID(_ context.Context, _ string) ([]domain.ProcessedEvent, error) {
	return m.events, m.err
}

type mockSummaryReader struct {
	summary *domain.DeveloperSummary
	err     error
}

func (m *mockSummaryReader) Get(_ context.Context, _ string) (*domain.DeveloperSummary, error) {
	return m.summary, m.err
}

// --- AWS mock transport for health endpoint ---

// mockAWSTransport intercepts AWS SDK HTTP calls and returns configurable responses.
// Both DynamoDB and SQS (sdk v2) use X-Amz-Target, so we distinguish them by URL host:
// dynamodb.*.amazonaws.com vs sqs.*.amazonaws.com.
type mockAWSTransport struct {
	dynamoErr error
	sqsErr    error
}

func (m *mockAWSTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	isDynamo := strings.Contains(r.URL.Host, "dynamodb")

	if isDynamo {
		if m.dynamoErr != nil {
			return nil, m.dynamoErr
		}
		hdr := make(http.Header)
		hdr.Set("Content-Type", "application/x-amz-json-1.0")
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Header:     hdr,
		}, nil
	}

	// SQS request (aws-sdk-go-v2 SQS uses JSON protocol)
	if m.sqsErr != nil {
		return nil, m.sqsErr
	}
	hdr := make(http.Header)
	hdr.Set("Content-Type", "application/x-amz-json-1.0")
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(`{"QueueUrls":[]}`)),
		Header:     hdr,
	}, nil
}

func newMockAWSClients(transport http.RoundTripper) (*dynamodb.Client, *sqs.Client) {
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""),
		HTTPClient:  &http.Client{Transport: transport},
	}
	return dynamodb.NewFromConfig(cfg), sqs.NewFromConfig(cfg)
}

// --- helpers ---

func newRouter(events *mockEventReader, summaries *mockSummaryReader) http.Handler {
	// nil AWS clients are safe for endpoints that don't touch them (getEvents, getSummary)
	return api.NewRouter(events, summaries, nil, nil)
}

func decodeJSON(t *testing.T, body io.Reader, v any) {
	t.Helper()
	if err := json.NewDecoder(body).Decode(v); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
}

// --- /metrics/{developer_id} tests ---

func TestGetEvents_ReturnsList(t *testing.T) {
	ts := time.Now().Add(-time.Hour)
	events := []domain.ProcessedEvent{
		{EventID: "e1", DeveloperID: "dev-1", MetricType: "commits", Value: 5, Timestamp: ts},
		{EventID: "e2", DeveloperID: "dev-1", MetricType: "pull_requests", Value: 2, Timestamp: ts},
	}

	router := newRouter(&mockEventReader{events: events}, &mockSummaryReader{})
	req := httptest.NewRequest(http.MethodGet, "/metrics/dev-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got []domain.ProcessedEvent
	decodeJSON(t, rec.Body, &got)

	if len(got) != 2 {
		t.Errorf("expected 2 events, got %d", len(got))
	}
}

func TestGetEvents_EmptyList_Returns200(t *testing.T) {
	router := newRouter(&mockEventReader{events: []domain.ProcessedEvent{}}, &mockSummaryReader{})
	req := httptest.NewRequest(http.MethodGet, "/metrics/dev-unknown", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetEvents_RepositoryError_Returns500(t *testing.T) {
	reader := &mockEventReader{err: errors.New("db error")}
	router := newRouter(reader, &mockSummaryReader{})
	req := httptest.NewRequest(http.MethodGet, "/metrics/dev-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestGetEvents_ContentTypeIsJSON(t *testing.T) {
	router := newRouter(&mockEventReader{events: []domain.ProcessedEvent{}}, &mockSummaryReader{})
	req := httptest.NewRequest(http.MethodGet, "/metrics/dev-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
}

// --- /metrics/{developer_id}/summary tests ---

func TestGetSummary_Found_Returns200WithBody(t *testing.T) {
	summary := &domain.DeveloperSummary{
		DeveloperID:          "dev-1",
		TotalCommits:         42,
		TotalPullRequests:    10,
		AvgReviewTimeMinutes: 30.5,
		EventsProcessed:      52,
		LastActivity:         time.Now().Add(-time.Hour),
	}

	router := newRouter(&mockEventReader{}, &mockSummaryReader{summary: summary})
	req := httptest.NewRequest(http.MethodGet, "/metrics/dev-1/summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got domain.DeveloperSummary
	decodeJSON(t, rec.Body, &got)

	if got.DeveloperID != "dev-1" {
		t.Errorf("DeveloperID: got %q, want %q", got.DeveloperID, "dev-1")
	}
	if got.TotalCommits != 42 {
		t.Errorf("TotalCommits: got %d, want 42", got.TotalCommits)
	}
	if got.TotalPullRequests != 10 {
		t.Errorf("TotalPullRequests: got %d, want 10", got.TotalPullRequests)
	}
}

func TestGetSummary_NotFound_Returns404(t *testing.T) {
	router := newRouter(&mockEventReader{}, &mockSummaryReader{summary: nil})
	req := httptest.NewRequest(http.MethodGet, "/metrics/dev-unknown/summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestGetSummary_RepositoryError_Returns500(t *testing.T) {
	reader := &mockSummaryReader{err: errors.New("db error")}
	router := newRouter(&mockEventReader{}, reader)
	req := httptest.NewRequest(http.MethodGet, "/metrics/dev-1/summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestGetSummary_AuxFieldsHiddenFromJSON(t *testing.T) {
	summary := &domain.DeveloperSummary{
		DeveloperID:         "dev-1",
		ReviewTimeTotal:     300,
		ReviewTimeCount:     5,
		AvgReviewTimeMinutes: 60,
	}

	router := newRouter(&mockEventReader{}, &mockSummaryReader{summary: summary})
	req := httptest.NewRequest(http.MethodGet, "/metrics/dev-1/summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "review_time_total") {
		t.Error("review_time_total must not appear in JSON response (json:\"-\")")
	}
	if strings.Contains(body, "review_time_count") {
		t.Error("review_time_count must not appear in JSON response (json:\"-\")")
	}
}

// --- /health tests ---

func TestHealth_AllServicesUp_Returns200(t *testing.T) {
	transport := &mockAWSTransport{}
	dynamo, sqsCli := newMockAWSClients(transport)

	router := api.NewRouter(&mockEventReader{}, &mockSummaryReader{}, sqsCli, dynamo)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHealth_DynamoUnreachable_Returns503(t *testing.T) {
	transport := &mockAWSTransport{dynamoErr: errors.New("connection refused")}
	dynamo, sqsCli := newMockAWSClients(transport)

	router := api.NewRouter(&mockEventReader{}, &mockSummaryReader{}, sqsCli, dynamo)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}

	var body map[string]string
	decodeJSON(t, rec.Body, &body)
	if body["status"] != "error" {
		t.Errorf("expected status=error, got %q", body["status"])
	}
}

func TestHealth_SQSUnreachable_Returns503(t *testing.T) {
	transport := &mockAWSTransport{sqsErr: errors.New("connection refused")}
	dynamo, sqsCli := newMockAWSClients(transport)

	router := api.NewRouter(&mockEventReader{}, &mockSummaryReader{}, sqsCli, dynamo)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}
