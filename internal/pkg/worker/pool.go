package worker

import (
	"log/slog"
	"sync"
)

// Task is a unit of work processed by the pool. The caller must capture its
// own detached context (context.WithoutCancel) in the closure — the pool
// itself carries no context, since a request's cancellation must never
// propagate into background work.
type Task func()

// Pool is a bounded worker pool for fire-and-forget background work (e.g.
// audit inserts) that must never block the caller and must never leak an
// unbounded number of goroutines.
type Pool struct {
	tasks  chan Task
	wg     sync.WaitGroup
	logger *slog.Logger
}

// NewPool starts n workers reading from a buffered channel of size bufferSize.
func NewPool(n, bufferSize int, logger *slog.Logger) *Pool {
	p := &Pool{
		tasks:  make(chan Task, bufferSize),
		logger: logger,
	}

	p.wg.Add(n)
	for i := 0; i < n; i++ {
		go p.run()
	}

	return p
}

func (p *Pool) run() {
	defer p.wg.Done()
	for task := range p.tasks {
		p.execute(task)
	}
}

// execute recovers a panicking task so one bad task can't kill a worker goroutine.
func (p *Pool) execute(task Task) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("worker task panicked", slog.Any("panic", r))
		}
	}()
	task()
}

// Enqueue submits a task without blocking. If the buffer is full the task is
// dropped and logged — callers must never be blocked by audit/background work.
func (p *Pool) Enqueue(task Task) {
	select {
	case p.tasks <- task:
	default:
		p.logger.Warn("worker pool buffer full, dropping task")
	}
}

// Shutdown closes the task channel and blocks until every queued task drains.
func (p *Pool) Shutdown() {
	close(p.tasks)
	p.wg.Wait()
}
