package concurrent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// QueuePriority represents the priority of a task in the queue.
type QueuePriority int

const (
	// PriorityLow is the lowest priority.
	PriorityLow QueuePriority = 0
	// PriorityNormal is the default priority.
	PriorityNormal QueuePriority = 10
	// PriorityHigh is high priority.
	PriorityHigh QueuePriority = 20
	// PriorityCritical is the highest priority.
	PriorityCritical QueuePriority = 30
)

// QueueTask represents a task in the priority queue.
type QueueTask struct {
	// ID is the unique task identifier.
	ID string
	// Priority is the task priority.
	Priority QueuePriority
	// Execute is the function to execute.
	Execute func(ctx context.Context) (interface{}, error)
	// Timeout is the maximum execution time.
	Timeout time.Duration
	// SubmittedAt records when the task was submitted.
	SubmittedAt time.Time
}

// PriorityQueue is a thread-safe priority queue for tasks.
type PriorityQueue struct {
	mu     sync.Mutex
	tasks  []*QueueTask
	signal chan struct{}
	closed bool
}

// NewPriorityQueue creates a new PriorityQueue.
func NewPriorityQueue() *PriorityQueue {
	return &PriorityQueue{
		tasks:  make([]*QueueTask, 0),
		signal: make(chan struct{}, 1),
	}
}

// Enqueue adds a task to the priority queue.
func (q *PriorityQueue) Enqueue(task *QueueTask) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return fmt.Errorf("queue is closed")
	}

	task.SubmittedAt = time.Now()
	q.tasks = append(q.tasks, task)

	// Sort by priority (descending), then by submission time (ascending)
	q.sortLocked()

	// Signal that a new task is available
	select {
	case q.signal <- struct{}{}:
	default:
	}

	return nil
}

// Dequeue removes and returns the highest priority task.
// Blocks until a task is available or the queue is closed.
func (q *PriorityQueue) Dequeue() (*QueueTask, error) {
	for {
		q.mu.Lock()
		if len(q.tasks) > 0 {
			task := q.tasks[0]
			q.tasks = q.tasks[1:]
			q.mu.Unlock()
			return task, nil
		}
		if q.closed {
			q.mu.Unlock()
			return nil, fmt.Errorf("queue is closed")
		}
		q.mu.Unlock()

		// Wait for a signal or timeout
		select {
		case <-q.signal:
			continue
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}

// TryDequeue attempts to dequeue without blocking. Returns nil if queue is empty.
func (q *PriorityQueue) TryDequeue() (*QueueTask, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil, fmt.Errorf("queue is closed")
	}

	if len(q.tasks) == 0 {
		return nil, nil
	}

	task := q.tasks[0]
	q.tasks = q.tasks[1:]
	return task, nil
}

// Length returns the number of tasks in the queue.
func (q *PriorityQueue) Length() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.tasks)
}

// Close closes the queue. Waiting goroutines will receive an error.
func (q *PriorityQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.signal)
	}
}

// sortLocked sorts tasks by priority (high to low), then by submission time (oldest first).
// Must be called with the mutex held.
func (q *PriorityQueue) sortLocked() {
	// Simple insertion sort for small queues; for large queues this could be optimized
	for i := 1; i < len(q.tasks); i++ {
		j := i
		for j > 0 {
			prev := q.tasks[j-1]
			curr := q.tasks[j]
			if curr.Priority > prev.Priority {
				q.tasks[j-1], q.tasks[j] = q.tasks[j], q.tasks[j-1]
				j--
			} else if curr.Priority == prev.Priority && curr.SubmittedAt.Before(prev.SubmittedAt) {
				q.tasks[j-1], q.tasks[j] = q.tasks[j], q.tasks[j-1]
				j--
			} else {
				break
			}
		}
	}
}

// ProcessQueue starts processing tasks from the queue using a worker pool.
// It returns when the context is cancelled or the queue is closed.
func (q *PriorityQueue) ProcessQueue(ctx context.Context, workers int) {
	if workers <= 0 {
		workers = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				task, err := q.Dequeue()
				if err != nil {
					return
				}

				// Execute the task
				taskCtx := ctx
				if task.Timeout > 0 {
					var cancel context.CancelFunc
					taskCtx, cancel = context.WithTimeout(ctx, task.Timeout)
					func() {
						defer cancel()
						if task.Execute != nil {
							_, _ = task.Execute(taskCtx)
						}
					}()
				} else {
					if task.Execute != nil {
						_, _ = task.Execute(taskCtx)
					}
				}
			}
		}()
	}
	wg.Wait()
}
