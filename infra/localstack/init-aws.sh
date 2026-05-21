#!/bin/bash
set -e

export AWS_DEFAULT_REGION=us-east-1
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test

ENDPOINT=http://localhost:4566

echo "==> Creating SQS queues..."

aws --endpoint-url=$ENDPOINT sqs create-queue --queue-name raw-events-dlq
aws --endpoint-url=$ENDPOINT sqs create-queue --queue-name raw-events \
  --attributes '{
    "RedrivePolicy": "{\"deadLetterTargetArn\":\"arn:aws:sqs:us-east-1:000000000000:raw-events-dlq\",\"maxReceiveCount\":\"3\"}"
  }'

aws --endpoint-url=$ENDPOINT sqs create-queue --queue-name processed-events-dlq
aws --endpoint-url=$ENDPOINT sqs create-queue --queue-name processed-events \
  --attributes '{
    "RedrivePolicy": "{\"deadLetterTargetArn\":\"arn:aws:sqs:us-east-1:000000000000:processed-events-dlq\",\"maxReceiveCount\":\"3\"}"
  }'

echo "==> Creating DynamoDB tables..."

aws --endpoint-url=$ENDPOINT dynamodb create-table \
  --table-name events \
  --attribute-definitions AttributeName=id,AttributeType=S \
  --key-schema AttributeName=id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

aws --endpoint-url=$ENDPOINT dynamodb create-table \
  --table-name developer_summary \
  --attribute-definitions AttributeName=developer_id,AttributeType=S \
  --key-schema AttributeName=developer_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

echo "==> AWS resources created successfully!"
