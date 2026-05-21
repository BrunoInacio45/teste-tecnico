package domain

import "time"

// ProcessedEvent espelha o contrato publicado pelo processor.
type ProcessedEvent struct {
	EventID     string    `json:"event_id"`
	DeveloperID string    `json:"developer_id"`
	MetricType  string    `json:"metric_type"`
	Value       float64   `json:"value"`
	Repository  string    `json:"repository"`
	Timestamp   time.Time `json:"timestamp"`
	ProcessedAt time.Time `json:"processed_at"`
	ProcessorID string    `json:"processor_id"`
}
