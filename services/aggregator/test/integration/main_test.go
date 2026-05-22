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
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	eventsTable    = "events"
	summaryTable   = "developer_summary"
	processedQueue = "processed-events"
)

var (
	awsEndpoint  string
	dynamoClient *dynamodb.Client
	sqsClient    *sqs.Client

	eventCounter atomic.Int64
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "localstack/localstack:3.4",
			ExposedPorts: []string{"4566/tcp"},
			Env:          map[string]string{"SERVICES": "sqs,dynamodb"},
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
	awsEndpoint = fmt.Sprintf("http://%s:%s", host, port.Port())

	awsCfg, _ := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("test", "test", ""),
		),
	)
	dynamoClient = dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(awsEndpoint)
	})
	sqsClient = sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(awsEndpoint)
	})

	if err := bootstrapInfra(ctx); err != nil {
		log.Fatalf("bootstrap infra: %v", err)
	}

	os.Exit(m.Run())
}

func bootstrapInfra(ctx context.Context) error {
	tables := []struct{ name, key string }{
		{eventsTable, "event_id"},
		{summaryTable, "developer_id"},
	}
	for _, tbl := range tables {
		if _, err := dynamoClient.CreateTable(ctx, &dynamodb.CreateTableInput{
			TableName: aws.String(tbl.name),
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String(tbl.key), AttributeType: types.ScalarAttributeTypeS},
			},
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String(tbl.key), KeyType: types.KeyTypeHash},
			},
			BillingMode: types.BillingModePayPerRequest,
		}); err != nil {
			return fmt.Errorf("create table %s: %w", tbl.name, err)
		}
	}

	if _, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(processedQueue),
	}); err != nil {
		return fmt.Errorf("create queue %s: %w", processedQueue, err)
	}
	return nil
}

// nextEventID gera um event_id único com formato compatível com o domínio.
func nextEventID() string {
	n := eventCounter.Add(1)
	return fmt.Sprintf("550e8400-e29b-41d4-a716-%012x", n)
}
