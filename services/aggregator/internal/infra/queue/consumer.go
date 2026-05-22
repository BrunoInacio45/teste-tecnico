package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// Message contém o corpo da mensagem SQS e seu receipt handle para deleção.
type Message struct {
	Body          string
	ReceiptHandle string
	Attributes    map[string]string // trace context headers (traceparent, tracestate)
}

type Consumer struct {
	client   *sqs.Client
	queueURL string
}

func NewConsumer(client *sqs.Client, queueURL string) *Consumer {
	return &Consumer{client: client, queueURL: queueURL}
}

// Start faz long polling na fila e envia cada mensagem para o canal out.
// Retorna quando o ctx for cancelado.
func (c *Consumer) Start(ctx context.Context, out chan<- Message) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:              aws.String(c.queueURL),
			MaxNumberOfMessages:   10,
			WaitTimeSeconds:       20,
			MessageAttributeNames: []string{"All"},
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("failed to receive messages", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		for _, msg := range result.Messages {
			attrs := make(map[string]string, len(msg.MessageAttributes))
			for k, v := range msg.MessageAttributes {
				if v.StringValue != nil {
					attrs[k] = *v.StringValue
				}
			}
			out <- Message{
				Body:          aws.ToString(msg.Body),
				ReceiptHandle: aws.ToString(msg.ReceiptHandle),
				Attributes:    attrs,
			}
		}
	}
}

// Delete remove uma mensagem da fila após o processamento.
func (c *Consumer) Delete(ctx context.Context, receiptHandle string) error {
	_, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	return err
}
