package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/furudenipa/anigrass/internal/annict"
)

func TestValidationAndHelp(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"help", []string{"--help"}, ""},
		{"missing token", nil, "ANNICT_ACCESS_TOKEN"},
		{"bad theme", []string{"--theme", "invalid"}, "--theme"},
		{"bad date", []string{"--from", "2026-02-30"}, "--from"},
		{"bad end", []string{"--to", "today"}, "--to"},
		{"inverted range", []string{"--from", "2026-10-10", "--to", "2026-10-01"}, "on or before"},
		{"oversized range", []string{"--from", "2020-01-01", "--to", "2026-01-01"}, "one year"},
		{"bad timezone", []string{"--timezone", "bad/zone"}, "timezone"},
		{"empty glyph", []string{"--grass", " "}, "visible characters"},
		{"control glyph", []string{"--grass", "\x1b[0m"}, "visible characters"},
		{"positional", []string{"alice"}, "positional arguments"},
		{"unknown flag", []string{"--unknown"}, "flag provided"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			err := run(context.Background(), tc.args, &out, &stderr, func(string) string { return "" }, time.Now(), &annict.Client{})
			if tc.want == "" {
				if err != nil || !strings.Contains(stderr.String(), "Usage:") {
					t.Fatalf("help: %v, %s", err, stderr.String())
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v; want %q", err, tc.want)
			}
			if out.Len() != 0 {
				t.Fatal("unexpected stdout")
			}
		})
	}
}

func TestHelpFlagOrder(t *testing.T) {
	var out, stderr bytes.Buffer
	if err := Run(context.Background(), []string{"--help"}, &out, &stderr, func(string) string { return "" }); err != nil {
		t.Fatal(err)
	}
	var flags []string
	for _, line := range strings.Split(stderr.String(), "\n") {
		if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "  anigrass") {
			continue
		}
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, "--") {
				flags = append(flags, field)
				break
			}
		}
	}
	want := []string{"--user", "--from", "--to", "--timezone", "--theme", "--grass", "--no-color", "--show-user", "--show-range", "--show-months", "--show-weekdays", "--show-legend", "--total", "--help"}
	if !reflect.DeepEqual(flags, want) {
		t.Fatalf("help flag order = %v; want %v", flags, want)
	}
}

func TestEndToEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fake-token" {
			t.Error("token not read from env")
		}
		fmt.Fprint(w, `{"data":{"target":{"username":"alice","records":{"nodes":[{"createdAt":"2026-10-30T14:59:59Z"},{"createdAt":"2025-10-31T00:00:00Z"},{"createdAt":"2025-10-30T00:00:00Z"}],"pageInfo":{"hasNextPage":false}}}}}`)
	}))
	defer server.Close()
	for _, tc := range []struct {
		name  string
		args  []string
		env   map[string]string
		color bool
	}{
		{"default", nil, nil, true},
		{"details", []string{"--total", "--show-range", "--show-user", "--show-weekdays", "--show-months", "--show-legend"}, nil, true},
		{"no-color flag", []string{"--no-color"}, nil, false},
		{"NO_COLOR", nil, map[string]string{"NO_COLOR": "1"}, false},
		{"dumb terminal", nil, map[string]string{"TERM": "dumb"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			getenv := func(key string) string {
				if key == "ANNICT_ACCESS_TOKEN" {
					return " fake-token "
				}
				return tc.env[key]
			}
			now := time.Date(2026, 10, 30, 15, 0, 0, 0, time.FixedZone("JST", 9*3600))
			err := run(context.Background(), tc.args, &out, &stderr, getenv, now, &annict.Client{Endpoint: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "2025-10-31 → 2026-10-30") != (tc.name == "details") {
				t.Fatal("incorrect default range")
			}
			if strings.Contains(out.String(), "\x1b[") != tc.color {
				t.Fatal("incorrect color mode")
			}
			if strings.Contains(out.String(), "Total: 2 episode records") != (tc.name == "details") {
				t.Fatal("incorrect total")
			}
			if tc.name == "details" {
				for _, label := range []string{"alice", "Sun", "Less ", "More "} {
					if !strings.Contains(out.String(), label) {
						t.Fatalf("missing optional label %q", label)
					}
				}
			} else if strings.Count(out.String(), "\n") != 7 {
				t.Fatalf("default output must contain only seven grass rows: %q", out.String())
			}
		})
	}
}
