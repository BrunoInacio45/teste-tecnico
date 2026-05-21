package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"processor/internal/domain"
	"processor/internal/infra/config"
	"processor/internal/infra/queue"
	"processor/internal/infra/worker"
	"processor/internal/usecase"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	slog.Info("processor starting",
		"endpoint", cfg.AWSEndpoint,
		"workers", cfg.WorkersCount,
		"processor_id", cfg.ProcessorID,
	)

	sqsClient := newSQSClient(cfg)

	rawQueueURL := cfg.AWSEndpoint + "/000000000000/" + cfg.RawEventsQueue
	processedQueueURL := cfg.AWSEndpoint + "/000000000000/" + cfg.ProcessedEventsQueue

	consumer := queue.NewConsumer(sqsClient, rawQueueURL)
	publisher := queue.NewPublisher(sqsClient, processedQueueURL)

	ctx, cancel := context.WithCancel(context.Background())

	// jobs: canal entre o consumer SQS e os workers
	jobs := make(chan worker.Job, cfg.WorkersCount*2)

	// Goroutine que lê mensagens do SQS e as coloca no canal jobs.
	// Quando o ctx é cancelado, consumer.Start retorna → close(msgs) → close(jobs).
	go func() {
		msgs := make(chan queue.Message, cfg.WorkersCount*2)
		go func() {
			consumer.Start(ctx, msgs)
			close(msgs)
		}()
		for msg := range msgs {
			jobs <- worker.Job{Body: msg.Body, ReceiptHandle: msg.ReceiptHandle}
		}
		close(jobs)
	}()

	// Escuta sinais do SO para graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-quit
		slog.Info("received signal, shutting down", "signal", sig.String())
		cancel()
	}()

	// worker.Start bloqueia até todos os workers terminarem.
	// Os workers param quando ctx é cancelado ou jobs é fechado.
	worker.Start(ctx, cfg.WorkersCount, jobs, func(ctx context.Context, j worker.Job) {
		processJob(ctx, j, consumer, publisher, cfg.ProcessorID)
	})

	slog.Info("processor stopped")
}

// processJob unmarshal, valida, enriquece e publica um único evento.
func processJob(ctx context.Context, j worker.Job, c *queue.Consumer, p *queue.Publisher, processorID string) {
	var raw domain.RawEvent
	if err := json.Unmarshal([]byte(j.Body), &raw); err != nil {
		slog.Error("failed to unmarshal event", "error", err)
		return // deixa na fila para ir ao DLQ
	}

	processed, err := usecase.Process(raw, processorID)
	if err != nil {
		slog.Warn("event validation failed", "event_id", raw.EventID, "error", err)
		c.Delete(ctx, j.ReceiptHandle) // evento inválido: descarta
		return
	}

	body, _ := json.Marshal(processed)

	// Tenta publicar com até 3 tentativas e backoff exponencial (1s, 2s, 4s)
	if err := retry(3, func() error { return p.Publish(ctx, string(body)) }); err != nil {
		slog.Error("failed to publish event", "event_id", processed.EventID, "error", err)
		return // deixa na fila; SQS vai retentar → DLQ
	}

	if err := c.Delete(ctx, j.ReceiptHandle); err != nil {
		slog.Error("failed to delete message", "event_id", processed.EventID, "error", err)
		return
	}

	slog.Info("event processed",
		"service", "processor",
		"event_id", processed.EventID,
		"developer_id", processed.DeveloperID,
		"metric_type", processed.MetricType,
	)
}

// retry executa fn até maxAttempts vezes com backoff exponencial: 1s, 2s, 4s...
func retry(maxAttempts int, fn func() error) error {
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

func newSQSClient(cfg config.Config) *sqs.Client {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.AWSRegion),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("test", "test", ""),
		),
	)
	if err != nil {
		slog.Error("failed to load AWS config", "error", err)
		os.Exit(1)
	}
	// BaseEndpoint aponta para o LocalStack em vez da AWS real
	return sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
	})
}
