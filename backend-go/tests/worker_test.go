package tests

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/worker"
)

func TestNewPool(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	pool := worker.NewPool(logger)

	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
	if pool.Context() == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestPoolSubmit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	pool := worker.NewPool(logger)

	var counter int32

	for i := 0; i < 10; i++ {
		pool.Submit(func(ctx context.Context) {
			atomic.AddInt32(&counter, 1)
		})
	}

	pool.Shutdown(5 * time.Second)

	if atomic.LoadInt32(&counter) != 10 {
		t.Errorf("expected counter to be 10, got %d", counter)
	}
}

func TestPoolSubmitWithTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	pool := worker.NewPool(logger)

	var completed int32
	done := make(chan struct{})

	pool.SubmitWithTimeout(1*time.Second, func(ctx context.Context) {
		select {
		case <-ctx.Done():
		case <-time.After(100 * time.Millisecond):
			atomic.AddInt32(&completed, 1)
		}
		close(done)
	})

	<-done

	pool.Shutdown(5 * time.Second)

	if atomic.LoadInt32(&completed) != 1 {
		t.Errorf("expected task to complete, got %d", completed)
	}
}

func TestPoolContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	pool := worker.NewPool(logger)

	ctx := pool.Context()
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	select {
	case <-ctx.Done():
		t.Fatal("context should not be done")
	default:
	}

	pool.Shutdown(1 * time.Second)

	select {
	case <-ctx.Done():
	default:
		t.Fatal("context should be done after shutdown")
	}
}

func TestPoolWaitGroup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	pool := worker.NewPool(logger)

	wg := pool.WaitGroup()
	if wg == nil {
		t.Fatal("expected non-nil WaitGroup")
	}
}
