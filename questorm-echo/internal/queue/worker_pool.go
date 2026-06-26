package queue

import (
	"context"
	"sust-preli/internal/engine"
)

// Job represents a ticket analysis task pushed into the worker pool.
type Job struct {
	Ctx        context.Context
	Request    *engine.TicketRequest
	ResultChan chan Result
}

// Result holds the outcome of a job.
type Result struct {
	Response *engine.TicketResponse
	Err      error
}

// WorkerPool manages a pool of concurrent worker goroutines to process jobs.
type WorkerPool struct {
	jobQueue   chan Job
	maxWorkers int
}

// NewWorkerPool initializes a new worker pool.
func NewWorkerPool(maxWorkers int, queueSize int) *WorkerPool {
	return &WorkerPool{
		jobQueue:   make(chan Job, queueSize),
		maxWorkers: maxWorkers,
	}
}

// Start spawns the worker goroutines.
// The processFunc parameter is the core analysis pipeline function.
func (wp *WorkerPool) Start(processFunc func(context.Context, *engine.TicketRequest) (*engine.TicketResponse, error)) {
	for i := 0; i < wp.maxWorkers; i++ {
		go func() {
			for job := range wp.jobQueue {
				// If the context is already canceled (e.g., client timeout), skip processing
				if job.Ctx.Err() != nil {
					job.ResultChan <- Result{Err: job.Ctx.Err()}
					continue
				}

				// Process the job
				resp, err := processFunc(job.Ctx, job.Request)
				job.ResultChan <- Result{Response: resp, Err: err}
			}
		}()
	}
}

// Submit enqueues a request and blocks until a result is ready or the context times out.
func (wp *WorkerPool) Submit(ctx context.Context, req *engine.TicketRequest) (*engine.TicketResponse, error) {
	resultChan := make(chan Result, 1)
	job := Job{
		Ctx:        ctx,
		Request:    req,
		ResultChan: resultChan,
	}

	// Try to queue the job. If the queue is full, we block until space is available or context times out.
	select {
	case wp.jobQueue <- job:
		// Job queued successfully. Now wait for the worker to finish or for context timeout.
		select {
		case res := <-resultChan:
			return res.Response, res.Err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
