# Origamy Go SDK — Integration Demo

A self-contained demo application that exercises every message type supported by
the Go SDK. It is designed for integration testing against the Origamy platform.

## How it works

By default the demo uses `NoopDispatcher`, which prints all batched events to
stdout instead of sending real HTTP requests. Set `ORIGAMY_WRITE_KEY` to a real
key and remove (or replace) `WithDispatcher(noop)` in `main.go` to send live
traffic.

## Run

```sh
# From this directory
go run .

# With a real write key
ORIGAMY_WRITE_KEY=your_key go run .
```

The demo covers all six message types:

| Type     | Purpose                                      |
|----------|----------------------------------------------|
| Identify | Attach traits (name, email, plan) to a user  |
| Track    | Record a custom named event                  |
| Page     | Record a web page view                       |
| Screen   | Record a mobile screen view                  |
| Group    | Associate a user with a company or workspace |
| Alias    | Merge an anonymous ID into a known user ID   |

## Configuration

Key options set in `main.go`:

| Option              | Value in demo       | Description                      |
|---------------------|---------------------|----------------------------------|
| `WithDispatcher`    | `NoopDispatcher`    | Prints to stdout, no HTTP        |
| `WithBatchSize`     | 10                  | Flush after 10 messages          |
| `WithInterval`      | 2 s                 | Flush every 2 seconds            |
| `WithCallback`      | `demoCallback`      | Logs success/failure per message |
| `WithDefaultContext`| app name + version  | Injected into every message      |
| `WithVerbose`       | true                | Detailed internal logging        |

Swap `WithDispatcher(noop)` for `WithDispatcher(origamy.NewHTTPDispatcher(...))`,
or omit `WithDispatcher` entirely, to target a live Origamy endpoint.
