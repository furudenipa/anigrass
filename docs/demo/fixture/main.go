// This recording-only entry point runs the real CLI against fictional Annict
// responses. It never sends requests or reads a real access token.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/furudenipa/anigrass/internal/cli"
)

type record struct {
	CreatedAt time.Time `json:"createdAt"`
}

type fixtureTransport struct {
	records []record
}

func (f fixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var query struct {
		Variables struct {
			After string `json:"after"`
		} `json:"variables"`
	}
	if err := json.NewDecoder(req.Body).Decode(&query); err != nil {
		return nil, err
	}
	start := 0
	if query.Variables.After != "" {
		var err error
		start, err = strconv.Atoi(query.Variables.After)
		if err != nil || start < 0 || start > len(f.records) {
			return nil, fmt.Errorf("invalid fixture cursor")
		}
	}
	end := min(start+100, len(f.records))
	body, err := json.Marshal(map[string]any{
		"data": map[string]any{"target": map[string]any{
			"username": "anime-demo",
			"records": map[string]any{
				"nodes": f.records[start:end],
				"pageInfo": map[string]any{
					"hasNextPage": end < len(f.records),
					"endCursor":   strconv.Itoa(end),
				},
			},
		}},
	})
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)), Request: req}, nil
}

func main() {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 20, 0, 0, 0, loc)
	rng := rand.New(rand.NewSource(42))
	var records []record
	for offset := 0; offset < 365; offset++ {
		day := today.AddDate(0, 0, -offset)
		// Regular seasonal viewing, days off, and occasional weekend binges.
		count := rng.Intn(5)
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			count += rng.Intn(5)
		}
		if offset%31 == 9 {
			count = 10 + rng.Intn(4)
		}
		if offset >= 160 && offset < 167 {
			count = 0 // One week away from watching during the year.
		}
		for n := 0; n < count; n++ {
			records = append(records, record{CreatedAt: day.Add(-time.Duration(n) * 25 * time.Minute)})
		}
	}
	http.DefaultTransport = fixtureTransport{records: records}
	getenv := func(key string) string {
		if key == "ANNICT_ACCESS_TOKEN" {
			return "fictional-demo-token"
		}
		return os.Getenv(key)
	}
	if err := cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, getenv); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
