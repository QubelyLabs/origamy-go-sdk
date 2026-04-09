## Installation

The package can be simply installed via go get, we recommend that you use a
package version management system like the Go vendor directory or a tool like
Godep to avoid issues related to API breaking changes introduced between major
versions of the library.

To install it in the GOPATH:

```
go get https://github.com/qubely/origamy-go-sdk
```

## Usage

### Basic Usage

```go
package main

import (
    "os"

    origamy "github.com/qubely/origamy-go-sdk"
)

func main() {
    // Instantiates a client to send messages to the Origamy API.
    client := origamy.New(os.Getenv("ORIGAMY_WRITE_KEY"))

    // Enqueues a track event that will be sent asynchronously.
    client.Enqueue(origamy.Track{
        UserId: "test-user",
        Event:  "test-snippet",
    })

    // Flushes any queued messages and closes the client.
    client.Close()
}
```

### With Options

The SDK supports functional options for flexible configuration:

```go
package main

import (
    "time"

    origamy "github.com/qubely/origamy-go-sdk"
)

func main() {
    // Create client with custom options
    client := origamy.New("your-write-key",
        origamy.WithVerbose(true),
        origamy.WithBatchSize(100),
        origamy.WithInterval(10 * time.Second),
    )
    defer client.Close()

    client.Enqueue(origamy.Track{
        UserId: "user-123",
        Event:  "purchase",
        Properties: origamy.NewProperties().
            SetRevenue(99.99).
            SetCurrency("USD"),
    })
}
```

### Development Mode (Noop Dispatcher)

For development and debugging, use the `NoopDispatcher` which logs events to the console in a human-readable format instead of sending them to the API:

```go
package main

import origamy "github.com/qubely/origamy-go-sdk"

func main() {
    // Create a noop dispatcher for development
    noopDispatcher := origamy.NewNoopDispatcher(origamy.DispatcherConfig{
        Verbose: true,
    })

    // Create client with noop dispatcher
    client := origamy.New("your-write-key",
        origamy.WithDispatcher(noopDispatcher),
    )
    defer client.Close()

    // Events will be logged to console instead of sent to API
    client.Enqueue(origamy.Track{
        UserId: "user-123",
        Event:  "button_clicked",
        Properties: origamy.NewProperties().
            Set("button", "signup"),
    })

    client.Enqueue(origamy.Identify{
        UserId: "user-123",
        Traits: origamy.NewTraits().
            SetEmail("user@example.com").
            SetName("John Doe"),
    })
}
```

### Custom Queue

For advanced use cases, you can provide a custom queue implementation:

```go
package main

import origamy "github.com/qubely/origamy-go-sdk"

func main() {
    // Create a custom queue with larger capacity
    queue := origamy.NewChannelQueue(
        origamy.QueueWithCapacity(1000),
    )

    client := origamy.New("your-write-key",
        origamy.WithQueue(queue),
    )
    defer client.Close()

    // Use the client as normal
}
```

### Custom Dispatcher

Implement the `Dispatcher` interface for custom transport mechanisms:

```go
type Dispatcher interface {
    io.Closer
    Send(payload []byte) error
}
```

Example with a custom dispatcher:

```go
type MyGRPCDispatcher struct {
    // your gRPC client fields
}

func (d *MyGRPCDispatcher) Send(payload []byte) error {
    // Send via gRPC
    return nil
}

func (d *MyGRPCDispatcher) Close() error {
    return nil
}

func main() {
    client := origamy.New("your-write-key",
        origamy.WithDispatcher(&MyGRPCDispatcher{}),
    )
    defer client.Close()
}
```

### Full Configuration

For complete control, use `NewWithConfig`:

```go
package main

import (
    "log"
    "os"
    "time"

    origamy "github.com/qubely/origamy-go-sdk"
)

func main() {
    client, err := origamy.NewWithConfig("your-write-key", origamy.Config{
        Endpoint:      "https://api.custom-endpoint.io",
        Interval:      30 * time.Second,
        BatchSize:     250,
        Verbose:       true,
        Logger:        origamy.StdLogger(log.New(os.Stderr, "origamy ", log.LstdFlags)),
        QueueCapacity: 500,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
}
```

## Available Options

| Option                    | Description                                    |
| ------------------------- | ---------------------------------------------- |
| `WithDispatcher(d)`       | Set custom dispatcher (HTTP, Noop, gRPC, etc.) |
| `WithQueue(q)`            | Set custom message queue                       |
| `WithEndpoint(url)`       | Set API endpoint URL                           |
| `WithInterval(d)`         | Set flush interval                             |
| `WithBatchSize(n)`        | Set max batch size                             |
| `WithVerbose(bool)`       | Enable verbose logging                         |
| `WithLogger(l)`           | Set custom logger                              |
| `WithCallback(c)`         | Set success/failure callbacks                  |
| `WithDefaultContext(ctx)` | Set default context for all messages           |
| `WithRetryAfter(fn)`      | Set custom retry policy                        |
| `WithQueueCapacity(n)`    | Set queue capacity (default queue only)        |

## License

The library is released under the [MIT license](License.md).
