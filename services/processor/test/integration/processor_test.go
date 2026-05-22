//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"processor/internal/domain"
	"processor/internal/infra/queue"
	"processor/internal/infra/worker"
	"processor/internal/usecase"
)

func validRawEvent(devID string) domain.RawEvent {
	return domain.RawEvent{
		EventID:     nextUUID(),
		DeveloperID: devID,
		MetricType:  "commits",
		Value:       5,
		Repository:  "test-repo",
		Timestamp:   time.Now().Add(-1 * time.Hour).UTC(),
	}
}

func newOrchestrator() *usecase.Orchestrator {
	publisher := queue.NewPublisher(sqsClient, procQueueURL)
	consumer := queue.NewConsumer(sqsClient, rawQueueURL)
	return usecase.NewOrchestrator(publisher, consumer, processorID)
}

// drainQueue consome e descarta todas as mensagens visíveis de uma fila.
// Garante estado limpo entre testes sem depender de PurgeQueue.
func drainQueue(ctx context.Context, queueURL string) {
	for {
		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     1,
		})
		if err != nil || len(out.Messages) == 0 {
			return
		}
		for _, msg := range out.Messages {
			sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueURL),
				ReceiptHandle: msg.ReceiptHandle,
			})
		}
	}
}

// receiveOne consome uma mensagem da fila, falhando o teste se nenhuma chegar.
func receiveOne(t *testing.T, ctx context.Context, queueURL string) sqstypes.Message {
	t.Helper()
	out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     5,
	})
	if err != nil {
		t.Fatalf("ReceiveMessage: %v", err)
	}
	if len(out.Messages) == 0 {
		t.Fatal("nenhuma mensagem recebida da fila")
	}
	return out.Messages[0]
}

// TestOrchestrator_ValidEvent_PublishesToProcessedQueue verifica o fluxo completo:
// evento válido recebido do SQS → validado → enriquecido → publicado em processed-events.
func TestOrchestrator_ValidEvent_PublishesToProcessedQueue(t *testing.T) {
	ctx := context.Background()
	drainQueue(ctx, procQueueURL)

	raw := validRawEvent("dev-" + t.Name())
	body, _ := json.Marshal(raw)

	if _, err := sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(rawQueueURL),
		MessageBody: aws.String(string(body)),
	}); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	// Consome da fila de entrada (simula o consumer do processor)
	msg := receiveOne(t, ctx, rawQueueURL)

	newOrchestrator().Execute(ctx, aws.ToString(msg.Body), aws.ToString(msg.ReceiptHandle))

	// Verifica publicação em processed-events
	procMsg := receiveOne(t, ctx, procQueueURL)
	defer sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(procQueueURL),
		ReceiptHandle: procMsg.ReceiptHandle,
	})

	var processed domain.ProcessedEvent
	if err := json.Unmarshal([]byte(aws.ToString(procMsg.Body)), &processed); err != nil {
		t.Fatalf("unmarshal processed event: %v", err)
	}

	if processed.EventID != raw.EventID {
		t.Errorf("EventID = %q, want %q", processed.EventID, raw.EventID)
	}
	if processed.ProcessorID != processorID {
		t.Errorf("ProcessorID = %q, want %q", processed.ProcessorID, processorID)
	}
	if processed.ProcessedAt.IsZero() {
		t.Error("ProcessedAt não deve ser zero")
	}
}

// TestOrchestrator_InvalidEvent_NotPublished verifica que eventos que violam as
// regras de validação não são publicados em processed-events.
func TestOrchestrator_InvalidEvent_NotPublished(t *testing.T) {
	ctx := context.Background()
	drainQueue(ctx, procQueueURL)

	invalid := domain.RawEvent{
		EventID:     nextUUID(),
		DeveloperID: "", // campo obrigatório vazio → inválido
		MetricType:  "commits",
		Value:       5,
		Timestamp:   time.Now().Add(-1 * time.Hour).UTC(),
	}
	body, _ := json.Marshal(invalid)

	// Execute com receipt handle qualquer (o Delete não é chamado em caso de falha)
	newOrchestrator().Execute(ctx, string(body), "receipt-handle-invalido")

	// Nada deve ter sido publicado
	out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(procQueueURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     2,
	})
	if err != nil {
		t.Fatalf("ReceiveMessage: %v", err)
	}
	if len(out.Messages) != 0 {
		t.Error("evento inválido não deve ser publicado em processed-events")
	}
}

