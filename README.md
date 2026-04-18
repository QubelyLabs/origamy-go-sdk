## Installation

```
go get github.com/qubely/origamy-go-sdk
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
    client := origamy.New(os.Getenv("ORIGAMY_WRITE_KEY"))
    defer client.Close()

    client.Enqueue(origamy.Track{
        UserId: "user-123",
        Event:  "Signed Up",
    })
}
```

### Message Types

All six Segment-compatible message types are supported:

```go
// Track an action
client.Enqueue(origamy.Track{
    UserId: "user-123",
    Event:  "Order Completed",
    Properties: origamy.NewProperties().
        Set("orderId", "ORD-9999").
        SetRevenue(99.99).
        SetCurrency("USD"),
})

// Identify a user with traits
client.Enqueue(origamy.Identify{
    UserId: "user-123",
    Traits: origamy.NewTraits().
        SetEmail("alice@example.com").
        SetName("Alice Smith").
        Set("plan", "pro"),
})

// Track a page view
client.Enqueue(origamy.Page{
    UserId: "user-123",
    Name:   "Pricing",
    Properties: origamy.Properties{
        "url":   "https://example.com/pricing",
        "title": "Pricing Plans",
    },
})

// Track a mobile screen view
client.Enqueue(origamy.Screen{
    UserId: "user-123",
    Name:   "Dashboard",
    Properties: origamy.Properties{"tab": "overview"},
})

// Associate a user with a group/company
client.Enqueue(origamy.Group{
    UserId:  "user-123",
    GroupId: "company-acme",
    Traits: origamy.NewTraits().
        SetName("Acme Corp").
        SetWebsite("https://acme.com"),
})

// Alias an anonymous ID to an identified user
client.Enqueue(origamy.Alias{
    UserId:     "user-123",
    PreviousId: "anon-session-abc",
})
```

### Anonymous Users

Pass `AnonymousId` instead of (or in addition to) `UserId` for anonymous tracking:

```go
client.Enqueue(origamy.Track{
    AnonymousId: "anon-browser-xyz",
    Event:       "Page Scrolled",
    Properties:  origamy.Properties{"depth": 75},
})
```

### With Options

```go
client := origamy.New("your-write-key",
    origamy.WithVerbose(true),
    origamy.WithBatchSize(100),
    origamy.WithInterval(10 * time.Second),
    origamy.WithEndpoint("https://api.origamy.com"),
)
defer client.Close()
```

### Development Mode (Noop Dispatcher)

For local development, use `NoopDispatcher` to log events to the console instead of sending them:

```go
noopDispatcher := origamy.NewNoopDispatcher(origamy.DispatcherConfig{
    Verbose: true,
})

client := origamy.New("your-write-key",
    origamy.WithDispatcher(noopDispatcher),
)
defer client.Close()

client.Enqueue(origamy.Track{
    UserId: "user-123",
    Event:  "button_clicked",
    Properties: origamy.NewProperties().Set("button", "signup"),
})
```

### Custom Queue

```go
queue := origamy.NewChannelQueue(
    origamy.QueueWithCapacity(1000),
)

client := origamy.New("your-write-key",
    origamy.WithQueue(queue),
)
defer client.Close()
```

### Custom Dispatcher

Implement the `Dispatcher` interface for custom transport:

```go
type Dispatcher interface {
    io.Closer
    Send(payload []byte) error
}
```

```go
type MyGRPCDispatcher struct{}

func (d *MyGRPCDispatcher) Send(payload []byte) error {
    // Send via gRPC
    return nil
}

func (d *MyGRPCDispatcher) Close() error { return nil }

client := origamy.New("your-write-key",
    origamy.WithDispatcher(&MyGRPCDispatcher{}),
)
defer client.Close()
```

### Full Configuration

```go
client, err := origamy.NewWithConfig("your-write-key", origamy.Config{
    Endpoint:      "https://api.origamy.com",
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
```

## HTTP Wire Format

Events are batched and sent as a single HTTP POST to `/v1/batch`. The request body follows the same format as the Origamy Web SDK:

```json
{
  "batch": [
    {
      "type": "track",
      "messageId": "uuid",
      "userId": "user-123",
      "event": "Order Completed",
      "timestamp": "2024-01-15T10:30:00Z",
      "properties": { "revenue": 99.99 },
      "context": {
        "library": { "name": "origamy-go", "version": "3.0.0" }
      }
    }
  ],
  "sentAt": "2024-01-15T10:30:00.123Z"
}
```

Context is attached per-event (not at the batch level). `sentAt` uses ISO 8601 with milliseconds, matching `new Date().toISOString()` from the Web SDK.

Authentication uses HTTP Basic Auth with the write key as the username and an empty password.

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

## Defaults

| Setting         | Default                     |
| --------------- | --------------------------- |
| Endpoint        | `https://api.origamy.com`   |
| Flush interval  | 5 seconds                   |
| Batch size      | 250 messages                |
| Queue capacity  | 100 messages                |
| Request timeout | 10 seconds                  |
| Retry attempts  | 10 (exponential backoff)    |

## License

The library is released under the [MIT license](License.md).
