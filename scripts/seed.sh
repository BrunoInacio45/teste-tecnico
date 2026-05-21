#!/bin/bash
set -e

ENDPOINT="${AWS_ENDPOINT:-http://localhost:4566}"
REGION="${AWS_REGION:-us-east-1}"

export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=$REGION

echo "==> Sending test events to raw-events..."

# Evento 1: commits
aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "a1b2c3d4-e5f6-4789-abcd-ef0123456789",
    "developer_id": "dev-42",
    "metric_type": "commits",
    "value": 7,
    "repository": "org/backend-api",
    "timestamp": "2026-05-20T09:00:00Z"
  }'
echo "   sent: commits event"

# Evento 2: pull_requests
aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "b2c3d4e5-f6a7-4890-bcde-f01234567890",
    "developer_id": "dev-42",
    "metric_type": "pull_requests",
    "value": 2,
    "repository": "org/frontend-app",
    "timestamp": "2026-05-20T10:30:00Z"
  }'
echo "   sent: pull_requests event"

# Evento 3: review_time_minutes
aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "ecefa817-c12c-491e-885e-f1d619c30816",
    "developer_id": "dev-99",
    "metric_type": "review_time_minutes",
    "value": 45,
    "repository": "org/data-pipeline",
    "timestamp": "2026-05-20T14:00:00Z"
  }'
echo "   sent: review_time_minutes event"

# Evento 4: inválido (metric_type errado) — deve ser rejeitado pelo processor
aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "138130e0-4cfd-4b06-bfa1-1ad5051658c1",
    "developer_id": "dev-42",
    "metric_type": "invalid_type",
    "value": 1,
    "repository": "org/backend-api",
    "timestamp": "2026-05-20T11:00:00Z"
  }'
echo "   sent: invalid event (should be rejected)"

echo "==> Done. Watch processor logs to see the validation in action."
