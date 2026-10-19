package origamy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// MaxSegmentMembersPerRequest is the most rows the API takes in one request.
// AddMembers and RemoveMembers split longer lists into several requests.
const MaxSegmentMembersPerRequest = 1000

// Error codes a SegmentError can carry.
const (
	SegmentErrNotFound    = "segment_not_found" // no segment with this id in the key's workspace
	SegmentErrNotList     = "segment_not_list"  // the segment is computed from conditions
	SegmentErrManaged     = "segment_managed"   // the segment mirrors a group or status
	SegmentErrInvalid     = "invalid_request"
	SegmentErrUnavailable = "unavailable"
)

// SegmentMember names one person of a list segment. Set at least one of
// UserID, Email or Phone. Traits are stored on the person's profile (adds
// only).
type SegmentMember struct {
	UserID string         `json:"userId,omitempty"`
	Email  string         `json:"email,omitempty"`
	Phone  string         `json:"phone,omitempty"`
	Traits map[string]any `json:"traits,omitempty"`
}

// RejectedSegmentMember is a row the API did not use. Index is the row's
// position in the slice passed to AddMembers or RemoveMembers.
type RejectedSegmentMember struct {
	List  string `json:"list"` // "add" or "remove"
	Index int    `json:"index"`
	Error string `json:"error"`
}

// SegmentMembersResult reports a membership change. Added counts rows queued
// for adding: the people appear in the segment once the event pipeline has
// processed them, normally within seconds. NotFound counts removals that
// matched nobody in the segment.
type SegmentMembersResult struct {
	SegmentID int64                   `json:"segment_id"`
	Added     int                     `json:"added"`
	Removed   int                     `json:"removed"`
	NotFound  int                     `json:"not_found"`
	Rejected  []RejectedSegmentMember `json:"rejected"`
}

// SegmentError is a request the API refused as a whole. Code is one of the
// SegmentErr constants when the API names one.
type SegmentError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *SegmentError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("origamy: segment members: %d %s: %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("origamy: segment members: %d: %s", e.StatusCode, e.Message)
}

// SegmentMembersOption configures one AddMembers or RemoveMembers call.
type SegmentMembersOption func(*segmentMembersRequest)

// WithPhoneRegion reads phone numbers written without a country code as
// numbers of this ISO 3166 alpha-2 country, e.g. "NG".
func WithPhoneRegion(region string) SegmentMembersOption {
	return func(r *segmentMembersRequest) {
		r.PhoneRegion = region
	}
}

type segmentMembersRequest struct {
	Add         []SegmentMember `json:"add,omitempty"`
	Remove      []SegmentMember `json:"remove,omitempty"`
	PhoneRegion string          `json:"phoneRegion,omitempty"`
}

// segmentMaxAttempts bounds the tries of one request that is rate limited
// or finds the service unavailable.
const segmentMaxAttempts = 3

// SegmentsClient changes who is in a list (manual) segment. It needs a
// secret (sk_) key: public keys are refused. Unlike Client, its calls are
// synchronous and report the API's answer.
type SegmentsClient struct {
	endpoint   string
	key        string
	http       *http.Client
	retryAfter func(int) time.Duration
}

// NewSegments returns a client for list segment membership, authenticated
// with a secret key. Of the Options, it uses WithEndpoint, WithRetryAfter
// and Config.Transport; the others are ignored.
//
// Example:
//
//	segments := origamy.NewSegments(os.Getenv("ORIGAMY_SECRET_KEY"))
//	res, err := segments.AddMembers(ctx, 42, []origamy.SegmentMember{
//	    {Email: "jane@example.com", Traits: map[string]any{"first_name": "Jane"}},
//	    {Phone: "0803 123 4567"},
//	}, origamy.WithPhoneRegion("NG"))
func NewSegments(secretKey string, opts ...Option) *SegmentsClient {
	config := Config{}
	for _, opt := range opts {
		opt(&config)
	}
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	transport := config.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	retryAfter := config.RetryAfter
	if retryAfter == nil {
		retryAfter = func(attempt int) time.Duration { return time.Second << attempt }
	}
	return &SegmentsClient{
		endpoint:   strings.TrimRight(endpoint, "/"),
		key:        secretKey,
		http:       &http.Client{Transport: transport, Timeout: 30 * time.Second},
		retryAfter: retryAfter,
	}
}

