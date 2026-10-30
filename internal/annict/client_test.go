package annict

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func bounds(t *testing.T) (time.Time, time.Time) {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	return time.Date(2026, 10, 1, 0, 0, 0, 0, loc), time.Date(2026, 10, 3, 0, 0, 0, 0, loc)
}

func TestFetchPaginationAndLocalDateBoundaries(t *testing.T) {
	for _, username := range []string{"", "alice"} {
		t.Run("user="+username, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("Content-Type") != "application/json" {
					t.Error("unexpected request headers or method")
				}
				if r.URL.RawQuery != "" {
					t.Error("credentials must not appear in URL")
				}
				var request struct {
					Query     string
					Variables map[string]any
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if strings.Contains(request.Query, "test-token") {
					t.Error("token appeared in query")
				}
				if !strings.Contains(request.Query, "field: CREATED_AT, direction: DESC") {
					t.Error("records must be sorted")
				}
				if username == "" {
					if !strings.Contains(request.Query, "target: viewer") {
						t.Error("expected viewer query")
					}
				} else if !strings.Contains(request.Query, "user(username: $username)") || request.Variables["username"] != username {
					t.Error("expected variable-based user query")
				}
				switch calls {
				case 1:
					if request.Variables["after"] != nil {
						t.Error("first cursor should be null")
					}
					// Upper boundary is excluded; both following timestamps are October 2 in JST.
					fmt.Fprint(w, `{"data":{"target":{"username":"alice","records":{"nodes":[{"createdAt":"2026-10-02T15:00:00Z"},{"createdAt":"2026-10-02T14:59:59Z"},{"createdAt":"2026-10-01T15:00:00Z"}],"pageInfo":{"hasNextPage":true,"endCursor":"page-1"}}}}}`)
				case 2:
					if request.Variables["after"] != "page-1" {
						t.Error("cursor not advanced")
					}
					// Lower boundary is included. Older records stop pagination, even if more pages exist.
					fmt.Fprint(w, `{"data":{"target":{"username":"alice","records":{"nodes":[{"createdAt":"2026-09-30T15:00:00Z"},{"createdAt":"2026-09-30T14:59:59Z"}],"pageInfo":{"hasNextPage":true,"endCursor":"page-2"}}}}}`)
				default:
					t.Error("fetched history outside range")
					http.Error(w, "unexpected page", 500)
				}
			}))
			defer server.Close()
			from, until := bounds(t)
			client := &Client{Endpoint: server.URL, Token: "test-token"}
			got, err := client.Fetch(context.Background(), username, from, until)
			if err != nil {
				t.Fatal(err)
			}
			want := Result{Username: "alice", Days: map[string]int{"2026-10-01": 1, "2026-10-02": 2}, Total: 3}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v; want %#v", got, want)
			}
			if calls != 2 {
				t.Fatalf("got %d calls; want 2", calls)
			}
		})
	}
}

func TestFetchFailures(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body, want string
	}{
		{"unauthorized", 401, "secret response", "ANNICT_ACCESS_TOKEN"},
		{"forbidden", 403, "secret response", "authentication"},
		{"rate limit", 429, "", "rate limit"},
		{"server error", 500, "", "HTTP 500"},
		{"invalid JSON", 200, "not json", "JSON"},
		{"GraphQL partial data", 200, `{"data":{"target":{"username":"alice"}},"errors":[{"message":"invalid test-token"}]}`, "[REDACTED]"},
		{"missing data", 200, `{}`, "data"},
		{"missing user", 200, `{"data":{"target":null}}`, "user"},
		{"missing records", 200, `{"data":{"target":{"username":"alice","records":null}}}`, "episode records"},
		{"missing date", 200, `{"data":{"target":{"records":{"nodes":[{}]}}}}`, "creation timestamp"},
		{"missing cursor", 200, `{"data":{"target":{"records":{"nodes":[],"pageInfo":{"hasNextPage":true}}}}}`, "cursor"},
		{"repeated cursor", 200, `{"data":{"target":{"records":{"nodes":[],"pageInfo":{"hasNextPage":true,"endCursor":"same"}}}}}`, "cursor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls > 3 {
					t.Error("pagination loop")
					w.WriteHeader(500)
					return
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			from, until := bounds(t)
			got, err := (&Client{Endpoint: server.URL, Token: "test-token"}).Fetch(context.Background(), "", from, until)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v; want containing %q", err, tc.want)
			}
			if strings.Contains(err.Error(), "test-token") || strings.Contains(err.Error(), "secret response") {
				t.Fatal("error leaked sensitive contents")
			}
			if got.Total != 0 || got.Days != nil {
				t.Fatal("returned partial data after failure")
			}
		})
	}
}

func TestFetchEmptyRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"target":{"username":"alice","records":{"nodes":[],"pageInfo":{"hasNextPage":false}}}}}`)
	}))
	defer server.Close()
	from, until := bounds(t)
	got, err := (&Client{Endpoint: server.URL}).Fetch(context.Background(), "alice", from, until)
	if err != nil || got.Total != 0 || got.Username != "alice" {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestFetchCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	from, until := bounds(t)
	_, err := (&Client{Endpoint: "http://127.0.0.1:1"}).Fetch(ctx, "", from, until)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v; want canceled", err)
	}
}

func TestDoesNotFollowRedirects(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("redirect must not be followed") }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 307) }))
	defer server.Close()
	from, until := bounds(t)
	_, err := (&Client{Endpoint: server.URL, Token: "test-token"}).Fetch(context.Background(), "", from, until)
	if err == nil || !strings.Contains(err.Error(), "HTTP 307") {
		t.Fatalf("got %v", err)
	}
}
