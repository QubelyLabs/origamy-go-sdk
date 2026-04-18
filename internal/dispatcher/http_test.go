package dispatcher

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testLogger struct {
	logf   func(string, ...interface{})
	errorf func(string, ...interface{})
}

func (l testLogger) Logf(format string, args ...interface{}) {
	if l.logf != nil {
		l.logf(format, args...)
	}
}

func (l testLogger) Errorf(format string, args ...interface{}) {
	if l.errorf != nil {
		l.errorf(format, args...)
	}
}

func makePayload(t *testing.T, body interface{}) []byte {
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

func TestHTTPDispatcherSendsToCorrectEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
	}))
	defer server.Close()

	d := NewHTTPDispatcher(Config{
		Endpoint: server.URL,
		WriteKey: "wk",
	})

	payload := makePayload(t, map[string]interface{}{
		"batch":  []interface{}{},
		"sentAt": "2024-01-01T00:00:00.000Z",
	})

	if err := d.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotPath != "/v1/batch" {
		t.Errorf("path: got %s, want /v1/batch", gotPath)
	}
}

func TestHTTPDispatcherSetsBasicAuth(t *testing.T) {
	var gotUser, gotPass string
	var gotOK bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, gotOK = r.BasicAuth()
		w.WriteHeader(200)
	}))
	defer server.Close()

	d := NewHTTPDispatcher(Config{
		Endpoint: server.URL,
		WriteKey: "my-secret-key",
	})

	payload := makePayload(t, map[string]interface{}{
		"batch":  []interface{}{},
		"sentAt": "2024-01-01T00:00:00.000Z",
	})

	if err := d.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !gotOK {
		t.Error("missing Basic auth header")
	}
	if gotUser != "my-secret-key" {
		t.Errorf("auth user: got %q, want my-secret-key", gotUser)
	}
	if gotPass != "" {
		t.Errorf("auth password: got %q, want empty", gotPass)
	}
}

func TestHTTPDispatcherSetsContentType(t *testing.T) {
	var gotCT string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(200)
	}))
	defer server.Close()

	d := NewHTTPDispatcher(Config{Endpoint: server.URL, WriteKey: "wk"})
	payload := makePayload(t, map[string]interface{}{"batch": []interface{}{}, "sentAt": "2024-01-01T00:00:00.000Z"})

	if err := d.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type: got %q, want application/json", gotCT)
	}
}

func TestHTTPDispatcherSetsUserAgent(t *testing.T) {
	var gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(200)
	}))
	defer server.Close()

	d := NewHTTPDispatcher(Config{Endpoint: server.URL, WriteKey: "wk", Version: "3.0.0"})
	payload := makePayload(t, map[string]interface{}{"batch": []interface{}{}, "sentAt": "2024-01-01T00:00:00.000Z"})

	if err := d.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotUA != "origamy-go (version: 3.0.0)" {
		t.Errorf("User-Agent: got %q, want origamy-go (version: 3.0.0)", gotUA)
	}
}

func TestHTTPDispatcherSendsPayloadBody(t *testing.T) {
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer server.Close()

	d := NewHTTPDispatcher(Config{Endpoint: server.URL, WriteKey: "wk"})

	payload := makePayload(t, map[string]interface{}{
		"batch": []map[string]interface{}{
			{"type": "track", "event": "Test", "userId": "u1"},
		},
		"sentAt": "2024-01-01T00:00:00.000Z",
	})

	if err := d.Send(payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var received map[string]interface{}
	if err := json.Unmarshal(gotBody, &received); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	batch := received["batch"].([]interface{})
	if len(batch) != 1 {
		t.Errorf("batch length: got %d, want 1", len(batch))
	}
}

func TestHTTPDispatcherSuccessOn2xx(t *testing.T) {
	for _, code := range []int{200, 201, 204} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer server.Close()

			d := NewHTTPDispatcher(Config{Endpoint: server.URL, WriteKey: "wk"})
			payload := makePayload(t, map[string]interface{}{"batch": []interface{}{}, "sentAt": "2024-01-01T00:00:00.000Z"})

			if err := d.Send(payload); err != nil {
				t.Errorf("status %d should succeed, got error: %v", code, err)
			}
		})
	}
}

func TestHTTPDispatcherErrorOn4xx(t *testing.T) {
	for _, code := range []int{400, 401, 413, 429} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer server.Close()

			d := NewHTTPDispatcher(Config{Endpoint: server.URL, WriteKey: "wk"})
			payload := makePayload(t, map[string]interface{}{"batch": []interface{}{}, "sentAt": "2024-01-01T00:00:00.000Z"})

			if err := d.Send(payload); err == nil {
				t.Errorf("status %d should return error", code)
			}
		})
	}
}

func TestHTTPDispatcherErrorOn5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	d := NewHTTPDispatcher(Config{Endpoint: server.URL, WriteKey: "wk"})
	payload := makePayload(t, map[string]interface{}{"batch": []interface{}{}, "sentAt": "2024-01-01T00:00:00.000Z"})

	if err := d.Send(payload); err == nil {
		t.Error("5xx response should return error")
	}
}

func TestHTTPDispatcherTimeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer slow.Close()

	d := NewHTTPDispatcher(Config{Endpoint: slow.URL, WriteKey: "wk"},
		WithTimeout(50*time.Millisecond))

	payload := makePayload(t, map[string]interface{}{"batch": []interface{}{}, "sentAt": "2024-01-01T00:00:00.000Z"})

	if err := d.Send(payload); err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestHTTPDispatcherMalformedEndpoint(t *testing.T) {
	d := NewHTTPDispatcher(Config{Endpoint: "://bad-url", WriteKey: "wk"})
	payload := makePayload(t, map[string]interface{}{"batch": []interface{}{}, "sentAt": "2024-01-01T00:00:00.000Z"})

	if err := d.Send(payload); err == nil {
		t.Error("malformed endpoint should return error")
	}
}

func TestHTTPDispatcherCloseIsIdempotent(t *testing.T) {
	d := NewHTTPDispatcher(Config{Endpoint: "https://api.origamy.com", WriteKey: "wk"})
	if err := d.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestNoopDispatcherSendDoesNotError(t *testing.T) {
	d := NewNoopDispatcher(Config{}, WithColor(false))

	payload := makePayload(t, map[string]interface{}{
		"batch": []map[string]interface{}{
			{
				"type":      "track",
				"messageId": "msg-1",
				"userId":    "user-1",
				"event":     "Test Event",
				"timestamp": "2024-01-01T00:00:00Z",
			},
		},
		"sentAt": "2024-01-01T00:00:00.000Z",
	})

	if err := d.Send(payload); err != nil {
		t.Errorf("NoopDispatcher.Send should not error: %v", err)
	}
}

func TestNoopDispatcherHandlesMalformedPayload(t *testing.T) {
	d := NewNoopDispatcher(Config{}, WithColor(false))
	// Should not panic or return error for malformed input
	if err := d.Send([]byte("not-json")); err != nil {
		t.Errorf("NoopDispatcher should not error on malformed input: %v", err)
	}
}
