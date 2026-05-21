package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"aggregator/internal/domain"
)

// SummaryRepository persiste e consulta o resumo agregado por desenvolvedor.
type SummaryRepository struct {
	client    *dynamodb.Client
	tableName string
}

func NewSummaryRepository(client *dynamodb.Client, tableName string) *SummaryRepository {
	return &SummaryRepository{client: client, tableName: tableName}
}

// Get retorna o summary de um desenvolvedor. Retorna nil se ainda não existir.
func (r *SummaryRepository) Get(ctx context.Context, developerID string) (*domain.DeveloperSummary, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"developer_id": &types.AttributeValueMemberS{Value: developerID},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Item) == 0 {
		return nil, nil
	}

	var summary domain.DeveloperSummary
	if err := attributevalue.UnmarshalMap(out.Item, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

// Save persiste o summary de um desenvolvedor (cria ou substitui).
func (r *SummaryRepository) Save(ctx context.Context, summary domain.DeveloperSummary) error {
	item, err := attributevalue.MarshalMap(summary)
	if err != nil {
		return err
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}
