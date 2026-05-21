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
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"aggregator/internal/domain"
	"aggregator/internal/infra/api"
	"aggregator/internal/infra/config"
	"aggregator/internal/infra/queue"
	"aggregator/internal/infra/repository"
	"aggregator/internal/usecase"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	slog.Info("aggregator starting",
		"endpoint", cfg.AWSEndpoint,
		"port", cfg.Port,
	)

	// Cria um único aws.Config compartilhado por SQS e DynamoDB
	awsCfg := newAWSConfig(cfg)

	sqsClient := sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
	})
	dynamoClient := dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
	})

	// Repositórios e caso de uso
	eventRepo := repository.NewEventRepository(dynamoClient, cfg.EventsTable)
	summaryRepo := repository.NewSummaryRepository(dynamoClient, cfg.SummaryTable)
	aggregateUC := usecase.NewAggregateUseCase(eventRepo, summaryRepo)

	// Consumer SQS
	processedQueueURL := cfg.AWSEndpoint + "/000000000000/" + cfg.ProcessedEventsQueue
	consumer := queue.NewConsumer(sqsClient, processedQueueURL)

	ctx, cancel := context.WithCancel(context.Background())

	msgs := make(chan queue.Message, 20)
	go func() {
		consumer.Start(ctx, msgs)
		close(msgs)
	}()

	go func() {
		for msg := range msgs {
			handleMessage(ctx, consumer, aggregateUC, msg)
		}
	}()

	// Servidor HTTP com os endpoints de métricas
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: api.NewRouter(eventRepo, summaryRepo, sqsClient, dynamoClient),
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

// handleMessage faz unmarshal do evento, chama o usecase e deleta da fila.
func handleMessage(ctx context.Context, c *queue.Consumer, uc *usecase.AggregateUseCase, msg queue.Message) {
	var event domain.ProcessedEvent
	if err := json.Unmarshal([]byte(msg.Body), &event); err != nil {
		slog.Error("failed to unmarshal processed event", "error", err)
		return
	}

	slog.Info("received event",
		"service", "aggregator",
		"event_id", event.EventID,
		"developer_id", event.DeveloperID,
		"metric_type", event.MetricType,
	)

	if err := uc.Process(ctx, event); err != nil {
		slog.Error("failed to process event",
			"service", "aggregator",
			"event_id", event.EventID,
			"error", err,
		)
		return
	}

	if err := c.Delete(ctx, msg.ReceiptHandle); err != nil {
		slog.Error("failed to delete message", "event_id", event.EventID, "error", err)
	}
}

func newAWSConfig(cfg config.Config) aws.Config {
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
	return awsCfg
}
