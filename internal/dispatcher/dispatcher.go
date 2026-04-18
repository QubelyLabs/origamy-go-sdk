// Package dispatcher provides transport abstractions for sending analytics events.
// It defines a Dispatcher interface that can be implemented by different transport
// providers such as HTTP, gRPC, or message queues like NATS.
package dispatcher

import "io"

// Dispatcher is the interface for sending analytics event batches to a backend.
// Implementations can use different transport mechanisms (HTTP, gRPC, NATS, etc.).
type Dispatcher interface {
	io.Closer

	// Send dispatches a serialized batch payload to the analytics backend.
	// It returns an error if the send operation fails or if the backend
	// returns an error response.
	Send(payload []byte) error
}

// Config holds common configuration for dispatchers.
type Config struct {
	// Endpoint is the base URL/address of the analytics backend.
	Endpoint string

	// WriteKey is used for authentication with the backend.
	WriteKey string

	// Version is the SDK version for the User-Agent header.
	Version string

	// Verbose enables detailed logging when true.
	Verbose bool

	// Logger is used for logging dispatcher operations.
	Logger Logger
}

// Logger interface for dispatcher logging.
type Logger interface {
	Logf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}
