package dispatcher

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// HTTPDispatcher implements the Dispatcher interface using HTTP transport.
type HTTPDispatcher struct {
	config Config
	client http.Client
}

// HTTPOption is a function that configures an HTTPDispatcher.
type HTTPOption func(*HTTPDispatcher)

// WithTransport sets a custom HTTP transport for the dispatcher.
func WithTransport(transport http.RoundTripper) HTTPOption {
	return func(d *HTTPDispatcher) {
		d.client.Transport = transport
		if supportsTimeout(transport) {
			d.client.Timeout = 10 * time.Second
		}
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) HTTPOption {
	return func(d *HTTPDispatcher) {
		d.client.Timeout = timeout
	}
}

// NewHTTPDispatcher creates a new HTTP dispatcher with the given configuration.
func NewHTTPDispatcher(config Config, opts ...HTTPOption) *HTTPDispatcher {
	d := &HTTPDispatcher{
		config: config,
		client: http.Client{
			Transport: http.DefaultTransport,
			Timeout:   10 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// Send implements the Dispatcher interface by sending the payload via HTTP POST.
func (d *HTTPDispatcher) Send(payload []byte) error {
	url := d.config.Endpoint + "/v1/batch"
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		d.errorf("creating request - %s", err)
		return err
	}

	version := d.config.Version
	if version == "" {
		version = "3.0.0"
	}
	req.Header.Add("User-Agent", "origamy-go (version: "+version+")")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Content-Length", strconv.Itoa(len(payload)))
	req.SetBasicAuth(d.config.WriteKey, "")

	res, err := d.client.Do(req)
	if err != nil {
		d.errorf("sending request - %s", err)
		return err
	}
	defer res.Body.Close()

	return d.handleResponse(res)
}

// Close implements the Dispatcher interface. HTTP dispatcher doesn't require cleanup.
func (d *HTTPDispatcher) Close() error {
	return nil
}

// handleResponse processes the HTTP response and returns an error for non-success status codes.
func (d *HTTPDispatcher) handleResponse(res *http.Response) error {
	if res.StatusCode < 300 {
		d.debugf("response %s", res.Status)
		return nil
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		d.errorf("response %d %s - %s", res.StatusCode, res.Status, err)
		return err
	}

	d.logf("response %d %s – %s", res.StatusCode, res.Status, string(body))
	return fmt.Errorf("%d %s", res.StatusCode, res.Status)
}

func (d *HTTPDispatcher) debugf(format string, args ...interface{}) {
	if d.config.Verbose && d.config.Logger != nil {
		d.config.Logger.Logf(format, args...)
	}
}

func (d *HTTPDispatcher) logf(format string, args ...interface{}) {
	if d.config.Logger != nil {
		d.config.Logger.Logf(format, args...)
	}
}

func (d *HTTPDispatcher) errorf(format string, args ...interface{}) {
	if d.config.Logger != nil {
		d.config.Logger.Errorf(format, args...)
	}
}
