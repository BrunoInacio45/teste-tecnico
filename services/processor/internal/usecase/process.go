package usecase

import (
	"time"

	"processor/internal/domain"
)

// Process valida um RawEvent e retorna um ProcessedEvent enriquecido com metadados.
func Process(raw domain.RawEvent, processorID string) (domain.ProcessedEvent, error) {
	if err := domain.Validate(raw); err != nil {
		return domain.ProcessedEvent{}, err
	}
	return domain.ProcessedEvent{
		EventID:     raw.EventID,
		DeveloperID: raw.DeveloperID,
		MetricType:  raw.MetricType,
		Value:       raw.Value,
		Repository:  raw.Repository,
		Timestamp:   raw.Timestamp,
		ProcessedAt: time.Now().UTC(),
		ProcessorID: processorID,
	}, nil
}
