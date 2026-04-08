# Architecture

## Overview

Origamy SDK is a Go client library for the Origamy analytics API. It provides an asynchronous, batched approach to sending analytics events with automatic retry logic, configurable flushing, and concurrent request management.

The SDK is designed with pluggable interfaces for both message queuing and dispatching, allowing you to swap implementations for different use cases (development, testing, production) or integrate with external systems (RabbitMQ, Redis, gRPC, etc.).

## High-Level Architecture

```mermaid
graph TB
    subgraph "Application Layer"
        App[Application Code]
    end

    subgraph "Public API"
        Client[Client Interface]
        New[New / NewWithConfig]
        Options[WithDispatcher / WithQueue / ...]
    end

    subgraph "Message Types"
        Track[Track]
        Identify[Identify]
        Page[Page]
        Screen[Screen]
        Group[Group]
        Alias[Alias]
    end

    subgraph "Queue Layer"
        QueueInterface[Queue Interface]
        ChannelQueue[ChannelQueue<br>In-memory]
        CustomQueue[Custom Queue<br>RabbitMQ/Redis/etc]
    end

    subgraph "Internal Processing"
        Loop[Event Loop]
        Batch[Message Batching]
        Executor[Executor]
    end

    subgraph "Dispatcher Layer"
        DispatcherInterface[Dispatcher Interface]
        HTTPDispatcher[HTTPDispatcher<br>Production]
        NoopDispatcher[NoopDispatcher<br>Development/Debug]
        CustomDispatcher[Custom Dispatcher<br>gRPC/NATS/etc]
    end

    subgraph "External"
        OrigamyAPI[Origamy API<br>/v1/batch]
        Console[Console Output]
    end

    App --> New
    App --> Options
    Options --> New
    New --> Client
    App --> Track & Identify & Page & Screen & Group & Alias
    Track & Identify & Page & Screen & Group & Alias --> Client

    Client -->|Enqueue| QueueInterface
    QueueInterface --> ChannelQueue
    QueueInterface -.-> CustomQueue

    ChannelQueue --> Loop
    Loop --> Batch
    Batch -->|Batch Ready| Executor

    Executor --> DispatcherInterface
    DispatcherInterface --> HTTPDispatcher
    DispatcherInterface --> NoopDispatcher
    DispatcherInterface -.-> CustomDispatcher

    HTTPDispatcher --> OrigamyAPI
    NoopDispatcher --> Console
```

## Component Architecture

```mermaid
graph LR
    subgraph "Core Components"
        direction TB
        A[analytics.go<br>Client implementation]
        B[config.go<br>Configuration]
        C[message.go<br>Message handling]
        D[executor.go<br>Concurrent execution]
    end

    subgraph "Dispatcher Module"
        direction TB
        DA[dispatcher.go<br>Interface definition]
        DB[http.go<br>HTTP implementation]
        DC[noop.go<br>Console logging]
    end

    subgraph "Queue Module"
        direction TB
        QA[queue.go<br>Interface definition]
        QB[channel.go<br>Channel-based impl]
    end

    subgraph "Message Types"
        direction TB
        E[track.go]
        F[identify.go]
        G[page.go]
        H[screen.go]
        I[group.go]
        J[alias.go]
    end

    subgraph "Data Types"
        direction TB
        K[context.go<br>Context metadata]
        L[properties.go<br>Event properties]
        M[traits.go<br>User traits]
        N[integrations.go<br>Integration controls]
    end

    subgraph "Support"
        direction TB
        O[logger.go<br>Logging interface]
        P[error.go<br>Error types]
        Q[validate.go<br>Validation helpers]
    end

    A --> B & C & D
    A --> QA & DA
    QA --> QB
    DA --> DB & DC
    A --> E & F & G & H & I & J
    E & F & G & H & I & J --> K & L & M & N
    A --> O & P
```

## Key Interfaces

### Dispatcher Interface

The `Dispatcher` interface allows you to customize how events are sent to the backend:

```go
type Dispatcher interface {
    io.Closer
    Send(payload []byte) error
}
```

**Built-in Implementations:**

- `HTTPDispatcher` - Production HTTP transport (default)
- `NoopDispatcher` - Logs to console in human-readable format (development/debug)

### Queue Interface

The `Queue` interface allows you to customize how messages are buffered:

```go
type Queue[T any] interface {
    io.Closer
    Enqueue(msg T) error
    Dequeue() (T, bool)
    TryDequeue() (T, bool)
    Drain() <-chan T
    Channel() <-chan T
    Len() int
    IsClosed() bool
}
```

