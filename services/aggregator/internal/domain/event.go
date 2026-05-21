package domain

import "time"

// ProcessedEvent espelha o contrato publicado pelo processor.
type ProcessedEvent struct {
	EventID     string    `json:"event_id"     dynamodbav:"event_id"`
	DeveloperID string    `json:"developer_id" dynamodbav:"developer_id"`
	MetricType  string    `json:"metric_type"  dynamodbav:"metric_type"`
	Value       float64   `json:"value"        dynamodbav:"value"`
	Repository  string    `json:"repository"   dynamodbav:"repository"`
	Timestamp   time.Time `json:"timestamp"    dynamodbav:"timestamp"`
	ProcessedAt time.Time `json:"processed_at" dynamodbav:"processed_at"`
	ProcessorID string    `json:"processor_id" dynamodbav:"processor_id"`
}

// DeveloperSummary mantém as métricas agregadas de um desenvolvedor.
type DeveloperSummary struct {
	DeveloperID          string    `json:"developer_id"            dynamodbav:"developer_id"`
	TotalCommits         int64     `json:"total_commits"           dynamodbav:"total_commits"`
	TotalPullRequests    int64     `json:"total_pull_requests"     dynamodbav:"total_pull_requests"`
	AvgReviewTimeMinutes float64   `json:"avg_review_time_minutes" dynamodbav:"avg_review_time_minutes"`
	EventsProcessed      int64     `json:"events_processed"        dynamodbav:"events_processed"`
	LastActivity         time.Time `json:"last_activity"           dynamodbav:"last_activity"`

	// Campos auxiliares para recalcular a média corretamente.
	// json:"-" os exclui da resposta da API; dynamodbav os persiste no DynamoDB.
	ReviewTimeTotal float64 `json:"-" dynamodbav:"review_time_total"`
	ReviewTimeCount int64   `json:"-" dynamodbav:"review_time_count"`
}

// Apply aplica um evento ao summary, atualizando os totais incrementalmente.
func (s *DeveloperSummary) Apply(event ProcessedEvent) {
	s.EventsProcessed++

	// Mantém o timestamp mais recente como última atividade
	if s.LastActivity.IsZero() || event.Timestamp.After(s.LastActivity) {
		s.LastActivity = event.Timestamp
	}

	switch event.MetricType {
	case "commits":
		s.TotalCommits += int64(event.Value)

	case "pull_requests":
		s.TotalPullRequests += int64(event.Value)

	case "review_time_minutes":
		// Recalcula a média acumulando total e contagem
		s.ReviewTimeTotal += event.Value
		s.ReviewTimeCount++
		s.AvgReviewTimeMinutes = s.ReviewTimeTotal / float64(s.ReviewTimeCount)
	}
}
