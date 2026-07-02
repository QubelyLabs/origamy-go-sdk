package core

// Comprehensive tests with realistic sample events covering all message types,
// HTTP wire format, and common analytics patterns.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// captureServer records the raw HTTP request for inspection.
type capturedRequest struct {
	method      string
	path        string
	contentType string
	authUser    string
	authPass    string
	authOK      bool
	body        map[string]interface{}
}

func captureServer() (chan capturedRequest, *httptest.Server) {
	ch := make(chan capturedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		ch <- capturedRequest{
			method:      r.Method,
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			authUser:    user,
			authPass:    pass,
			authOK:      ok,
			body:        body,
		}
		w.WriteHeader(200)
	}))
	return ch, server
}

// ---- HTTP wire format tests ----

func TestHTTPRequestFormat(t *testing.T) {
	ch, server := captureServer()
	defer server.Close()

	client, _ := NewWithConfig("my-write-key", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	client.Enqueue(Track{UserId: "u1", Event: "Test"})
	client.Close()

	req := <-ch

	if req.method != "POST" {
		t.Errorf("method: got %s, want POST", req.method)
	}
	if req.path != "/v1/batch" {
		t.Errorf("path: got %s, want /v1/batch", req.path)
	}
	if req.contentType != "application/json" {
		t.Errorf("Content-Type: got %s, want application/json", req.contentType)
	}
	if !req.authOK {
		t.Error("missing Basic auth header")
	}
	if req.authUser != "my-write-key" {
		t.Errorf("auth user: got %s, want my-write-key", req.authUser)
	}
	if req.authPass != "" {
		t.Errorf("auth password: got %q, want empty string", req.authPass)
	}
}

func TestBatchBodyStructure(t *testing.T) {
	ch, server := captureServer()
	defer server.Close()

	client, _ := NewWithConfig("wk", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	client.Enqueue(Track{UserId: "u1", Event: "Test"})
	client.Close()

	req := <-ch
	body := req.body

	// Must have batch array
	batch, ok := body["batch"].([]interface{})
	if !ok || len(batch) == 0 {
		t.Fatal("missing or empty 'batch' array in request body")
	}

	// Must have sentAt in ISO 8601 format with milliseconds
	sentAt, _ := body["sentAt"].(string)
	if !strings.HasSuffix(sentAt, "Z") || !strings.Contains(sentAt, ".") {
		t.Errorf("sentAt %q should be ISO 8601 with milliseconds (e.g. 2009-11-10T23:00:00.000Z)", sentAt)
	}

	// Must NOT have messageId or context at batch level (per Web SDK spec)
	if _, exists := body["messageId"]; exists {
		t.Error("batch body must NOT contain top-level 'messageId' (should be per-event)")
	}
	if _, exists := body["context"]; exists {
		t.Error("batch body must NOT contain top-level 'context' (should be per-event)")
	}
}

func TestContextIsPerEvent(t *testing.T) {
	ch, server := captureServer()
	defer server.Close()

	client, _ := NewWithConfig("wk", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	client.Enqueue(Track{UserId: "u1", Event: "Test"})
	client.Close()

	req := <-ch
	batch := req.body["batch"].([]interface{})
	event := batch[0].(map[string]interface{})

	ctx, ok := event["context"].(map[string]interface{})
	if !ok {
		t.Fatal("event missing 'context' field")
	}
	lib, ok := ctx["library"].(map[string]interface{})
	if !ok {
		t.Fatal("event context missing 'library'")
	}
	if lib["name"] != "origamy-go" {
		t.Errorf("library name: got %v, want origamy-go", lib["name"])
	}
	if lib["version"] != Version {
		t.Errorf("library version: got %v, want %s", lib["version"], Version)
	}
}

func TestDefaultEndpointIsOrigamy(t *testing.T) {
	if DefaultEndpoint != "https://events.origamy.io" {
		t.Errorf("DefaultEndpoint: got %s, want https://events.origamy.io", DefaultEndpoint)
	}
}

// ---- Realistic sample event tests ----

// TestEcommerceCheckoutFlow simulates a real e-commerce session:
// identify → product viewed → add to cart → order completed.
func TestEcommerceCheckoutFlow(t *testing.T) {
	type result struct {
		eventType string
		body      map[string]interface{}
	}
	results := make(chan result, 4)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		batch := body["batch"].([]interface{})
		for _, e := range batch {
			ev := e.(map[string]interface{})
			results <- result{ev["type"].(string), body}
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	client, _ := NewWithConfig("shop-write-key", Config{
		Endpoint:  server.URL,
		BatchSize: 10,
		now:       mockTime,
		uid:       mockId,
	})

	client.Enqueue(Identify{
		UserId: "customer-42",
		Traits: NewTraits().
			SetEmail("alice@example.com").
			SetName("Alice Smith").
			Set("plan", "premium").
			Set("signedUpAt", "2024-01-01T00:00:00Z"),
	})

	client.Enqueue(Track{
		UserId: "customer-42",
		Event:  "Product Viewed",
		Properties: NewProperties().
			Set("productId", "prod-789").
			Set("name", "Running Shoes").
			Set("category", "Footwear").
			SetPrice(89.99).
			SetCurrency("USD"),
	})

	client.Enqueue(Track{
		UserId: "customer-42",
		Event:  "Product Added",
		Properties: NewProperties().
			Set("productId", "prod-789").
			Set("name", "Running Shoes").
			Set("quantity", 1).
			SetPrice(89.99).
			SetCurrency("USD"),
	})

	client.Enqueue(Track{
		UserId: "customer-42",
		Event:  "Order Completed",
		Properties: NewProperties().
			Set("orderId", "order-001").
			SetRevenue(89.99).
			SetCurrency("USD").
			Set("products", []map[string]interface{}{
				{"productId": "prod-789", "name": "Running Shoes", "price": 89.99, "quantity": 1},
			}),
	})

	client.Close()

	seen := map[string]bool{}
	timeout := time.After(3 * time.Second)
	for i := 0; i < 4; i++ {
		select {
		case r := <-results:
			seen[r.eventType] = true
		case <-timeout:
			t.Fatal("timed out waiting for events")
		}
	}

	for _, typ := range []string{"identify", "track"} {
		if !seen[typ] {
			t.Errorf("expected event type %q not received", typ)
		}
	}
}

// TestUserSignupFlow simulates anonymous browsing → signup → identify.
func TestUserSignupFlow(t *testing.T) {
	body, server := mockServer()
	defer server.Close()

	client, _ := NewWithConfig("app-key", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	defer client.Close()

	// Anonymous user signs up – now has both anonymousId and userId
	client.Enqueue(Identify{
		UserId:      "new-user-99",
		AnonymousId: "anon-browser-xyz",
		Traits: NewTraits().
			SetEmail("bob@example.com").
			SetFirstName("Bob").
			SetLastName("Jones").
			Set("company", "Acme Inc").
			Set("createdAt", "2024-06-01T12:00:00Z"),
	})

	res := <-body
	var parsed map[string]interface{}
	json.Unmarshal([]byte(res), &parsed)

	batch := parsed["batch"].([]interface{})
	if len(batch) != 1 {
		t.Fatalf("expected 1 event, got %d", len(batch))
	}

	event := batch[0].(map[string]interface{})
	if event["type"] != "identify" {
		t.Errorf("type: got %v, want identify", event["type"])
	}
	if event["userId"] != "new-user-99" {
		t.Errorf("userId: got %v, want new-user-99", event["userId"])
	}
	if event["anonymousId"] != "anon-browser-xyz" {
		t.Errorf("anonymousId: got %v, want anon-browser-xyz", event["anonymousId"])
	}

	traits := event["traits"].(map[string]interface{})
	if traits["email"] != "bob@example.com" {
		t.Errorf("email trait: got %v, want bob@example.com", traits["email"])
	}
}

// TestAnonymousUserTracking verifies events sent with only anonymousId (pre-login).
func TestAnonymousUserTracking(t *testing.T) {
	body, server := mockServer()
	defer server.Close()

	client, _ := NewWithConfig("wk", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	defer client.Close()

	client.Enqueue(Track{
		AnonymousId: "anon-abc123",
		Event:       "Page Scrolled",
		Properties: Properties{
			"depth":     75,
			"direction": "down",
		},
	})

	res := <-body
	var parsed map[string]interface{}
	json.Unmarshal([]byte(res), &parsed)

	batch := parsed["batch"].([]interface{})
	event := batch[0].(map[string]interface{})

	if event["anonymousId"] != "anon-abc123" {
		t.Errorf("anonymousId: got %v, want anon-abc123", event["anonymousId"])
	}
	if _, hasUser := event["userId"]; hasUser {
		t.Error("userId should be absent for anonymous events")
	}
}

// TestIdentifyWithFullTraits verifies all standard Segment traits are sent correctly.
func TestIdentifyWithFullTraits(t *testing.T) {
	body, server := mockServer()
	defer server.Close()

	client, _ := NewWithConfig("wk", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	defer client.Close()

	client.Enqueue(Identify{
		UserId: "user-full",
		Traits: NewTraits().
			SetEmail("full@example.com").
			SetFirstName("Jane").
			SetLastName("Doe").
			SetName("Jane Doe").
			SetPhone("+15551234567").
			SetUsername("janedoe").
			SetWebsite("https://jane.dev").
			SetAvatar("https://cdn.example.com/avatar.jpg").
			SetAge(28).
			SetGender("female").
			SetBirthday(time.Date(1996, 3, 15, 0, 0, 0, 0, time.UTC)).
			Set("plan", "enterprise").
			Set("logins", 42),
	})

	res := <-body
	var parsed map[string]interface{}
	json.Unmarshal([]byte(res), &parsed)

	batch := parsed["batch"].([]interface{})
	event := batch[0].(map[string]interface{})
	traits := event["traits"].(map[string]interface{})

	checks := map[string]interface{}{
		"email":     "full@example.com",
		"firstName": "Jane",
		"lastName":  "Doe",
		"name":      "Jane Doe",
		"phone":     "+15551234567",
		"username":  "janedoe",
		"website":   "https://jane.dev",
	}
	for field, want := range checks {
		if got := traits[field]; got != want {
			t.Errorf("trait %s: got %v, want %v", field, got, want)
		}
	}
}

// TestPageEventWithProperties verifies a page view with URL context.
func TestPageEventWithProperties(t *testing.T) {
	body, server := mockServer()
	defer server.Close()

	client, _ := NewWithConfig("wk", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	defer client.Close()

	client.Enqueue(Page{
		UserId: "user-1",
		Name:   "Pricing",
		Properties: Properties{
			"url":      "https://example.com/pricing",
			"path":     "/pricing",
			"title":    "Pricing Plans",
			"referrer": "https://google.com",
			"search":   "?plan=pro",
		},
		Context: &Context{
			Page: PageInfo{
				URL:      "https://example.com/pricing",
				Path:     "/pricing",
				Title:    "Pricing Plans",
				Referrer: "https://google.com",
			},
			UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
			Locale:    "en-US",
		},
	})

	res := <-body
	var parsed map[string]interface{}
	json.Unmarshal([]byte(res), &parsed)

	batch := parsed["batch"].([]interface{})
	event := batch[0].(map[string]interface{})

	if event["type"] != "page" {
		t.Errorf("type: got %v, want page", event["type"])
	}
	if event["name"] != "Pricing" {
		t.Errorf("name: got %v, want Pricing", event["name"])
	}

	props := event["properties"].(map[string]interface{})
	if props["url"] != "https://example.com/pricing" {
		t.Errorf("url property: got %v", props["url"])
	}
}

// TestScreenEventWithProperties verifies a mobile screen view.
func TestScreenEventWithProperties(t *testing.T) {
	body, server := mockServer()
	defer server.Close()

	client, _ := NewWithConfig("wk", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	defer client.Close()

	client.Enqueue(Screen{
		UserId: "mobile-user-7",
		Name:   "Dashboard",
		Properties: Properties{
			"category": "Main",
			"tab":      "overview",
		},
		Context: &Context{
			OS:     OSInfo{Name: "iOS", Version: "17.2"},
			Device: DeviceInfo{Manufacturer: "Apple", Model: "iPhone 15", Type: "mobile"},
			Screen: ScreenInfo{Width: 390, Height: 844, Density: 3},
		},
	})

	res := <-body
	var parsed map[string]interface{}
	json.Unmarshal([]byte(res), &parsed)

	batch := parsed["batch"].([]interface{})
	event := batch[0].(map[string]interface{})

	if event["type"] != "screen" {
		t.Errorf("type: got %v, want screen", event["type"])
	}
	if event["name"] != "Dashboard" {
		t.Errorf("name: got %v, want Dashboard", event["name"])
	}

	ctx := event["context"].(map[string]interface{})
	os := ctx["os"].(map[string]interface{})
	if os["name"] != "iOS" {
		t.Errorf("os.name: got %v, want iOS", os["name"])
	}
}

// TestGroupEventWithCompanyTraits verifies a group association with company data.
func TestGroupEventWithCompanyTraits(t *testing.T) {
	body, server := mockServer()
	defer server.Close()

	client, _ := NewWithConfig("wk", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	defer client.Close()

	client.Enqueue(Group{
		UserId:  "employee-55",
		GroupId: "company-acme",
		Traits: NewTraits().
			SetName("Acme Corp").
			SetEmail("contact@acme.com").
			SetWebsite("https://acme.com").
			Set("industry", "Software").
			Set("employees", 500).
			Set("plan", "enterprise").
			Set("mrr", 12000),
	})

	res := <-body
	var parsed map[string]interface{}
	json.Unmarshal([]byte(res), &parsed)

	batch := parsed["batch"].([]interface{})
	event := batch[0].(map[string]interface{})

	if event["type"] != "group" {
		t.Errorf("type: got %v, want group", event["type"])
	}
	if event["groupId"] != "company-acme" {
		t.Errorf("groupId: got %v, want company-acme", event["groupId"])
	}

	traits := event["traits"].(map[string]interface{})
	if traits["name"] != "Acme Corp" {
		t.Errorf("name trait: got %v, want Acme Corp", traits["name"])
	}
	if traits["website"] != "https://acme.com" {
		t.Errorf("website trait: got %v, want https://acme.com", traits["website"])
	}
}

// TestAliasFlow verifies linking an anonymous ID to an identified user.
func TestAliasFlow(t *testing.T) {
	body, server := mockServer()
	defer server.Close()

	client, _ := NewWithConfig("wk", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	defer client.Close()

	client.Enqueue(Alias{
		UserId:     "identified-user-1",
		PreviousId: "anon-session-xyz",
	})

	res := <-body
	var parsed map[string]interface{}
	json.Unmarshal([]byte(res), &parsed)

	batch := parsed["batch"].([]interface{})
	event := batch[0].(map[string]interface{})

	if event["type"] != "alias" {
		t.Errorf("type: got %v, want alias", event["type"])
	}
	if event["userId"] != "identified-user-1" {
		t.Errorf("userId: got %v, want identified-user-1", event["userId"])
	}
	if event["previousId"] != "anon-session-xyz" {
		t.Errorf("previousId: got %v, want anon-session-xyz", event["previousId"])
	}
}

// TestTrackWithRichContext verifies per-event context overrides the default.
func TestTrackWithRichContext(t *testing.T) {
	body, server := mockServer()
	defer server.Close()

	client, _ := NewWithConfig("wk", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	defer client.Close()

	client.Enqueue(Track{
		UserId: "user-ctx",
		Event:  "Button Clicked",
		Properties: Properties{
			"button": "upgrade",
			"page":   "/dashboard",
		},
		Context: &Context{
			Library:   LibraryInfo{Name: "origamy-go", Version: Version},
			UserAgent: "Go-http-client/2.0",
			Locale:    "en-GB",
			Timezone:  "Europe/London",
			Campaign: CampaignInfo{
				Name:   "spring-promo",
				Source: "email",
				Medium: "newsletter",
			},
		},
	})

	res := <-body
	var parsed map[string]interface{}
	json.Unmarshal([]byte(res), &parsed)

	batch := parsed["batch"].([]interface{})
	event := batch[0].(map[string]interface{})
	ctx := event["context"].(map[string]interface{})

	if ctx["locale"] != "en-GB" {
		t.Errorf("locale: got %v, want en-GB", ctx["locale"])
	}
	if ctx["timezone"] != "Europe/London" {
		t.Errorf("timezone: got %v, want Europe/London", ctx["timezone"])
	}

	campaign := ctx["campaign"].(map[string]interface{})
	if campaign["name"] != "spring-promo" {
		t.Errorf("campaign.name: got %v, want spring-promo", campaign["name"])
	}
	if campaign["source"] != "email" {
		t.Errorf("campaign.source: got %v, want email", campaign["source"])
	}
}

// TestTrackWithEcommerceProperties exercises the typed Properties helpers.
func TestTrackWithEcommerceProperties(t *testing.T) {
	body, server := mockServer()
	defer server.Close()

	client, _ := NewWithConfig("wk", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	defer client.Close()

	client.Enqueue(Track{
		UserId: "buyer-7",
		Event:  "Order Completed",
		Properties: NewProperties().
			Set("orderId", "ORDER-9999").
			SetRevenue(249.95).
			SetCurrency("EUR").
			SetDiscount(25.00).
			SetShipping(9.99).
			SetTax(20.00).
			Set("products", []map[string]interface{}{
				{"sku": "SKU-A", "name": "Widget Pro", "price": 199.99, "quantity": 1},
				{"sku": "SKU-B", "name": "Widget Case", "price": 49.96, "quantity": 1},
			}),
	})

	res := <-body
	var parsed map[string]interface{}
	json.Unmarshal([]byte(res), &parsed)

	batch := parsed["batch"].([]interface{})
	event := batch[0].(map[string]interface{})
	props := event["properties"].(map[string]interface{})

	if props["revenue"] != 249.95 {
		t.Errorf("revenue: got %v, want 249.95", props["revenue"])
	}
	if props["currency"] != "EUR" {
		t.Errorf("currency: got %v, want EUR", props["currency"])
	}
	products := props["products"].([]interface{})
	if len(products) != 2 {
		t.Errorf("products count: got %d, want 2", len(products))
	}
}

// TestMultipleEventsInOneBatch verifies that multiple events share one HTTP request.
func TestMultipleEventsInOneBatch(t *testing.T) {
	requests := make(chan int, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		batch := body["batch"].([]interface{})
		requests <- len(batch)
		w.WriteHeader(200)
	}))
	defer server.Close()

	client, _ := NewWithConfig("wk", Config{
		Endpoint:  server.URL,
		BatchSize: 5,
		now:       mockTime,
		uid:       mockId,
	})

	for i := 0; i < 5; i++ {
		client.Enqueue(Track{
			UserId: "user-bulk",
			Event:  "Step Completed",
			Properties: Properties{"step": i},
		})
	}
	client.Close()

	count := <-requests
	if count != 5 {
		t.Errorf("expected 5 events in one batch, got %d", count)
	}
}

// TestSentAtFormat verifies that sentAt uses ISO 8601 with milliseconds, matching the Web SDK.
func TestSentAtFormat(t *testing.T) {
	ch, server := captureServer()
	defer server.Close()

	client, _ := NewWithConfig("wk", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	client.Enqueue(Track{UserId: "u1", Event: "Test"})
	client.Close()

	req := <-ch
	sentAt, _ := req.body["sentAt"].(string)

	// Web SDK uses new Date().toISOString() which produces "YYYY-MM-DDTHH:mm:ss.sssZ"
	if sentAt != "2009-11-10T23:00:00.000Z" {
		t.Errorf("sentAt: got %q, want 2009-11-10T23:00:00.000Z", sentAt)
	}
}

// TestCustomTimestampOnEvent verifies a caller-supplied timestamp is respected.
func TestCustomTimestampOnEvent(t *testing.T) {
	body, server := mockServer()
	defer server.Close()

	client, _ := NewWithConfig("wk", Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	defer client.Close()

	customTime := time.Date(2023, time.December, 25, 9, 0, 0, 0, time.UTC)
	client.Enqueue(Track{
		UserId:    "user-ts",
		Event:     "Christmas Present Opened",
		Timestamp: customTime,
	})

	res := <-body
	var parsed map[string]interface{}
	json.Unmarshal([]byte(res), &parsed)

	batch := parsed["batch"].([]interface{})
	event := batch[0].(map[string]interface{})

	if event["timestamp"] != "2023-12-25T09:00:00Z" {
		t.Errorf("timestamp: got %v, want 2023-12-25T09:00:00Z", event["timestamp"])
	}
}

// TestValidationRejectsInvalidMessages verifies that required fields are enforced.
func TestValidationRejectsInvalidMessages(t *testing.T) {
	client := New("wk")
	defer client.Close()

	cases := []struct {
		name string
		msg  Message
	}{
		{"Track missing Event", Track{UserId: "u1"}},
		{"Track missing user ID", Track{Event: "E"}},
		{"Group missing GroupId", Group{UserId: "u1"}},
		{"Alias missing UserId", Alias{PreviousId: "p1"}},
		{"Alias missing PreviousId", Alias{UserId: "u1"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := client.Enqueue(tc.msg); err == nil {
				t.Errorf("%s: expected validation error, got nil", tc.name)
			}
		})
	}
}

// TestWriteKeyIsCorrectlyEncoded verifies that the write key round-trips through Basic auth.
func TestWriteKeyIsCorrectlyEncoded(t *testing.T) {
	specialKey := "wk+special/chars==abc123"
	ch, server := captureServer()
	defer server.Close()

	client, _ := NewWithConfig(specialKey, Config{
		Endpoint:  server.URL,
		BatchSize: 1,
		now:       mockTime,
		uid:       mockId,
	})
	client.Enqueue(Track{UserId: "u1", Event: "E"})
	client.Close()

	req := <-ch
	if !req.authOK {
		t.Fatal("missing Basic auth header")
	}
	if req.authUser != specialKey {
		t.Errorf("auth user: got %q, want %q", req.authUser, specialKey)
	}
	if req.authPass != "" {
		t.Errorf("auth password should be empty, got %q", req.authPass)
	}
}