**Built-in Implementations:**

- `ChannelQueue` - In-memory Go channel-based queue (default)

## Functional Options Pattern

The SDK uses the functional options pattern for configuration:

```go
// Simple usage with defaults
client := origamy.New("write-key")

// With custom options
client := origamy.New("write-key",
    origamy.WithDispatcher(origamy.NewNoopDispatcher(origamy.DispatcherConfig{})),
    origamy.WithQueue(customQueue),
    origamy.WithVerbose(true),
    origamy.WithBatchSize(100),
)
```

## Data Flow

1. **Enqueue**: Application calls `client.Enqueue(message)`
2. **Validate**: Message is validated
3. **Queue**: Message is pushed to the Queue
4. **Batch**: Event loop collects messages into batches
5. **Serialize**: Batch is JSON serialized
6. **Dispatch**: Dispatcher sends the payload (or logs it for NoopDispatcher)
7. **Retry**: On failure, retry with exponential backoff
8. **Callback**: Success/failure callbacks are invoked

## Data Flow

```mermaid
sequenceDiagram
    participant App as Application
    participant Client as Client
    participant Chan as Message Channel
    participant Loop as Event Loop
    participant Queue as Message Queue
    participant Exec as Executor
    participant HTTP as HTTP Client
    participant API as Segment API

    App->>Client: Enqueue(Track{...})
    Client->>Client: Validate message
    Client->>Client: Add messageId & timestamp
    Client->>Chan: Send message

    loop Event Loop
        Chan->>Loop: Receive message
        Loop->>Queue: Push to queue

        alt Batch size reached OR Interval tick
            Queue->>Loop: Flush messages
            Loop->>Exec: sendAsync(batch)
            Exec->>HTTP: POST /v1/batch
            HTTP->>API: JSON payload
            API-->>HTTP: Response

            alt Success
                HTTP-->>Client: notifySuccess
            else Failure (retry)
                HTTP->>HTTP: Wait (exponential backoff)
                HTTP->>API: Retry request
            end
        end
    end

    App->>Client: Close()
    Client->>Chan: Close channel
    Loop->>Loop: Drain remaining messages
    Loop->>Client: Shutdown complete
```

## Key Interfaces

### Client Interface

```go
type Client interface {
    io.Closer
    Enqueue(Message) error
}
```

The main API for interacting with the analytics library. Messages are queued and sent asynchronously in batches.

### Message Interface

```go
type Message interface {
    Validate() error
}
```

All message types (Track, Identify, Page, Screen, Group, Alias) implement this interface.

### Callback Interface

```go
type Callback interface {
    Success(Message)
    Failure(Message, error)
}
```

Optional interface for receiving notifications about message delivery status.

### Logger Interface

```go
type Logger interface {
    Logf(format string, args ...interface{})
    Errorf(format string, args ...interface{})
}
```

Customizable logging interface for operational visibility.

## Message Types

```mermaid
classDiagram
    class Message {
        <<interface>>
        +Validate() error
    }

    class Track {
        +Type string
        +MessageId string
        +UserId string
        +AnonymousId string
        +Event string
        +Timestamp time.Time
        +Context *Context
        +Properties Properties
        +Integrations Integrations
    }

    class Identify {
        +Type string
        +MessageId string
        +UserId string
        +AnonymousId string
        +Timestamp time.Time
        +Context *Context
        +Traits Traits
        +Integrations Integrations
    }

    class Page {
        +Type string
        +MessageId string
        +UserId string
        +AnonymousId string
        +Name string
        +Timestamp time.Time
        +Context *Context
        +Properties Properties
        +Integrations Integrations
    }

    class Screen {
        +Type string
        +MessageId string
        +UserId string
        +AnonymousId string
        +Name string
        +Timestamp time.Time
        +Context *Context
        +Properties Properties
        +Integrations Integrations
    }

    class Group {
        +Type string
        +MessageId string
        +UserId string
        +AnonymousId string
        +GroupId string
        +Timestamp time.Time
        +Context *Context
        +Traits Traits
        +Integrations Integrations
    }

    class Alias {
        +Type string
        +MessageId string
        +UserId string
        +PreviousId string
        +Timestamp time.Time
        +Context *Context
        +Integrations Integrations
    }

    Message <|.. Track
    Message <|.. Identify
    Message <|.. Page
    Message <|.. Screen
    Message <|.. Group
    Message <|.. Alias
```

## Internal Components

