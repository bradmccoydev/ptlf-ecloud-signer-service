package workerpool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// testJob is a simple job implementation for testing.
type testJob struct {
	fn func(ctx context.Context)
}

func (j *testJob) Execute(ctx context.Context) {
	j.fn(ctx)
}

func TestJobsAreExecuted(t *testing.T) {
	logger := zap.NewNop()
	pool := New(3, 10, logger)

	var executed atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		pool.Submit(&testJob{fn: func(ctx context.Context) {
			executed.Add(1)
			wg.Done()
		}})
	}

	wg.Wait()

	if got := executed.Load(); got != 5 {
		t.Errorf("expected 5 jobs executed, got %d", got)
	}

	err := pool.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}

func TestPoolRespectsWorkerCountBounds(t *testing.T) {
	logger := zap.NewNop()
	workerCount := 2
	pool := New(workerCount, 100, logger)

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		pool.Submit(&testJob{fn: func(ctx context.Context) {
			defer wg.Done()
			cur := concurrent.Add(1)
			// Track maximum concurrency observed.
			for {
				max := maxConcurrent.Load()
				if cur <= max || maxConcurrent.CompareAndSwap(max, cur) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			concurrent.Add(-1)
		}})
	}

	wg.Wait()

	if got := maxConcurrent.Load(); got > int32(workerCount) {
		t.Errorf("expected max concurrency <= %d, got %d", workerCount, got)
	}

	err := pool.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}

func TestSubmitOnFullPoolDropsJob(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)

	// Create pool with queue size 1 and 1 slow worker to fill the queue.
	pool := New(1, 1, logger)

	// Block the single worker with a long-running job.
	blocker := make(chan struct{})
	pool.Submit(&testJob{fn: func(ctx context.Context) {
		<-blocker
	}})

	// Give worker time to pick up the blocking job.
	time.Sleep(20 * time.Millisecond)

	// Fill the queue (size 1).
	pool.Submit(&testJob{fn: func(ctx context.Context) {}})

	// This job should be dropped since the queue is full and worker is busy.
	pool.Submit(&testJob{fn: func(ctx context.Context) {}})

	// Check that a warning was logged.
	if logs.Len() == 0 {
		t.Error("expected warning log for dropped job, got none")
	} else {
		entry := logs.All()[0]
		if entry.Message != "worker pool queue full, dropping job" {
			t.Errorf("unexpected log message: %s", entry.Message)
		}
	}

	// Unblock and shut down.
	close(blocker)
	err := pool.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}

func TestGracefulShutdownDrainsInflightJobs(t *testing.T) {
	logger := zap.NewNop()
	pool := New(2, 10, logger)

	var executed atomic.Int32

	for i := 0; i < 5; i++ {
		pool.Submit(&testJob{fn: func(ctx context.Context) {
			time.Sleep(20 * time.Millisecond)
			executed.Add(1)
		}})
	}

	// Give workers a moment to start processing.
	time.Sleep(10 * time.Millisecond)

	err := pool.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}

	if got := executed.Load(); got != 5 {
		t.Errorf("expected all 5 jobs drained, got %d", got)
	}
}

func TestShutdownWithTimeoutReturnsError(t *testing.T) {
	logger := zap.NewNop()
	pool := New(1, 10, logger)

	// Submit a job that takes a long time.
	pool.Submit(&testJob{fn: func(ctx context.Context) {
		time.Sleep(5 * time.Second)
	}})

	// Give worker time to start the long job.
	time.Sleep(20 * time.Millisecond)

	// Shutdown with a very short deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := pool.Shutdown(ctx)
	if err == nil {
		t.Fatal("expected error from shutdown with exceeded deadline, got nil")
	}
	if !contains(err.Error(), "deadline exceeded") {
		t.Errorf("expected deadline exceeded error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