// TestOrchestrator_EnrichesEventMetadata verifica que o processor enriquece
// corretamente o evento com ProcessedAt e ProcessorID, preservando os campos originais.
func TestOrchestrator_EnrichesEventMetadata(t *testing.T) {
	ctx := context.Background()
	drainQueue(ctx, procQueueURL)

	raw := validRawEvent("dev-" + t.Name())
	raw.MetricType = "review_time_minutes"
	raw.Value = 45
	body, _ := json.Marshal(raw)

	if _, err := sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(rawQueueURL),
		MessageBody: aws.String(string(body)),
	}); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	msg := receiveOne(t, ctx, rawQueueURL)

	before := time.Now().UTC()
	newOrchestrator().Execute(ctx, aws.ToString(msg.Body), aws.ToString(msg.ReceiptHandle))
	after := time.Now().UTC()

	procMsg := receiveOne(t, ctx, procQueueURL)
	defer sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(procQueueURL),
		ReceiptHandle: procMsg.ReceiptHandle,
	})

	var processed domain.ProcessedEvent
	if err := json.Unmarshal([]byte(aws.ToString(procMsg.Body)), &processed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Campos originais preservados
	if processed.DeveloperID != raw.DeveloperID {
		t.Errorf("DeveloperID = %q, want %q", processed.DeveloperID, raw.DeveloperID)
	}
	if processed.Value != raw.Value {
		t.Errorf("Value = %v, want %v", processed.Value, raw.Value)
	}
	if processed.MetricType != raw.MetricType {
		t.Errorf("MetricType = %q, want %q", processed.MetricType, raw.MetricType)
	}

	// Campos de enriquecimento
	if processed.ProcessorID != processorID {
		t.Errorf("ProcessorID = %q, want %q", processed.ProcessorID, processorID)
	}
	if processed.ProcessedAt.Before(before) || processed.ProcessedAt.After(after) {
		t.Errorf("ProcessedAt %v deve estar entre %v e %v", processed.ProcessedAt, before, after)
	}
}

// TestWorkerPool_ProcessesMultipleEventsConcurrently verifica que o worker pool
// processa N eventos em paralelo: publica na fila de entrada, consome com receipt
// handles reais e confirma que todos chegam à fila processed-events.
func TestWorkerPool_ProcessesMultipleEventsConcurrently(t *testing.T) {
	const N = 10
	ctx := context.Background()
	drainQueue(ctx, rawQueueURL)
	drainQueue(ctx, procQueueURL)

	// Publica N eventos na fila de entrada
	for i := 0; i < N; i++ {
		raw := validRawEvent(fmt.Sprintf("dev-pool-%d", i))
		body, _ := json.Marshal(raw)
		if _, err := sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:    aws.String(rawQueueURL),
			MessageBody: aws.String(string(body)),
		}); err != nil {
			t.Fatalf("SendMessage %d: %v", i, err)
		}
	}

	// Consome todos os N eventos da fila (receipt handles reais)
	jobs := make(chan worker.Job, N)
	collected := 0
	deadline := time.Now().Add(15 * time.Second)
	for collected < N && time.Now().Before(deadline) {
		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(rawQueueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     5,
		})
		if err != nil {
			t.Fatalf("ReceiveMessage (input): %v", err)
		}
		for _, msg := range out.Messages {
			jobs <- worker.Job{
				Body:          aws.ToString(msg.Body),
				ReceiptHandle: aws.ToString(msg.ReceiptHandle),
			}
			collected++
		}
	}
	close(jobs)

	if collected != N {
		t.Fatalf("só recebeu %d/%d mensagens da fila de entrada", collected, N)
	}

	poolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	orch := newOrchestrator()
	worker.Start(poolCtx, 3, jobs, func(ctx context.Context, j worker.Job) {
		orch.Execute(ctx, j.Body, j.ReceiptHandle)
	})

	// Verifica que todos os N eventos chegaram em processed-events
	processed := 0
	deadline = time.Now().Add(15 * time.Second)
	for processed < N && time.Now().Before(deadline) {
		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(procQueueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     2,
		})
		if err != nil {
			t.Fatalf("ReceiveMessage (output): %v", err)
		}
		for _, msg := range out.Messages {
			sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(procQueueURL),
				ReceiptHandle: msg.ReceiptHandle,
			})
			processed++
		}
	}

	if processed != N {
		t.Errorf("worker pool entregou %d eventos em processed-events, esperado %d", processed, N)
	}
}
