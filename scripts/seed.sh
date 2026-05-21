#!/bin/bash
set -e

ENDPOINT="${AWS_ENDPOINT:-http://localhost:4566}"
REGION="${AWS_REGION:-us-east-1}"

export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=$REGION

echo "==> Sending test events to raw-events..."

# --- dev-42: commits ---
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
echo "   sent: dev-42 commits (7)"

aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "c3d4e5f6-a7b8-4901-8def-012345678901",
    "developer_id": "dev-42",
    "metric_type": "commits",
    "value": 12,
    "repository": "org/backend-api",
    "timestamp": "2026-05-21T08:15:00Z"
  }'
echo "   sent: dev-42 commits (12)"

# --- dev-42: pull_requests ---
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
echo "   sent: dev-42 pull_requests (2)"

aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "d4e5f6a7-b8c9-4012-8efa-123456789012",
    "developer_id": "dev-42",
    "metric_type": "pull_requests",
    "value": 3,
    "repository": "org/backend-api",
    "timestamp": "2026-05-21T11:00:00Z"
  }'
echo "   sent: dev-42 pull_requests (3)"

# --- dev-42: review_time_minutes ---
aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "e5f6a7b8-c9d0-4123-8fab-234567890123",
    "developer_id": "dev-42",
    "metric_type": "review_time_minutes",
    "value": 30,
    "repository": "org/frontend-app",
    "timestamp": "2026-05-20T14:00:00Z"
  }'
echo "   sent: dev-42 review_time_minutes (30)"

aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "f6a7b8c9-d0e1-4234-8abc-345678901234",
    "developer_id": "dev-42",
    "metric_type": "review_time_minutes",
    "value": 60,
    "repository": "org/backend-api",
    "timestamp": "2026-05-21T15:30:00Z"
  }'
echo "   sent: dev-42 review_time_minutes (60)"

# --- dev-99: varios tipos ---
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
echo "   sent: dev-99 review_time_minutes (45)"

aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "a7b8c9d0-e1f2-4345-abcd-456789012345",
    "developer_id": "dev-99",
    "metric_type": "commits",
    "value": 5,
    "repository": "org/data-pipeline",
    "timestamp": "2026-05-21T09:30:00Z"
  }'
echo "   sent: dev-99 commits (5)"

aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "b8c9d0e1-f2a3-4456-bcde-567890123456",
    "developer_id": "dev-99",
    "metric_type": "pull_requests",
    "value": 1,
    "repository": "org/data-pipeline",
    "timestamp": "2026-05-21T10:00:00Z"
  }'
echo "   sent: dev-99 pull_requests (1)"

# --- dev-17: novo developer ---
aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "c9d0e1f2-a3b4-4567-8def-678901234567",
    "developer_id": "dev-17",
    "metric_type": "commits",
    "value": 20,
    "repository": "org/mobile-app",
    "timestamp": "2026-05-19T08:00:00Z"
  }'
echo "   sent: dev-17 commits (20)"

aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "d0e1f2a3-b4c5-4678-8efa-789012345678",
    "developer_id": "dev-17",
    "metric_type": "review_time_minutes",
    "value": 90,
    "repository": "org/mobile-app",
    "timestamp": "2026-05-20T16:00:00Z"
  }'
echo "   sent: dev-17 review_time_minutes (90)"

# --- Inválidos: para testar validação e DLQ ---
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
echo "   sent: INVALID metric_type (should go to DLQ)"

aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "not-a-valid-uuid",
    "developer_id": "dev-42",
    "metric_type": "commits",
    "value": 5,
    "repository": "org/backend-api",
    "timestamp": "2026-05-20T11:00:00Z"
  }'
echo "   sent: INVALID event_id (should go to DLQ)"

aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "event_id": "e1f2a3b4-c5d6-4789-8fab-890123456789",
    "developer_id": "dev-42",
    "metric_type": "review_time_minutes",
    "value": 9999,
    "repository": "org/backend-api",
    "timestamp": "2026-05-20T11:00:00Z"
  }'
echo "   sent: INVALID review_time_minutes > 1440 (should go to DLQ)"

# --- Duplicatas: para testar idempotência no aggregator ---
# Reenvia eventos já enviados acima (mesmo event_id)
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
echo "   sent: DUPLICATE of dev-42 commits (aggregator must ignore)"

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
echo "   sent: DUPLICATE of dev-99 review_time_minutes (aggregator must ignore)"

echo ""
echo "==> Done! Total: 15 events (11 valid, 3 invalid, 2 duplicates)"
echo "    Watch processor and aggregator logs to see the pipeline in action."
echo ""
echo "    Query the API:"
echo "      curl http://localhost:8080/metrics/dev-42/summary"
echo "      curl http://localhost:8080/metrics/dev-42"
echo "      curl http://localhost:8080/metrics/dev-99/summary"
echo "      curl http://localhost:8080/metrics/dev-17/summary"
