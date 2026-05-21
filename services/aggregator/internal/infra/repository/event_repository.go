package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"aggregator/internal/domain"
)

// EventRepository persiste e consulta eventos individuais no DynamoDB.
type EventRepository struct {
	client    *dynamodb.Client
	tableName string
}

func NewEventRepository(client *dynamodb.Client, tableName string) *EventRepository {
	return &EventRepository{client: client, tableName: tableName}
}

// Exists verifica se um evento com esse event_id já foi processado.
// Usado para garantir idempotência: o mesmo evento não é agregado duas vezes.
func (r *EventRepository) Exists(ctx context.Context, eventID string) (bool, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"event_id": &types.AttributeValueMemberS{Value: eventID},
		},
	})
	if err != nil {
		return false, err
	}
	return len(out.Item) > 0, nil
}

// Save persiste um evento processado na tabela events.
func (r *EventRepository) Save(ctx context.Context, event domain.ProcessedEvent) error {
	item, err := attributevalue.MarshalMap(event)
	if err != nil {
		return err
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

// FindByDeveloperID retorna todos os eventos de um desenvolvedor.
// Usa Scan com filtro — simples e direto, sem preocupação de performance aqui.
func (r *EventRepository) FindByDeveloperID(ctx context.Context, developerID string) ([]domain.ProcessedEvent, error) {
	out, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("developer_id = :dev"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":dev": &types.AttributeValueMemberS{Value: developerID},
		},
	})
	if err != nil {
		return nil, err
	}

	var events []domain.ProcessedEvent
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &events); err != nil {
		return nil, err
	}
	return events, nil
}