### Event Loop (`client.loop()`)

The event loop is the heart of the client, running in a dedicated goroutine:

```mermaid
stateDiagram-v2
    [*] --> Waiting

    Waiting --> ProcessMessage: Message received
    Waiting --> FlushTimer: Interval tick
    Waiting --> Shutdown: Close() called

    ProcessMessage --> PushToQueue: Validate & queue
    PushToQueue --> CheckBatch: Check batch limits
    CheckBatch --> SendBatch: Batch full
    CheckBatch --> Waiting: Continue waiting
    SendBatch --> Waiting: Async send

    FlushTimer --> FlushQueue: Flush pending
    FlushQueue --> Waiting: Continue

    Shutdown --> DrainChannel: Close message channel
    DrainChannel --> FlushRemaining: Process remaining
    FlushRemaining --> [*]: Signal shutdown complete
```

### Executor

Manages concurrent HTTP requests with a configurable limit:

```mermaid
graph TB
    subgraph Executor
        Queue[Task Queue]
        Counter[Size Counter]
        Cap[Capacity Limit]
    end

    Task1[Task 1] --> Queue
    Task2[Task 2] --> Queue
    Task3[Task 3] --> Queue

    Queue --> |spawn goroutine| Worker1[Worker]
    Queue --> |spawn goroutine| Worker2[Worker]

    Worker1 --> |done| Counter
    Worker2 --> |done| Counter

    Counter --> |check| Cap
```

### Message Queue

Batches messages based on count and byte size limits:

- **Max Batch Size**: 250 messages (default)
- **Max Batch Bytes**: ~500KB
- **Max Message Size**: Limited per message

## Configuration

```mermaid
graph LR
    subgraph Config
        Endpoint[Endpoint<br>Default: api.segment.io]
        Interval[Flush Interval<br>Default: 5s]
        BatchSize[Batch Size<br>Default: 250]
        Transport[HTTP Transport]
        Logger[Logger]
        Callback[Callback]
        Verbose[Verbose Mode]
        RetryAfter[Retry Policy]
        DefaultContext[Default Context]
    end

    Config --> Client
```

## Error Handling

```mermaid
graph TB
    subgraph "Error Types"
        ConfigError[ConfigError<br>Invalid configuration]
        FieldError[FieldError<br>Invalid field values]
        ErrClosed[ErrClosed<br>Client already closed]
        ErrTooManyRequests[ErrTooManyRequests<br>Rate limited]
        ErrMessageTooBig[ErrMessageTooBig<br>Message exceeds limit]
    end

    subgraph "Recovery"
        Retry[Exponential Backoff<br>Up to 10 attempts]
        Callback[Failure Callback]
        Log[Error Logging]
    end

    ConfigError --> Log
    FieldError --> Callback
    ErrClosed --> Callback
    ErrTooManyRequests --> Callback & Log
    ErrMessageTooBig --> Callback & Log

    HTTPError[HTTP Error] --> Retry
    Retry --> |Max retries| Callback & Log
```

## File Structure

```
origamy sdk/
├── analytics.go      # Core client implementation
├── config.go         # Configuration types and defaults
├── message.go        # Message queue and batch handling
├── executor.go       # Concurrent request executor
│
├── track.go          # Track message type
├── identify.go       # Identify message type
├── page.go           # Page message type
├── screen.go         # Screen message type
├── group.go          # Group message type
├── alias.go          # Alias message type
│
├── context.go        # Context metadata types
├── properties.go     # Properties helper type
├── traits.go         # Traits helper type
├── integrations.go   # Integrations control type
│
├── logger.go         # Logger interface
├── error.go          # Error types
├── validate.go       # Validation utilities
├── json.go           # JSON serialization helpers
│
├── timeout_15.go     # Go 1.5 timeout handling
├── timeout_16.go     # Go 1.6+ timeout handling
│
├── cmd/cli/          # CLI tool
├── examples/         # Usage examples
└── fixtures/         # Test fixtures
```

## Key Design Patterns

1. **Asynchronous Processing**: Messages are queued and processed in a background goroutine
2. **Batching**: Multiple messages are combined into single HTTP requests for efficiency
3. **Backpressure**: Channel-based flow control prevents memory exhaustion
4. **Graceful Shutdown**: `Close()` drains pending messages before terminating
5. **Retry with Backoff**: Failed requests are retried with exponential backoff
6. **Concurrent Execution**: Executor limits concurrent HTTP requests
7. **Callback Notifications**: Optional callbacks inform the application of success/failure
