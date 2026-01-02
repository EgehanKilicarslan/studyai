package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Pool manages background goroutines and ensures graceful shutdown
type Pool struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger
}

// NewPool creates a new worker pool
func NewPool(logger *slog.Logger) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}
}

// Submit adds a task to the pool and tracks it
func (p *Pool) Submit(task func(ctx context.Context)) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		task(p.ctx)
	}()
}

// SubmitWithTimeout adds a task with a timeout to the pool
func (p *Pool) SubmitWithTimeout(timeout time.Duration, task func(ctx context.Context)) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ctx, cancel := context.WithTimeout(p.ctx, timeout)
		defer cancel()
		task(ctx)
	}()
}

// Context returns the pool's context
func (p *Pool) Context() context.Context {
	return p.ctx
}

// Shutdown signals all workers to stop and waits for completion
func (p *Pool) Shutdown(timeout time.Duration) {
	p.logger.Info("🛑 [Worker] Initiating graceful shutdown...")

	// Signal all workers to stop
	p.cancel()

	// Wait for all goroutines with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("✅ [Worker] All background tasks completed")
	case <-time.After(timeout):
		p.logger.Warn("⚠️ [Worker] Shutdown timeout exceeded, some tasks may not have completed",
			"timeout", timeout,
		)
	}
}

// WaitGroup returns the underlying WaitGroup (for advanced use cases)
func (p *Pool) WaitGroup() *sync.WaitGroup {
	return &p.wg
}
