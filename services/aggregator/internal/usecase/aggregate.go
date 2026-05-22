package usecase

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"aggregator/internal/domain"
)

var tracer = otel.Tracer("aggregator")

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
	ctx, span := tracer.Start(ctx, "aggregate.process")
	defer span.End()

	span.SetAttributes(
		attribute.String("event.id", event.EventID),
		attribute.String("event.developer_id", event.DeveloperID),
		attribute.String("event.metric_type", event.MetricType),
	)

	// Passo 1: idempotência — ignora evento já processado
	exists, err := uc.events.Exists(ctx, event.EventID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "exists check failed")
		return err
	}
	if exists {
		slog.Info("duplicate event ignored",
			"service", "aggregator",
			"event_id", event.EventID,
		)
		span.SetStatus(codes.Ok, "duplicate ignored")
		return nil
	}

	// Passo 2: persiste o evento individual
	if err := uc.events.Save(ctx, event); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "save event failed")
		return err
	}
	slog.Info("event persisted",
		"service", "aggregator",
		"event_id", event.EventID,
	)

	// Passo 3: carrega o summary existente ou cria um novo
	summary, err := uc.summaries.Get(ctx, event.DeveloperID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "get summary failed")
		return err
	}
	if summary == nil {
		summary = &domain.DeveloperSummary{DeveloperID: event.DeveloperID}
	}

	// Aplica a agregação incremental e salva
	summary.Apply(event)
	if err := uc.summaries.Save(ctx, *summary); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "save summary failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
