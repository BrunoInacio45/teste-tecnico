// package usecase (not usecase_test) gives access to unexported retryWithBackoff.
package usecase

import (
	"context"
	"errors"
	"testing"
)

// --- mock implementations ---

type mockPublisher struct {
	calls   int
	failFor int // fail the first N calls, then succeed
	err     error
}

func (m *mockPublisher) Publish(_ context.Context, _ string) error {
	m.calls++
	if m.failFor > 0 && m.calls <= m.failFor {
		return m.err
	}
	return nil
}

type mockAcker struct {
	calls          int
	receiptHandles []string
}

func (m *mockAcker) Delete(_ context.Context, receiptHandle string) error {
	m.calls++
	m.receiptHandles = append(m.receiptHandles, receiptHandle)
	return nil
}

// --- retryWithBackoff tests ---

func TestRetryWithBackoff_SucceedsFirstAttempt(t *testing.T) {
	calls := 0
	err := retryWithBackoff(3, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

// Uses maxAttempts=1 to avoid sleep delays while still testing the failure path.
func TestRetryWithBackoff_AllAttemptsFail(t *testing.T) {
	sentinel := errors.New("fail")
	calls := 0
	err := retryWithBackoff(1, func() error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

// Uses maxAttempts=2 (1 sleep of 1 s) to verify retry count and eventual success.
func TestRetryWithBackoff_SucceedsOnRetry(t *testing.T) {
	calls := 0
	err := retryWithBackoff(2, func() error {
		calls++
		if calls == 1 {
			return errors.New("first attempt fails")
		}
		return nil
	})
	if err != nil {
		t.Errorf("expected nil after retry, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

// --- Orchestrator.Execute tests ---

const validBody = `{
	"event_id":     "550e8400-e29b-41d4-a716-446655440000",
	"developer_id": "dev-123",
	"metric_type":  "commits",
	"value":        5,
	"repository":   "org/repo",
	"timestamp":    "2025-01-01T10:00:00Z"
}`

func TestOrchestrator_Execute_InvalidJSON_DoesNotDelete(t *testing.T) {
	pub := &mockPublisher{}
	ack := &mockAcker{}
	o := NewOrchestrator(pub, ack, "proc-1")

	o.Execute(context.Background(), `{not valid json`, "receipt-1")

	if pub.calls != 0 {
		t.Errorf("publish should not be called, got %d calls", pub.calls)
	}
	if ack.calls != 0 {
		t.Errorf("delete should not be called, got %d calls", ack.calls)
	}
}

func TestOrchestrator_Execute_InvalidEvent_DoesNotDelete(t *testing.T) {
	pub := &mockPublisher{}
	ack := &mockAcker{}
	o := NewOrchestrator(pub, ack, "proc-1")

	// future timestamp → validation failure
	invalidBody := `{
		"event_id":     "550e8400-e29b-41d4-a716-446655440000",
		"developer_id": "dev-123",
		"metric_type":  "commits",
		"value":        5,
		"repository":   "org/repo",
		"timestamp":    "2099-01-01T00:00:00Z"
	}`
	o.Execute(context.Background(), invalidBody, "receipt-2")

	if pub.calls != 0 {
		t.Errorf("publish should not be called, got %d calls", pub.calls)
	}
	if ack.calls != 0 {
		t.Errorf("delete should not be called, got %d calls", ack.calls)
	}
}

func TestOrchestrator_Execute_Success_DeletesMessage(t *testing.T) {
	pub := &mockPublisher{}
	ack := &mockAcker{}
	o := NewOrchestrator(pub, ack, "proc-1")

	o.Execute(context.Background(), validBody, "receipt-ok")

	if pub.calls != 1 {
		t.Errorf("expected 1 publish call, got %d", pub.calls)
	}
	if ack.calls != 1 {
		t.Errorf("expected 1 delete call, got %d", ack.calls)
	}
	if len(ack.receiptHandles) == 0 || ack.receiptHandles[0] != "receipt-ok" {
		t.Errorf("expected receipt handle %q, got %v", "receipt-ok", ack.receiptHandles)
	}
}

// This test takes ~3 s due to exponential backoff (1 s + 2 s sleeps across 3 attempts).
func TestOrchestrator_Execute_PublishFails_DoesNotDelete(t *testing.T) {
	sentinel := errors.New("queue unavailable")
	pub := &mockPublisher{failFor: 99, err: sentinel} // always fails
	ack := &mockAcker{}
	o := NewOrchestrator(pub, ack, "proc-1")

	o.Execute(context.Background(), validBody, "receipt-fail")

	if pub.calls != 3 {
		t.Errorf("expected 3 publish attempts, got %d", pub.calls)
	}
	if ack.calls != 0 {
		t.Errorf("delete must not be called when publish fails, got %d calls", ack.calls)
	}
}
