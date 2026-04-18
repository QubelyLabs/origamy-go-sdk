// Package queue provides message queue abstractions for the analytics client.
// It defines a Queue interface that can be implemented by different queue providers
// such as in-memory channels, RabbitMQ, Redis, or other message brokers.
package queue

import "io"

// Queue is the interface for managing message queues.
// Implementations can use different backing stores (channels, RabbitMQ, Redis, etc.).
type Queue[T any] interface {
	io.Closer

	// Enqueue adds a message to the queue.
	// Returns an error if the queue is closed or full (depending on implementation).
	Enqueue(msg T) error

	// Dequeue removes and returns the next message from the queue.
	// This is a blocking operation that waits until a message is available
	// or the queue is closed.
	Dequeue() (T, bool)

	// TryDequeue attempts to dequeue a message without blocking.
	// Returns the message and true if successful, or zero value and false if
	// the queue is empty.
	TryDequeue() (T, bool)

	// Drain returns a channel that yields all remaining messages.
	// This is typically used during shutdown to process pending messages.
	// The returned channel is closed when all messages have been drained.
	Drain() <-chan T

	// Channel returns a receive-only channel for use in select statements.
	// This enables non-blocking reads in event loops.
	// For non-channel implementations, this should return a channel that
	// proxies messages from the underlying queue.
	Channel() <-chan T

	// Len returns the current number of messages in the queue.
	// Note: This may not be accurate for distributed queue implementations.
	Len() int

	// IsClosed returns true if the queue has been closed.
	IsClosed() bool
}

// Config holds common configuration for queue implementations.
type Config struct {
	// Capacity is the maximum number of messages the queue can hold.
	// For unbounded queues, set to 0 or negative.
	Capacity int

	// Logger is used for logging queue operations.
	Logger Logger
}

// Logger interface for queue logging.
type Logger interface {
	Logf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// ErrQueueClosed is returned when attempting to enqueue to a closed queue.
type ErrQueueClosed struct{}

func (e ErrQueueClosed) Error() string {
	return "queue is closed"
}

// ErrQueueFull is returned when the queue is at capacity (for bounded queues).
type ErrQueueFull struct{}

func (e ErrQueueFull) Error() string {
	return "queue is full"
}
