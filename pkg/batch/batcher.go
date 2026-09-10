package batch

import (
	"context"
	"sync"
	"time"
)

type Handler[T any] func(ctx context.Context, items []T) error

type Batcher[T any] struct {
	maxSize   int
	interval  time.Duration
	handler   Handler[T]
	mu        sync.Mutex
	items     []T
	lastFlush time.Time
	stopCh    chan struct{}
	doneCh    chan struct{}
	stopped   bool
}

func NewBatcher[T any](maxSize int, interval time.Duration, handler Handler[T]) *Batcher[T] {
	return &Batcher[T]{
		maxSize:   maxSize,
		interval:  interval,
		handler:   handler,
		items:     make([]T, 0, maxSize),
		lastFlush: time.Now(),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

func (b *Batcher[T]) Start(ctx context.Context) {
	ticker := time.NewTicker(b.interval / 2)
	defer ticker.Stop()
	defer close(b.doneCh)

	for {
		select {
		case <-ctx.Done():
			_ = b.Flush(ctx)
			return
		case <-b.stopCh:
			_ = b.Flush(context.Background())
			return
		case <-ticker.C:
			b.mu.Lock()
			shouldFlush := len(b.items) > 0 && time.Since(b.lastFlush) >= b.interval
			b.mu.Unlock()
			if shouldFlush {
				_ = b.Flush(ctx)
			}
		}
	}
}

func (b *Batcher[T]) Add(ctx context.Context, item T) error {
	b.mu.Lock()
	b.items = append(b.items, item)
	shouldFlush := len(b.items) >= b.maxSize
	b.mu.Unlock()

	if shouldFlush {
		return b.Flush(ctx)
	}
	return nil
}

func (b *Batcher[T]) Flush(ctx context.Context) error {
	b.mu.Lock()
	if len(b.items) == 0 {
		b.mu.Unlock()
		return nil
	}
	items := b.items
	b.items = make([]T, 0, b.maxSize)
	b.lastFlush = time.Now()
	b.mu.Unlock()

	return b.handler(ctx, items)
}

func (b *Batcher[T]) Stop() {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}
	b.stopped = true
	b.mu.Unlock()

	close(b.stopCh)
	<-b.doneCh
}

func (b *Batcher[T]) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.items)
}
