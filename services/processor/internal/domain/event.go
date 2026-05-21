package domain

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

// uuidV4Regex valida o formato UUID v4.
var uuidV4Regex = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

var validMetricTypes = map[string]bool{
	"commits":             true,
	"pull_requests":       true,
	"review_time_minutes": true,
}

// RawEvent é o evento recebido da fila raw-events.
type RawEvent struct {
	EventID     string    `json:"event_id"`
	DeveloperID string    `json:"developer_id"`
	MetricType  string    `json:"metric_type"`
	Value       float64   `json:"value"`
	Repository  string    `json:"repository"`
	Timestamp   time.Time `json:"timestamp"`
}

// ProcessedEvent é o evento enriquecido publicado em processed-events.
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

// Validate verifica se um RawEvent atende todas as regras de negócio.
func Validate(e RawEvent) error {
	if !uuidV4Regex.MatchString(e.EventID) {
		return errors.New("event_id must be a valid UUID v4")
	}
	if e.DeveloperID == "" {
		return errors.New("developer_id is required")
	}
	if !validMetricTypes[e.MetricType] {
		return fmt.Errorf("metric_type %q is invalid; allowed: commits, pull_requests, review_time_minutes", e.MetricType)
	}
	if e.Value < 0 {
		return errors.New("value must be >= 0")
	}
	if e.MetricType == "review_time_minutes" && e.Value > 1440 {
		return errors.New("review_time_minutes cannot exceed 1440 (24h)")
	}
	if e.Timestamp.IsZero() {
		return errors.New("timestamp is required")
	}
	if e.Timestamp.After(time.Now()) {
		return errors.New("timestamp cannot be in the future")
	}
	return nil
}
