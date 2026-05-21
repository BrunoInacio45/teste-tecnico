package usecase

import (
	"context"
	"log/slog"

	"aggregator/internal/domain"
)

type EventRepository interface {
	Exists(ctx context.Context, eventID string) (bool, error)
	Save(ctx context.Context, event domain.ProcessedEvent) error
}

type SummaryRepository interface {
	Get(ctx context.Context, developerID string) (*domain.DeveloperSummary, error)
	Save(ctx context.Context, summary domain.DeveloperSummary) error
}

// AggregateUseCase orquestra o processamento de cada evento recebido:
// 1. verifica idempotência
// 2. persiste o evento individual
// 3. atualiza o summary do desenvolvedor
type AggregateUseCase struct {
	events    EventRepository
	summaries SummaryRepository
}

func NewAggregateUseCase(events EventRepository, summaries SummaryRepository) *AggregateUseCase {
	return &AggregateUseCase{events: events, summaries: summaries}
}

func (uc *AggregateUseCase) Process(ctx context.Context, event domain.ProcessedEvent) error {
	// Passo 1: idempotência — ignora evento já processado
	exists, err := uc.events.Exists(ctx, event.EventID)
	if err != nil {
		return err
	}
	if exists {
		slog.Info("duplicate event ignored",
			"service", "aggregator",
			"event_id", event.EventID,
		)
		return nil
	}

	// Passo 2: persiste o evento individual
	if err := uc.events.Save(ctx, event); err != nil {
		return err
	}
	slog.Info("event persisted",
		"service", "aggregator",
		"event_id", event.EventID,
	)

	// Passo 3: carrega o summary existente ou cria um novo
	summary, err := uc.summaries.Get(ctx, event.DeveloperID)
	if err != nil {
		return err
	}
	if summary == nil {
		summary = &domain.DeveloperSummary{DeveloperID: event.DeveloperID}
	}

	// Aplica a agregação incremental e salva
	summary.Apply(event)
	return uc.summaries.Save(ctx, *summary)
}
