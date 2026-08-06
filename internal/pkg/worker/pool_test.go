package worker

import (
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestPoolExecutesEnqueuedTasks(t *testing.T) {
	p := NewPool(2, 10, silentLogger())
	defer p.Shutdown()

	var count int32
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		p.Enqueue(func() {
			atomic.AddInt32(&count, 1)
			wg.Done()
		})
	}
	wg.Wait()

	if got := atomic.LoadInt32(&count); got != 5 {
		t.Errorf("executed count = %d, want 5", got)
	}
}

// TestPoolDropsWhenBufferFull pins the required drop-not-block behaviour:
// once the buffered channel is saturated, Enqueue must return immediately
// rather than wait for a worker to free a slot.
func TestPoolDropsWhenBufferFull(t *testing.T) {
	block := make(chan struct{})
	p := NewPool(1, 1, silentLogger())
	defer func() {
		close(block)
		p.Shutdown()
	}()

	// Occupy the single worker so it can't drain the buffer.
	p.Enqueue(func() { <-block })
	// Fill the one-slot buffer.
	p.Enqueue(func() {})

	done := make(chan struct{})
	go func() {
		// Buffer is full and the worker is busy — this must be dropped, not block.
		p.Enqueue(func() {})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Enqueue blocked instead of dropping the task when the buffer was full")
	}
}

func TestPoolRecoversPanickingTask(t *testing.T) {
	p := NewPool(1, 2, silentLogger())
	defer p.Shutdown()

	var wg sync.WaitGroup
	wg.Add(1)
	var ran int32

	p.Enqueue(func() { panic("boom") })
	p.Enqueue(func() {
		atomic.AddInt32(&ran, 1)
		wg.Done()
	})

	wg.Wait()
	if got := atomic.LoadInt32(&ran); got != 1 {
		t.Errorf("task after a panicking task ran = %d times, want 1 — a panic must not kill the worker", got)
	}
}

func TestPoolShutdownDrainsBuffer(t *testing.T) {
	p := NewPool(2, 10, silentLogger())

	var count int32
	for i := 0; i < 10; i++ {
		p.Enqueue(func() { atomic.AddInt32(&count, 1) })
	}

	p.Shutdown()

	if got := atomic.LoadInt32(&count); got != 10 {
		t.Errorf("executed count after Shutdown = %d, want 10 — Shutdown must drain the buffer", got)
	}
}
