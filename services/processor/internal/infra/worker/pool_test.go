package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStart_ProcessesAllJobs(t *testing.T) {
	jobs := make(chan Job, 5)
	for i := range 5 {
		jobs <- Job{Body: "body", ReceiptHandle: string(rune('A' + i))}
	}
	close(jobs)

	var processed atomic.Int32
	Start(context.Background(), 2, jobs, func(_ context.Context, _ Job) {
		processed.Add(1)
	})

	if got := processed.Load(); got != 5 {
		t.Errorf("expected 5 jobs processed, got %d", got)
	}
}

func TestStart_ChannelClose_StopsWorkers(t *testing.T) {
	jobs := make(chan Job)
	close(jobs) // immediately closed, no jobs

	var processed atomic.Int32
	Start(context.Background(), 3, jobs, func(_ context.Context, _ Job) {
		processed.Add(1)
	})

	if got := processed.Load(); got != 0 {
		t.Errorf("expected 0 jobs processed, got %d", got)
	}
}

func TestStart_ContextCancel_StopsWorkers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	jobs := make(chan Job) // never closes, never sends
	var started sync.WaitGroup
	started.Add(1)

	done := make(chan struct{})
	go func() {
		started.Done()
		Start(ctx, 2, jobs, func(_ context.Context, _ Job) {})
		close(done)
	}()

	started.Wait()
	cancel()

	select {
	case <-done:
		// workers stopped cleanly
	case <-time.After(2 * time.Second):
		t.Error("Start did not return after context cancellation")
	}
}

func TestStart_RunsFnWithCorrectJob(t *testing.T) {
	jobs := make(chan Job, 1)
	want := Job{Body: "hello", ReceiptHandle: "rh-123"}
	jobs <- want
	close(jobs)

	var got Job
	Start(context.Background(), 1, jobs, func(_ context.Context, j Job) {
		got = j
	})

	if got != want {
		t.Errorf("fn received %+v, want %+v", got, want)
	}
}

func TestStart_ConcurrentWorkers(t *testing.T) {
	const numWorkers = 4
	const numJobs = 8

	jobs := make(chan Job, numJobs)
	for range numJobs {
		jobs <- Job{Body: "work"}
	}
	close(jobs)

	var maxConcurrent atomic.Int32
	var current atomic.Int32
	var mu sync.Mutex
	_ = mu

	Start(context.Background(), numWorkers, jobs, func(_ context.Context, _ Job) {
		n := current.Add(1)
		if n > maxConcurrent.Load() {
			maxConcurrent.Store(n)
		}
		time.Sleep(10 * time.Millisecond)
		current.Add(-1)
	})

	if maxConcurrent.Load() < 2 {
		t.Errorf("expected concurrent execution with %d workers, max observed = %d",
			numWorkers, maxConcurrent.Load())
	}
}

func TestStart_BlocksUntilAllWorkersFinish(t *testing.T) {
	jobs := make(chan Job, 1)
	jobs <- Job{Body: "slow"}
	close(jobs)

	var finished atomic.Bool
	Start(context.Background(), 1, jobs, func(_ context.Context, _ Job) {
		time.Sleep(50 * time.Millisecond)
		finished.Store(true)
	})

	if !finished.Load() {
		t.Error("Start returned before the worker finished processing")
	}
}
