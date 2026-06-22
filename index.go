// Package origamy provides a Go SDK for the Origamy analytics API.
// It provides an asynchronous, batched approach to sending analytics events
// with automatic retry logic, configurable flushing, and concurrent request management.
package origamy

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/QubelyLabs/origamy-go-sdk/internal/core"
	"github.com/QubelyLabs/origamy-go-sdk/internal/dispatcher"
	"github.com/QubelyLabs/origamy-go-sdk/internal/queue"
)

// Version of the client.
const Version = core.Version

// Default configuration constants.
const (
	DefaultEndpoint  = core.DefaultEndpoint
	DefaultInterval  = core.DefaultInterval
	DefaultBatchSize = core.DefaultBatchSize
)

// Error variables.
var (
	// ErrClosed is returned by methods of the Client interface when they are
	// called after the client was already closed.
	ErrClosed = core.ErrClosed

	// ErrTooManyRequests is used to notify the application that too many requests are
	// already being sent and no more messages can be accepted.
	ErrTooManyRequests = core.ErrTooManyRequests

	// ErrMessageTooBig is used to notify the client callbacks that a message send
	// failed because the JSON representation of a message exceeded the upper limit.
	ErrMessageTooBig = core.ErrMessageTooBig
)

// Client is the main API exposed by the analytics package.
// Values that satisfy this interface are returned by the client constructors
// provided by the package and provide a way to send messages via the HTTP API.
type Client = core.Client

// Message is the interface used to represent analytics objects that can be sent via a client.
type Message = core.Message

// Callback is used to notify the application when a message send succeeded or failed.
type Callback = core.Callback

// Logger is the interface for logging.
type Logger = core.Logger

// FieldGetter is used for field-based validation.
type FieldGetter = core.FieldGetter

// Dispatcher is the interface for sending analytics event batches to a backend.
// Implementations can use different transport mechanisms (HTTP, gRPC, NATS, etc.).
type Dispatcher = dispatcher.Dispatcher

// DispatcherConfig holds common configuration for dispatchers.
type DispatcherConfig = dispatcher.Config

// HTTPDispatcher implements the Dispatcher interface using HTTP transport.
type HTTPDispatcher = dispatcher.HTTPDispatcher

// HTTPOption is a function that configures an HTTPDispatcher.
type HTTPOption = dispatcher.HTTPOption

// NoopDispatcher implements the Dispatcher interface but doesn't send data anywhere.
// Instead, it logs the payload in a human-readable format to the console.
// This is useful for development, debugging, and testing.
type NoopDispatcher = dispatcher.NoopDispatcher

// NoopOption is a function that configures a NoopDispatcher.
type NoopOption = dispatcher.NoopOption

// Queue is the interface for managing message queues.
// Implementations can use different backing stores (channels, RabbitMQ, Redis, etc.).
type Queue = queue.Queue[Message]

// QueueConfig holds common configuration for queue implementations.
type QueueConfig = queue.Config

// ChannelQueue implements the Queue interface using Go channels.
type ChannelQueue = queue.ChannelQueue[Message]

// ChannelQueueOption is a function that configures a ChannelQueue.
type ChannelQueueOption = queue.ChannelOption[Message]

// ErrQueueClosed is returned when attempting to enqueue to a closed queue.
type ErrQueueClosed = queue.ErrQueueClosed

// ErrQueueFull is returned when the queue is at capacity.
type ErrQueueFull = queue.ErrQueueFull

// Config carries the different configuration options that may be set when instantiating a client.
type Config struct {
	// The endpoint to which the client connect and send their messages.
	Endpoint string

	// The flushing interval of the client.
	Interval time.Duration

	// The HTTP transport used by the client.
	// Deprecated: Use Dispatcher instead for custom transport implementations.
	Transport http.RoundTripper

	// The dispatcher used to send messages to the backend. This allows using
	// different transport mechanisms (HTTP, gRPC, NATS, etc.).
	// If none is specified, an HTTP dispatcher is created using the Transport field.
	Dispatcher Dispatcher

	// The queue used to buffer messages before sending. This allows using
	// different queue implementations (in-memory, RabbitMQ, Redis, etc.).
	// If none is specified, a channel-based in-memory queue is created.
	Queue Queue

	// The capacity of the message queue when using the default queue.
	// Defaults to 100 if not specified.
	QueueCapacity int

	// The logger used by the client.
	Logger Logger

	// The callback object for message success/failure notifications.
	Callback Callback

	// The maximum number of messages that will be sent in one API call.
	BatchSize int

	// When set to true the client will send more frequent and detailed messages.
	Verbose bool

	// The default context set on each message sent by the client.
	DefaultContext *Context

	// The retry policy used by the client.
	RetryAfter func(int) time.Duration
}