// AddMembers adds people to a list segment. Each person goes through the
// event pipeline as an identify event: someone new becomes a profile, and
// segment-entered journeys start. Re-adding a member refreshes their traits.
//
// Lists longer than MaxSegmentMembersPerRequest are sent in several requests
// and their results combined. If a request fails, the result covers the
// requests that succeeded before it.
func (c *SegmentsClient) AddMembers(ctx context.Context, segmentID int64, members []SegmentMember, opts ...SegmentMembersOption) (*SegmentMembersResult, error) {
	return c.write(ctx, segmentID, members, false, opts)
}

// RemoveMembers removes people from a list segment and starts
// segment-exited journeys. A userId matches that person; an email or phone
// matches members whose stored email or phone is the same. Rows that match
// nobody in the segment are counted in NotFound.
//
// Lists longer than MaxSegmentMembersPerRequest are sent in several requests
// and their results combined. If a request fails, the result covers the
// requests that succeeded before it.
func (c *SegmentsClient) RemoveMembers(ctx context.Context, segmentID int64, members []SegmentMember, opts ...SegmentMembersOption) (*SegmentMembersResult, error) {
	return c.write(ctx, segmentID, members, true, opts)
}

func (c *SegmentsClient) write(ctx context.Context, segmentID int64, members []SegmentMember, remove bool, opts []SegmentMembersOption) (*SegmentMembersResult, error) {
	total := &SegmentMembersResult{SegmentID: segmentID, Rejected: []RejectedSegmentMember{}}
	if segmentID <= 0 {
		return total, &SegmentError{Code: SegmentErrInvalid, Message: "segment id must be a positive number"}
	}

	for start := 0; start < len(members); start += MaxSegmentMembersPerRequest {
		end := min(start+MaxSegmentMembersPerRequest, len(members))
		req := segmentMembersRequest{}
		for _, opt := range opts {
			opt(&req)
		}
		if remove {
			req.Remove = members[start:end]
		} else {
			req.Add = members[start:end]
		}

		res, err := c.post(ctx, segmentID, req)
		if err != nil {
			return total, err
		}
		total.Added += res.Added
		total.Removed += res.Removed
		total.NotFound += res.NotFound
		for _, r := range res.Rejected {
			r.Index += start
			total.Rejected = append(total.Rejected, r)
		}
	}
	return total, nil
}

func (c *SegmentsClient) post(ctx context.Context, segmentID int64, body segmentMembersRequest) (*SegmentMembersResult, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := c.endpoint + "/v1/segments/" + strconv.FormatInt(segmentID, 10) + "/members"

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "origamy-go (version: "+Version+")")
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth(c.key, "")

		res, err := c.http.Do(req)
		var wait time.Duration
		if err == nil {
			result, retry, rerr := readSegmentResponse(res)
			if !retry || attempt+1 >= segmentMaxAttempts {
				return result, rerr
			}
			err = rerr
			wait = retryAfterHeader(res.Header.Get("Retry-After"))
		} else if ctx.Err() != nil || attempt+1 >= segmentMaxAttempts {
			return nil, err
		}
		if wait == 0 {
			wait = c.retryAfter(attempt)
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, errors.Join(err, ctx.Err())
		case <-timer.C:
		}
	}
}

// readSegmentResponse decodes a response; retry reports whether the request
// may succeed if sent again.
func readSegmentResponse(res *http.Response) (result *SegmentMembersResult, retry bool, err error) {
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, true, err
	}

	if res.StatusCode == http.StatusOK {
		result = &SegmentMembersResult{}
		if err := json.Unmarshal(body, result); err != nil {
			return nil, false, fmt.Errorf("origamy: segment members: decoding response: %w", err)
		}
		return result, false, nil
	}

	serr := &SegmentError{StatusCode: res.StatusCode}
	var payload struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.Error != "" {
		serr.Code, serr.Message = payload.Code, payload.Error
	} else {
		serr.Message = strings.TrimSpace(string(body))
		if serr.Message == "" {
			serr.Message = http.StatusText(res.StatusCode)
		}
	}
	switch res.StatusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		retry = true
	}
	return nil, retry, serr
}

func retryAfterHeader(v string) time.Duration {
	secs, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}
