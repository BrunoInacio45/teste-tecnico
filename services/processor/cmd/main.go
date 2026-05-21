package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

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
	orchestrator := usecase.NewOrchestrator(publisher, consumer, cfg.ProcessorID)

	ctx, cancel := context.WithCancel(context.Background())

	jobs := make(chan worker.Job, cfg.WorkersCount*2)

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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-quit
		slog.Info("received signal, shutting down", "signal", sig.String())
		cancel()
	}()

	worker.Start(ctx, cfg.WorkersCount, jobs, func(ctx context.Context, j worker.Job) {
		orchestrator.Execute(ctx, j.Body, j.ReceiptHandle)
	})

	slog.Info("processor stopped")
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