// toCoreConfig converts the public Config to internal core.Config.
func (c Config) toCoreConfig() core.Config {
	var ctx *core.Context
	if c.DefaultContext != nil {
		coreCtx := core.Context(*c.DefaultContext)
		ctx = &coreCtx
	}
	return core.Config{
		Endpoint:       c.Endpoint,
		Interval:       c.Interval,
		Transport:      c.Transport,
		Dispatcher:     c.Dispatcher,
		Queue:          c.Queue,
		QueueCapacity:  c.QueueCapacity,
		Logger:         c.Logger,
		Callback:       c.Callback,
		BatchSize:      c.BatchSize,
		Verbose:        c.Verbose,
		DefaultContext: ctx,
		RetryAfter:     c.RetryAfter,
	}
}

// ConfigError is returned when one of the configuration fields was set to an impossible value.
type ConfigError = core.ConfigError

// FieldError is returned when a field was not initialized properly.
type FieldError = core.FieldError

// Alias represents an object sent in an alias call.
type Alias = core.Alias

// Group represents an object sent in a group call.
type Group = core.Group

// Identify represents an object sent in an identify call.
type Identify = core.Identify

// Page represents an object sent in a page call.
type Page = core.Page

// Screen represents an object sent in a screen call.
type Screen = core.Screen

// Track represents an object sent in a track call.
type Track = core.Track

// Context provides the representation of the context object.
type Context = core.Context

// AppInfo provides the representation of the context.app object.
type AppInfo = core.AppInfo

// CampaignInfo provides the representation of the context.campaign object.
type CampaignInfo = core.CampaignInfo

// DeviceInfo provides the representation of the context.device object.
type DeviceInfo = core.DeviceInfo

// LibraryInfo provides the representation of the context.library object.
type LibraryInfo = core.LibraryInfo

// LocationInfo provides the representation of the context.location object.
type LocationInfo = core.LocationInfo

// NetworkInfo provides the representation of the context.network object.
type NetworkInfo = core.NetworkInfo

// OSInfo provides the representation of the context.os object.
type OSInfo = core.OSInfo

// PageInfo provides the representation of the context.page object.
type PageInfo = core.PageInfo

// ReferrerInfo provides the representation of the context.referrer object.
type ReferrerInfo = core.ReferrerInfo

// ScreenInfo provides the representation of the context.screen object.
type ScreenInfo = core.ScreenInfo

// Properties is used to represent properties in messages that support it.
type Properties = core.Properties

// Product represents products in the E-commerce API.
type Product = core.Product

// Traits is used to represent traits in messages that support it.
type Traits = core.Traits

// Integrations is used to represent integrations in messages that support it.
type Integrations = core.Integrations

// Option is a function that configures a Client.
// Options can be passed to New() to customize the client's behavior.
type Option func(*Config)

// WithDispatcher sets a custom dispatcher for the client.
// Use this to implement custom transport mechanisms (HTTP, gRPC, NATS, etc.).
func WithDispatcher(d Dispatcher) Option {
	return func(c *Config) {
		c.Dispatcher = d
	}
}

// WithQueue sets a custom queue for the client.
// Use this to implement custom message queuing (in-memory, RabbitMQ, Redis, etc.).
func WithQueue(q Queue) Option {
	return func(c *Config) {
		c.Queue = q
	}
}

// WithEndpoint sets the API endpoint for the client.
func WithEndpoint(endpoint string) Option {
	return func(c *Config) {
		c.Endpoint = endpoint
	}
}

// WithInterval sets the flush interval for the client.
func WithInterval(interval time.Duration) Option {
	return func(c *Config) {
		c.Interval = interval
	}
}

// WithBatchSize sets the maximum batch size for the client.
func WithBatchSize(size int) Option {
	return func(c *Config) {
		c.BatchSize = size
	}
}

// WithVerbose enables verbose logging for the client.
func WithVerbose(verbose bool) Option {
	return func(c *Config) {
		c.Verbose = verbose
	}
}

