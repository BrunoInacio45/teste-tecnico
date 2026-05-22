//go:build integration

package integration

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	rawQueue       = "raw-events"
	processedQueue = "processed-events"
	processorID    = "test-processor"
)

var (
	sqsClient    *sqs.Client
	rawQueueURL  string
	procQueueURL string

	eventCounter atomic.Int64
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "localstack/localstack:3.4",
			ExposedPorts: []string{"4566/tcp"},
			Env:          map[string]string{"SERVICES": "sqs"},
			WaitingFor:   wait.ForHTTP("/_localstack/health").WithPort("4566/tcp"),
		},
		Started: true,
	})
	if err != nil {
		log.Fatalf("start localstack: %v", err)
	}
	defer container.Terminate(ctx)

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "4566")
	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())

	awsCfg, _ := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("test", "test", ""),
		),
	)
	sqsClient = sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	rawQueueURL, procQueueURL, err = createQueues(ctx)
	if err != nil {
		log.Fatalf("create queues: %v", err)
	}

	os.Exit(m.Run())
}

func createQueues(ctx context.Context) (rawURL, procURL string, err error) {
	for _, q := range []string{rawQueue, processedQueue} {
		out, e := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{
			QueueName: aws.String(q),
		})
		if e != nil {
			return "", "", fmt.Errorf("create queue %s: %w", q, e)
		}
		switch q {
		case rawQueue:
			rawURL = aws.ToString(out.QueueUrl)
		case processedQueue:
			procURL = aws.ToString(out.QueueUrl)
		}
	}
	return rawURL, procURL, nil
}

// nextUUID gera um UUID v4 válido e único para uso como event_id nos testes.
// Formato: 550e8400-e29b-41d4-a716-<counter em hex>
// - Grupo 3 começa com "4" e grupo 4 começa com "a": satisfaz o regex UUID v4.
func nextUUID() string {
	n := eventCounter.Add(1)
	return fmt.Sprintf("550e8400-e29b-41d4-a716-%012x", n)
}
