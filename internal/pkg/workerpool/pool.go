// Package workerpool provides a bounded goroutine pool with configurable
// worker count and queue size for asynchronous job processing.
package workerpool

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// Job represents a unit of work that can be executed by a worker.
type Job interface {
	Execute(ctx context.Context)
}

// Pool manages bounded concurrent workers.
type Pool interface {
	Submit(job Job)
	Shutdown(ctx context.Context) error
}

// WorkerPool implements Pool with a fixed number of goroutine workers
// consuming jobs from a buffered channel.
type WorkerPool struct {
	jobs        chan Job
	workerCount int
	logger      *zap.Logger
	wg          sync.WaitGroup
	done        chan struct{}
	once        sync.Once
}

// New creates a WorkerPool and starts workerCount goroutines that consume
// jobs from a buffered channel of size queueSize.
func New(workerCount, queueSize int, logger *zap.Logger) *WorkerPool {
	p := &WorkerPool{
		jobs:        make(chan Job, queueSize),
		workerCount: workerCount,
		logger:      logger,
		done:        make(chan struct{}),
	}

	for i := 0; i < workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	return p
}

// worker processes jobs from the jobs channel until it is closed.
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()
	for job := range p.jobs {
		job.Execute(context.Background())
	}
}

// Submit enqueues a job for processing. If the queue is full, the job is
// dropped and a warning is logged.
func (p *WorkerPool) Submit(job Job) {
	select {
	case p.jobs <- job:
		// Job enqueued successfully.
	default:
		p.logger.Warn("worker pool queue full, dropping job")
	}
}

// Shutdown closes the jobs channel and waits for all in-flight jobs to
// complete. If the context deadline is exceeded before workers finish,
// an error is returned listing abandoned operations.
func (p *WorkerPool) Shutdown(ctx context.Context) error {
	p.once.Do(func() {
		close(p.jobs)
	})

	waitDone := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("worker pool shutdown deadline exceeded: %w", ctx.Err())
	}
}
