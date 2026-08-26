// Package concurrent provides utilities for concurrent task execution,
// including worker pools and task queues for parallel processing.
package concurrent

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Task represents a unit of work to be executed by a worker pool.
type Task struct {
	// ID is the unique identifier for the task.
	ID string
	// Execute is the function that performs the task work.
	Execute func(ctx context.Context) (interface{}, error)
	// Timeout is the maximum duration for this task (0 means no timeout).
	Timeout time.Duration
}

// Result represents the outcome of a task execution.
type Result struct {
	// ID is the task ID.
	ID string
	// Value is the task result value (nil if error).
	Value interface{}
	// Error is the error returned by the task (nil if success).
	Error error
	// Duration is how long the task took to execute.
	Duration time.Duration
}

// WorkerPool manages a pool of goroutines that execute tasks concurrently.
type WorkerPool struct {
	workers    int
	taskCh     chan Task
	resultCh   chan Result
	wg         sync.WaitGroup
	shutdownCh chan struct{}
	shutdown   int32
	active     int32
}

// NewWorkerPool creates a new WorkerPool with the specified number of workers.
func NewWorkerPool(workers int) *WorkerPool {
	if workers <= 0 {
		workers = 1
	}
	return &WorkerPool{
		workers:    workers,
		taskCh:     make(chan Task, workers*2),
		resultCh:   make(chan Result, workers*2),
		shutdownCh: make(chan struct{}),
	}
}

// Start begins the worker pool. Workers will process tasks submitted via Submit.
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

// Submit adds a task to the worker pool. Returns an error if the pool is shut down.
func (wp *WorkerPool) Submit(task Task) error {
	if atomic.LoadInt32(&wp.shutdown) == 1 {
		return fmt.Errorf("worker pool is shut down")
	}
	atomic.AddInt32(&wp.active, 1)
	wp.taskCh <- task
	return nil
}

// ResultChan returns the channel where task results are sent.
func (wp *WorkerPool) ResultChan() <-chan Result {
	return wp.resultCh
}

// Shutdown gracefully shuts down the worker pool, waiting for active tasks to complete.
func (wp *WorkerPool) Shutdown() {
	if !atomic.CompareAndSwapInt32(&wp.shutdown, 0, 1) {
		return
	}
	close(wp.taskCh)
	wp.wg.Wait()
	close(wp.resultCh)
	close(wp.shutdownCh)
}

// ActiveCount returns the number of tasks currently being processed or waiting.
func (wp *WorkerPool) ActiveCount() int32 {
	return atomic.LoadInt32(&wp.active)
}

// worker is the main worker goroutine loop.
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for task := range wp.taskCh {
		result := wp.executeTask(task)
		atomic.AddInt32(&wp.active, -1)
		wp.resultCh <- result
	}
}

// executeTask runs a single task with optional timeout.
func (wp *WorkerPool) executeTask(task Task) Result {
	start := time.Now()
	result := Result{
		ID: task.ID,
	}

	if task.Execute == nil {
		result.Error = fmt.Errorf("task has nil Execute function")
		result.Duration = time.Since(start)
		return result
	}

	ctx := context.Background()
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		result.Value, result.Error = task.Execute(ctx)
	}()

	select {
	case <-done:
		// Task completed normally
	case <-ctx.Done():
		result.Error = fmt.Errorf("task %s: %w", task.ID, ctx.Err())
	}

	result.Duration = time.Since(start)
	return result
}

// ExecuteAll submits multiple tasks and returns all results.
// It blocks until all tasks complete or the context is cancelled.
func (wp *WorkerPool) ExecuteAll(ctx context.Context, tasks []Task) []Result {
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]Result, 0, len(tasks))

	resultCh := make(chan Result, len(tasks))

	for _, task := range tasks {
		wg.Add(1)
		t := task // capture
		go func() {
			defer wg.Done()
			// Create a derived context for each task
			taskCtx := ctx
			if t.Timeout > 0 {
				var cancel context.CancelFunc
				taskCtx, cancel = context.WithTimeout(ctx, t.Timeout)
				defer cancel()
			}
			result := wp.executeTaskDirect(t, taskCtx)
			resultCh <- result
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for result := range resultCh {
		mu.Lock()
		results = append(results, result)
		mu.Unlock()
	}

	return results
}

// executeTaskDirect executes a task directly (without going through the pool channel).
func (wp *WorkerPool) executeTaskDirect(task Task, ctx context.Context) Result {
	start := time.Now()
	result := Result{
		ID: task.ID,
	}

	if task.Execute == nil {
		result.Error = fmt.Errorf("task has nil Execute function")
		result.Duration = time.Since(start)
		return result
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		result.Value, result.Error = task.Execute(ctx)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		result.Error = fmt.Errorf("task %s: %w", task.ID, ctx.Err())
	}

	result.Duration = time.Since(start)
	return result
}
