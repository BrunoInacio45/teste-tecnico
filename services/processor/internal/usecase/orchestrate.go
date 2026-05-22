package usecase

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"processor/internal/domain"
)

var tracer = otel.Tracer("processor")

// Publisher envia mensagens para uma fila SQS.
type Publisher interface {
	Publish(ctx context.Context, body string) error
}

// Acker confirma o processamento de uma mensagem removendo-a da fila.
type Acker interface {
	Delete(ctx context.Context, receiptHandle string) error
}

// Orchestrator coordena validação, enriquecimento e publicação de um evento.
type Orchestrator struct {
	publisher   Publisher
	acker       Acker
	processorID string
}

func NewOrchestrator(publisher Publisher, acker Acker, processorID string) *Orchestrator {
	return &Orchestrator{
		publisher:   publisher,
		acker:       acker,
		processorID: processorID,
	}
}

// Execute processa uma mensagem SQS: faz unmarshal, valida, enriquece e publica.
// Eventos com erro (unmarshal, validação ou falha de publicação) não são deletados —
// o SQS os reenvia e, após maxReceiveCount tentativas, move para a DLQ via RedrivePolicy.
func (o *Orchestrator) Execute(ctx context.Context, body, receiptHandle string) {
	ctx, span := tracer.Start(ctx, "orchestrator.execute")
	defer span.End()

	var raw domain.RawEvent
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "unmarshal failed")
		slog.Error("failed to unmarshal event", "error", err)
		return // não deleta: SQS reenvia → DLQ
	}

	span.SetAttributes(
		attribute.String("event.id", raw.EventID),
		attribute.String("event.developer_id", raw.DeveloperID),
		attribute.String("event.metric_type", raw.MetricType),
	)

	processed, err := Process(raw, o.processorID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "validation failed")
		slog.Warn("event validation failed", "event_id", raw.EventID, "error", err)
		return // não deleta: SQS reenvia → DLQ
	}

	msgBody, _ := json.Marshal(processed)

	if err := retryWithBackoff(3, func() error {
		return o.publisher.Publish(ctx, string(msgBody))
	}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish failed")
		slog.Error("failed to publish event", "event_id", processed.EventID, "error", err)
		return // não deleta: SQS reenvia → DLQ
	}

	if err := o.acker.Delete(ctx, receiptHandle); err != nil {
		span.RecordError(err)
		slog.Error("failed to delete message", "event_id", processed.EventID, "error", err)
		return
	}

	span.SetStatus(codes.Ok, "")
	slog.Info("event processed",
		"service", "processor",
		"event_id", processed.EventID,
		"developer_id", processed.DeveloperID,
		"metric_type", processed.MetricType,
	)
}

// retryWithBackoff executa fn até maxAttempts vezes com backoff exponencial: 1s, 2s, 4s…
func retryWithBackoff(maxAttempts int, fn func() error) error {
	var err error
	for i := 0; i < maxAttempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i < maxAttempts-1 {
			time.Sleep(time.Duration(1<<uint(i)) * time.Second)
		}
	}
	return err
}
