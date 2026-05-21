#!/bin/bash
set -e

ENDPOINT="${AWS_ENDPOINT:-http://localhost:4566}"
REGION="${AWS_REGION:-us-east-1}"

export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=$REGION

echo "==> Sending test message to raw-events..."

aws --endpoint-url=$ENDPOINT sqs send-message \
  --queue-url "$ENDPOINT/000000000000/raw-events" \
  --message-body '{
    "id": "test-event-1",
    "developer_id": "dev-42",
    "repo": "my-repo",
    "event_type": "push",
    "timestamp": "2026-05-21T10:00:00Z"
  }'

echo "==> Done."
