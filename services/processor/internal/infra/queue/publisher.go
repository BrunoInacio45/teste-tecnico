package queue

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type Publisher struct {
	client   *sqs.Client
	queueURL string
}

func NewPublisher(client *sqs.Client, queueURL string) *Publisher {
	return &Publisher{client: client, queueURL: queueURL}
}

// Publish envia um corpo de mensagem para a fila, injetando o trace context W3C nos MessageAttributes.
func (p *Publisher) Publish(ctx context.Context, body string) error {
	carrier := make(propagation.MapCarrier)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	msgAttrs := make(map[string]sqstypes.MessageAttributeValue, len(carrier))
	for k, v := range carrier {
		msgAttrs[k] = sqstypes.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(v),
		}
	}

	input := &sqs.SendMessageInput{
		QueueUrl:    aws.String(p.queueURL),
		MessageBody: aws.String(body),
	}
	if len(msgAttrs) > 0 {
		input.MessageAttributes = msgAttrs
	}

	_, err := p.client.SendMessage(ctx, input)
	return err
}
