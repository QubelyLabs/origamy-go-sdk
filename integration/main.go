package main

import (
	"fmt"
	"log"
	"os"
	"time"

	origamy "github.com/qubely/origamy-go-sdk"
)

// demoCallback logs success and failure events from the SDK.
type demoCallback struct{}

func (c *demoCallback) Success(msg origamy.Message) {
	fmt.Printf("[callback] sent:   %T\n", msg)
}

func (c *demoCallback) Failure(msg origamy.Message, err error) {
	fmt.Printf("[callback] failed: %T — %v\n", msg, err)
}

func main() {
	writeKey := os.Getenv("ORIGAMY_WRITE_KEY")
	if writeKey == "" {
		writeKey = "demo-write-key"
		log.Println("ORIGAMY_WRITE_KEY not set — using NoopDispatcher (no HTTP requests)")
	}

	// Use NoopDispatcher so the demo prints events to stdout without hitting the API.
	// Swap in origamy.NewHTTPDispatcher(...) (or omit WithDispatcher) to send real traffic.
	noop := origamy.NewNoopDispatcher(origamy.DispatcherConfig{
		Endpoint: origamy.DefaultEndpoint,
		WriteKey: writeKey,
		Version:  origamy.Version,
	})

	client := origamy.New(
		writeKey,
		origamy.WithDispatcher(noop),
		origamy.WithVerbose(true),
		origamy.WithBatchSize(10),
		origamy.WithInterval(2*time.Second),
		origamy.WithCallback(&demoCallback{}),
		origamy.WithDefaultContext(&origamy.Context{
			App: origamy.AppInfo{
				Name:    "origamy-go-integration",
				Version: "1.0.0",
			},
		}),
	)
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("close: %v", err)
		}
	}()

	// Identify — attach traits to a known user.
	mustEnqueue(client, origamy.Identify{
		UserId: "user-123",
		Traits: origamy.NewTraits().
			SetName("Test User").
			SetEmail("test@origamy.com").
			Set("role", "integration-tester"),
	})

	// Track — record a custom event.
	mustEnqueue(client, origamy.Track{
		UserId: "user-123",
		Event:  "Integration Test Started",
		Properties: origamy.NewProperties().
			Set("sdk", "go").
			Set("version", origamy.Version),
	})

	// Page — record a web page view.
	mustEnqueue(client, origamy.Page{
		UserId: "user-123",
		Name:   "Integration Dashboard",
		Properties: origamy.NewProperties().
			Set("url", "https://origamy.com/dashboard").
			Set("path", "/dashboard"),
	})

	// Screen — record a mobile screen view.
	mustEnqueue(client, origamy.Screen{
		UserId: "user-123",
		Name:   "Settings Screen",
	})

	// Group — associate a user with a company or workspace.
	mustEnqueue(client, origamy.Group{
		UserId:  "user-123",
		GroupId: "team-origamy",
		Traits: origamy.NewTraits().
			Set("name", "Origamy Team").
			Set("plan", "enterprise"),
	})

	// Anonymous track — no userId required.
	mustEnqueue(client, origamy.Track{
		AnonymousId: "anon-session-abc",
		Event:       "Landing Page Visited",
		Properties: origamy.NewProperties().
			Set("referrer", "https://google.com"),
	})

	// Alias — merge an anonymous identity into a known user.
	mustEnqueue(client, origamy.Alias{
		UserId:     "user-123",
		PreviousId: "anon-session-abc",
	})

	fmt.Println("\nAll messages enqueued — flushing on close.")
}

func mustEnqueue(client origamy.Client, msg origamy.Message) {
	if err := client.Enqueue(msg); err != nil {
		log.Printf("enqueue %T: %v", msg, err)
	}
}