// WithLogger sets a custom logger for the client.
func WithLogger(logger Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

// WithCallback sets a callback for message success/failure notifications.
func WithCallback(callback Callback) Option {
	return func(c *Config) {
		c.Callback = callback
	}
}

// WithDefaultContext sets the default context for all messages.
func WithDefaultContext(ctx *Context) Option {
	return func(c *Config) {
		c.DefaultContext = ctx
	}
}

// WithRetryAfter sets the retry policy for the client.
func WithRetryAfter(fn func(int) time.Duration) Option {
	return func(c *Config) {
		c.RetryAfter = fn
	}
}

// WithQueueCapacity sets the queue capacity when using the default queue.
func WithQueueCapacity(capacity int) Option {
	return func(c *Config) {
		c.QueueCapacity = capacity
	}
}

// New instantiates a new client that uses the write key passed as first argument
// to send messages to the backend. Optional configuration can be provided via Options.
//
// Example:
//
//	// Basic usage with defaults
//	client := origamy.New("write-key")
//
//	// With custom dispatcher (e.g., noop for development)
//	client := origamy.New("write-key", origamy.WithDispatcher(origamy.NewNoopDispatcher(origamy.DispatcherConfig{})))
//
//	// With custom queue and other options
//	client := origamy.New("write-key",
//	    origamy.WithQueue(customQueue),
//	    origamy.WithVerbose(true),
//	    origamy.WithBatchSize(100),
//	)
func New(writeKey string, opts ...Option) Client {
	config := Config{}
	for _, opt := range opts {
		opt(&config)
	}
	// Ignore error since default config is always valid
	client, _ := core.NewWithConfig(writeKey, config.toCoreConfig())
	return client
}

// NewWithConfig instantiates a new client that uses the write key and configuration
// passed as arguments to send messages to the backend.
func NewWithConfig(writeKey string, config Config) (Client, error) {
	return core.NewWithConfig(writeKey, config.toCoreConfig())
}

// NewProperties creates a new Properties map.
func NewProperties() Properties {
	return core.NewProperties()
}

// NewTraits creates a new Traits map.
func NewTraits() Traits {
	return core.NewTraits()
}

// NewIntegrations creates a new Integrations map.
func NewIntegrations() Integrations {
	return core.NewIntegrations()
}

// StdLogger creates a Logger that logs to a standard log.Logger.
func StdLogger(logger *log.Logger) Logger {
	return core.StdLogger(logger)
}

// ValidateFields validates message fields using a FieldGetter.
func ValidateFields(msg FieldGetter) error {
	return core.ValidateFields(msg)
}

// NewHTTPDispatcher creates a new HTTP dispatcher with the given configuration.
func NewHTTPDispatcher(config DispatcherConfig, opts ...HTTPOption) *HTTPDispatcher {
	return dispatcher.NewHTTPDispatcher(config, opts...)
}

// WithTransport sets a custom HTTP transport for the HTTP dispatcher.
func WithTransport(transport http.RoundTripper) HTTPOption {
	return dispatcher.WithTransport(transport)
}

// WithTimeout sets the HTTP client timeout for the HTTP dispatcher.
func WithTimeout(timeout time.Duration) HTTPOption {
	return dispatcher.WithTimeout(timeout)
}

// NewNoopDispatcher creates a new noop dispatcher that logs to console.
// This is useful for development, debugging, and testing.
func NewNoopDispatcher(config DispatcherConfig, opts ...NoopOption) *NoopDispatcher {
	return dispatcher.NewNoopDispatcher(config, opts...)
}

// NoopWithOutput sets the output writer for the noop dispatcher.
func NoopWithOutput(w *os.File) NoopOption {
	return dispatcher.WithOutput(w)
}

// NoopWithColor enables/disables colored output for the noop dispatcher.
func NoopWithColor(enabled bool) NoopOption {
	return dispatcher.WithColor(enabled)
}

// NewChannelQueue creates a new channel-based in-memory queue.
func NewChannelQueue(opts ...ChannelQueueOption) *ChannelQueue {
	return queue.NewChannelQueue[Message](opts...)
}

// NewChannelQueueWithConfig creates a new channel-based queue with explicit config.
func NewChannelQueueWithConfig(config QueueConfig) *ChannelQueue {
	return queue.NewChannelQueueWithConfig[Message](config)
}

// QueueWithCapacity sets the queue capacity.
func QueueWithCapacity(capacity int) ChannelQueueOption {
	return queue.WithCapacity[Message](capacity)
}

// QueueWithLogger sets the logger for the queue.
func QueueWithLogger(logger queue.Logger) ChannelQueueOption {
	return queue.WithLogger[Message](logger)
}
