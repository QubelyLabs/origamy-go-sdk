package queue

import (
	"sync"
	"sync/atomic"
)

// ChannelQueue implements the Queue interface using Go channels.
// It provides an in-memory, goroutine-safe queue backed by a buffered channel.
type ChannelQueue[T any] struct {
	ch        chan T
	closed    atomic.Bool
	closeMu   sync.Mutex
	closeOnce sync.Once
	config    Config
}

// ChannelOption is a function that configures a ChannelQueue.
type ChannelOption[T any] func(*ChannelQueue[T])

// WithCapacity sets the queue capacity.
func WithCapacity[T any](capacity int) ChannelOption[T] {
	return func(q *ChannelQueue[T]) {
		q.config.Capacity = capacity
	}
}

// WithLogger sets the logger for the queue.
func WithLogger[T any](logger Logger) ChannelOption[T] {
	return func(q *ChannelQueue[T]) {
		q.config.Logger = logger
	}
}

// NewChannelQueue creates a new channel-based queue with the given options.
// Default capacity is 100 if not specified.
func NewChannelQueue[T any](opts ...ChannelOption[T]) *ChannelQueue[T] {
	q := &ChannelQueue[T]{
		config: Config{
			Capacity: 100, // default capacity
		},
	}

	for _, opt := range opts {
		opt(q)
	}

	if q.config.Capacity <= 0 {
		q.config.Capacity = 100
	}

	q.ch = make(chan T, q.config.Capacity)
	return q
}

// NewChannelQueueWithConfig creates a new channel-based queue with explicit config.
func NewChannelQueueWithConfig[T any](config Config) *ChannelQueue[T] {
	if config.Capacity <= 0 {
		config.Capacity = 100
	}

	return &ChannelQueue[T]{
		ch:     make(chan T, config.Capacity),
		config: config,
	}
}

// Enqueue adds a message to the queue.
// Returns ErrQueueClosed if the queue has been closed.
func (q *ChannelQueue[T]) Enqueue(msg T) (err error) {
	if q.closed.Load() {
		return ErrQueueClosed{}
	}

	// Use defer/recover to catch panic if channel is closed between check and send
	defer func() {
		if recover() != nil {
			err = ErrQueueClosed{}
		}
	}()

	q.ch <- msg
	return nil
}

// TryEnqueue attempts to enqueue without blocking.
// Returns ErrQueueFull if the queue is at capacity.
func (q *ChannelQueue[T]) TryEnqueue(msg T) error {
	if q.closed.Load() {
		return ErrQueueClosed{}
	}

	select {
	case q.ch <- msg:
		return nil
	default:
		return ErrQueueFull{}
	}
}

// Dequeue removes and returns the next message from the queue.
// Blocks until a message is available or the queue is closed.
// Returns false as the second value if the queue is closed and empty.
func (q *ChannelQueue[T]) Dequeue() (T, bool) {
	msg, ok := <-q.ch
	return msg, ok
}

// TryDequeue attempts to dequeue without blocking.
// Returns false if the queue is empty.
func (q *ChannelQueue[T]) TryDequeue() (T, bool) {
	select {
	case msg, ok := <-q.ch:
		return msg, ok
	default:
		var zero T
		return zero, false
	}
}

// Drain returns a channel that yields all remaining messages.
// The queue must be closed before draining, otherwise this will block.
// The returned channel is the underlying channel itself for efficiency.
func (q *ChannelQueue[T]) Drain() <-chan T {
	return q.ch
}

// Len returns the current number of messages in the queue.
func (q *ChannelQueue[T]) Len() int {
	return len(q.ch)
}

// Cap returns the capacity of the queue.
func (q *ChannelQueue[T]) Cap() int {
	return cap(q.ch)
}

// IsClosed returns true if the queue has been closed.
func (q *ChannelQueue[T]) IsClosed() bool {
	return q.closed.Load()
}

// Close closes the queue, preventing further enqueues.
// Messages already in the queue can still be dequeued.
// Safe to call multiple times.
func (q *ChannelQueue[T]) Close() error {
	q.closeOnce.Do(func() {
		q.closeMu.Lock()
		defer q.closeMu.Unlock()
		q.closed.Store(true)
		close(q.ch)
	})
	return nil
}

// Channel returns the underlying channel for direct access.
// This is useful for select statements in the consumer.
func (q *ChannelQueue[T]) Channel() <-chan T {
	return q.ch
}
