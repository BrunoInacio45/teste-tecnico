package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"aggregator/internal/domain"
	"aggregator/internal/infra/api"
	"aggregator/internal/infra/config"
	"aggregator/internal/infra/queue"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	slog.Info("aggregator starting",
		"endpoint", cfg.AWSEndpoint,
		"port", cfg.Port,
	)

	sqsClient := newSQSClient(cfg)
	processedQueueURL := cfg.AWSEndpoint + "/000000000000/" + cfg.ProcessedEventsQueue
	consumer := queue.NewConsumer(sqsClient, processedQueueURL)

	ctx, cancel := context.WithCancel(context.Background())

	// Inicia o consumer SQS em background
	msgs := make(chan queue.Message, 20)
	go func() {
		consumer.Start(ctx, msgs)
		close(msgs)
	}()

	// Processa mensagens recebidas (apenas log nesta etapa)
	go func() {
		for msg := range msgs {
			handleMessage(ctx, consumer, msg)
		}
	}()

	// Inicia o servidor HTTP
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: api.NewRouter(),
	}
	go func() {
		slog.Info("aggregator listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	// Graceful shutdown ao receber sinal do SO
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("received signal, shutting down", "signal", sig.String())
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("aggregator stopped")
}

// handleMessage faz o unmarshal do evento recebido, loga e deleta da fila.
// Nesta etapa não há persistência — apenas log.
func handleMessage(ctx context.Context, c *queue.Consumer, msg queue.Message) {
	var event domain.ProcessedEvent
	if err := json.Unmarshal([]byte(msg.Body), &event); err != nil {
		slog.Error("failed to unmarshal processed event", "error", err)
		return
	}

	slog.Info("event received",
		"service", "aggregator",
		"event_id", event.EventID,
		"developer_id", event.DeveloperID,
		"metric_type", event.MetricType,
		"value", event.Value,
		"processor_id", event.ProcessorID,
	)

	if err := c.Delete(ctx, msg.ReceiptHandle); err != nil {
		slog.Error("failed to delete message", "event_id", event.EventID, "error", err)
	}
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
	return sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
	})
}
