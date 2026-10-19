package origamy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// membersServer is a mock of POST /v1/segments/{id}/members. handle answers
// each request; it records every request it gets.
type membersServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []capturedMembersRequest
}

type capturedMembersRequest struct {
	Path     string
	User     string
	Password string
	HasAuth  bool
	Body     segmentMembersRequest
	Raw      map[string]any
}

func newMembersServer(t *testing.T, handle func(w http.ResponseWriter, n int, req capturedMembersRequest)) *membersServer {
	t.Helper()
	s := &membersServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		var c capturedMembersRequest
		c.Path = r.URL.Path
		c.User, c.Password, c.HasAuth = r.BasicAuth()
		if err := json.Unmarshal(raw, &c.Body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_ = json.Unmarshal(raw, &c.Raw)

		s.mu.Lock()
		s.requests = append(s.requests, c)
		n := len(s.requests)
		s.mu.Unlock()
		handle(w, n, c)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *membersServer) got() []capturedMembersRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]capturedMembersRequest(nil), s.requests...)
}

// echoResult answers like the API: every add is queued, every removal found,
// except rows whose email is "bad".
func echoResult(w http.ResponseWriter, _ int, req capturedMembersRequest) {
	res := SegmentMembersResult{SegmentID: 42, Rejected: []RejectedSegmentMember{}}
	for i, m := range req.Body.Add {
		if m.Email == "bad" {
			res.Rejected = append(res.Rejected, RejectedSegmentMember{List: "add", Index: i, Error: "invalid email"})
			continue
		}
		res.Added++
	}
	for i, m := range req.Body.Remove {
		if m.Email == "bad" {
			res.Rejected = append(res.Rejected, RejectedSegmentMember{List: "remove", Index: i, Error: "invalid email"})
			continue
		}
		res.Removed++
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func noWait(int) time.Duration { return 0 }

func TestSegmentsAddMembersSendsRequest(t *testing.T) {
	srv := newMembersServer(t, echoResult)
	segments := NewSegments("sk_test", WithEndpoint(srv.URL+"/"))

	res, err := segments.AddMembers(context.Background(), 42, []SegmentMember{
		{Email: "jane@example.com", Traits: map[string]any{"first_name": "Jane"}},
		{Phone: "0803 123 4567"},
		{UserID: "user-42"},
	}, WithPhoneRegion("NG"))
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}
	if res.SegmentID != 42 || res.Added != 3 || res.Removed != 0 || len(res.Rejected) != 0 {
		t.Errorf("result: got %+v", res)
	}

	reqs := srv.got()
	if len(reqs) != 1 {
		t.Fatalf("requests: got %d, want 1", len(reqs))
	}
	r := reqs[0]
	if r.Path != "/v1/segments/42/members" {
		t.Errorf("path: got %s", r.Path)
	}
	if !r.HasAuth || r.User != "sk_test" || r.Password != "" {
		t.Errorf("auth: got %q:%q (%v)", r.User, r.Password, r.HasAuth)
	}
	if r.Raw["phoneRegion"] != "NG" {
		t.Errorf("phoneRegion: got %v", r.Raw["phoneRegion"])
	}
	if _, ok := r.Raw["remove"]; ok {
		t.Errorf("remove should be omitted on an add: %v", r.Raw)
	}
	add := r.Raw["add"].([]any)
	if got := add[2].(map[string]any)["userId"]; got != "user-42" {
		t.Errorf("userId: got %v", got)
	}
	if got := add[0].(map[string]any)["traits"].(map[string]any)["first_name"]; got != "Jane" {
		t.Errorf("traits: got %v", got)
	}
}

func TestSegmentsRemoveMembersSendsRemoveList(t *testing.T) {
	srv := newMembersServer(t, func(w http.ResponseWriter, _ int, _ capturedMembersRequest) {
		fmt.Fprint(w, `{"segment_id":7,"added":0,"removed":1,"not_found":1,"rejected":[]}`)
	})
	segments := NewSegments("sk_test", WithEndpoint(srv.URL))

	res, err := segments.RemoveMembers(context.Background(), 7, []SegmentMember{{Email: "a@example.com"}, {UserID: "u2"}})
	if err != nil {
		t.Fatalf("RemoveMembers: %v", err)
	}
	if res.Removed != 1 || res.NotFound != 1 {
		t.Errorf("result: got %+v", res)
	}
	r := srv.got()[0]
	if len(r.Body.Remove) != 2 || len(r.Body.Add) != 0 {
		t.Errorf("body: got %+v", r.Body)
	}
	if _, ok := r.Raw["phoneRegion"]; ok {
		t.Errorf("phoneRegion should be omitted when unset: %v", r.Raw)
	}
}

func TestSegmentsSplitsLongListsAndCombinesResults(t *testing.T) {
	srv := newMembersServer(t, echoResult)
	segments := NewSegments("sk_test", WithEndpoint(srv.URL))

	members := make([]SegmentMember, 2500)
	for i := range members {
		members[i] = SegmentMember{Email: fmt.Sprintf("p%d@example.com", i)}
	}
	members[5].Email = "bad"
	members[1500].Email = "bad"
	members[2499].Email = "bad"

	res, err := segments.AddMembers(context.Background(), 42, members, WithPhoneRegion("GB"))
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	reqs := srv.got()
	if len(reqs) != 3 {
		t.Fatalf("requests: got %d, want 3", len(reqs))
	}
	for i, want := range []int{1000, 1000, 500} {
		if got := len(reqs[i].Body.Add); got != want {
			t.Errorf("request %d rows: got %d, want %d", i, got, want)
		}
		if reqs[i].Body.PhoneRegion != "GB" {
			t.Errorf("request %d phoneRegion: got %q", i, reqs[i].Body.PhoneRegion)
		}
	}
	if reqs[1].Body.Add[0].Email != "p1000@example.com" {
		t.Errorf("second request starts at %q", reqs[1].Body.Add[0].Email)
	}

	if res.Added != 2497 {
		t.Errorf("added: got %d, want 2497", res.Added)
	}
	var idx []int
	for _, r := range res.Rejected {
		if r.List != "add" {
			t.Errorf("rejected list: got %q", r.List)
		}
		idx = append(idx, r.Index)
	}
	if fmt.Sprint(idx) != "[5 1500 2499]" {
		t.Errorf("rejected indexes: got %v, want positions in the caller's slice", idx)
	}
}

func TestSegmentsEmptyListSendsNothing(t *testing.T) {
	srv := newMembersServer(t, echoResult)
	segments := NewSegments("sk_test", WithEndpoint(srv.URL))

	res, err := segments.AddMembers(context.Background(), 42, nil)
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}
	if res.Added != 0 || res.Rejected == nil {
		t.Errorf("result: got %+v", res)
	}
	if n := len(srv.got()); n != 0 {
		t.Errorf("requests: got %d, want 0", n)
	}
}

func TestSegmentsAPIErrors(t *testing.T) {
	cases := []struct {
		status int
		body   string
		code   string
	}{
		{404, `{"error":"segment not found","code":"segment_not_found"}`, SegmentErrNotFound},
		{409, `{"error":"segment is computed from conditions","code":"segment_not_list"}`, SegmentErrNotList},
		{409, `{"error":"segment mirrors a group","code":"segment_managed"}`, SegmentErrManaged},
		{400, `{"error":"more than 1000 rows","code":"invalid_request"}`, SegmentErrInvalid},
		{403, `{"error":"changing segment members needs a secret (sk_) key"}`, ""},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d %s", tc.status, tc.code), func(t *testing.T) {
			srv := newMembersServer(t, func(w http.ResponseWriter, _ int, _ capturedMembersRequest) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			})
			segments := NewSegments("sk_test", WithEndpoint(srv.URL), WithRetryAfter(noWait))

			_, err := segments.AddMembers(context.Background(), 42, []SegmentMember{{Email: "a@example.com"}})
			var serr *SegmentError
			if !errors.As(err, &serr) {
				t.Fatalf("error: got %v, want *SegmentError", err)
			}
			if serr.StatusCode != tc.status || serr.Code != tc.code || serr.Message == "" {
				t.Errorf("error: got %+v", serr)
			}
			if n := len(srv.got()); n != 1 {
				t.Errorf("requests: got %d, want 1 (no retry)", n)
			}
		})
	}
}

func TestSegmentsRetriesRateLimitAndUnavailable(t *testing.T) {
	srv := newMembersServer(t, func(w http.ResponseWriter, n int, req capturedMembersRequest) {
		switch n {
		case 1:
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":"rate limit exceeded"}`)
		case 2:
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"error":"segment service did not answer","code":"unavailable"}`)
		default:
			echoResult(w, n, req)
		}
	})
	segments := NewSegments("sk_test", WithEndpoint(srv.URL), WithRetryAfter(noWait))

	res, err := segments.AddMembers(context.Background(), 42, []SegmentMember{{Email: "a@example.com"}})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}
	if res.Added != 1 || len(srv.got()) != 3 {
		t.Errorf("result %+v after %d requests", res, len(srv.got()))
	}
}

func TestSegmentsGivesUpAfterRetries(t *testing.T) {
	srv := newMembersServer(t, func(w http.ResponseWriter, _ int, _ capturedMembersRequest) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error":"segment service did not answer","code":"unavailable"}`)
	})
	segments := NewSegments("sk_test", WithEndpoint(srv.URL), WithRetryAfter(noWait))

	_, err := segments.AddMembers(context.Background(), 42, []SegmentMember{{Email: "a@example.com"}})
	var serr *SegmentError
	if !errors.As(err, &serr) || serr.Code != SegmentErrUnavailable {
		t.Fatalf("error: got %v", err)
	}
	if n := len(srv.got()); n != segmentMaxAttempts {
		t.Errorf("requests: got %d, want %d", n, segmentMaxAttempts)
	}
}

func TestSegmentsFailedChunkKeepsEarlierResults(t *testing.T) {
	srv := newMembersServer(t, func(w http.ResponseWriter, n int, req capturedMembersRequest) {
		if n == 2 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"invalid phone region","code":"invalid_request"}`)
			return
		}
		echoResult(w, n, req)
	})
	segments := NewSegments("sk_test", WithEndpoint(srv.URL))

	members := make([]SegmentMember, 1500)
	for i := range members {
		members[i] = SegmentMember{UserID: fmt.Sprint(i)}
	}
	res, err := segments.AddMembers(context.Background(), 42, members)
	if err == nil {
		t.Fatal("expected an error from the second request")
	}
	if res == nil || res.Added != 1000 {
		t.Errorf("result: got %+v, want the first request's 1000 adds", res)
	}
}

func TestSegmentsRejectsNonPositiveID(t *testing.T) {
	segments := NewSegments("sk_test", WithEndpoint("http://127.0.0.1:1"))
	_, err := segments.AddMembers(context.Background(), 0, []SegmentMember{{Email: "a@example.com"}})
	var serr *SegmentError
	if !errors.As(err, &serr) || serr.Code != SegmentErrInvalid {
		t.Fatalf("error: got %v", err)
	}
}
