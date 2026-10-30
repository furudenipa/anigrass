// Package annict provides a read-only client for Annict's GraphQL API.
package annict

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const Endpoint = "https://api.annict.com/graphql"

// Client never persists credentials or sends them in URLs.
type Client struct {
	HTTP     *http.Client
	Endpoint string
	Token    string
}

type Result struct {
	Username string
	Days     map[string]int
	Total    int
}

type connection struct {
	Nodes []*struct {
		CreatedAt time.Time `json:"createdAt"`
	} `json:"nodes"`
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
}

type user struct {
	Username string      `json:"username"`
	Records  *connection `json:"records"`
}

// Fetch counts episode records by creation date, in the range [from, until).
// Results are requested newest first, so history preceding from is not fetched.
func (c *Client) Fetch(ctx context.Context, username string, from, until time.Time) (Result, error) {
	result := Result{Days: make(map[string]int)}
	selection := "viewer"
	declaration := "$after: String"
	variables := map[string]any{"after": nil}
	if username != "" {
		selection = "user(username: $username)"
		declaration += ", $username: String!"
		variables["username"] = username
	}
	query := `query Grass(` + declaration + `) {
  target: ` + selection + ` {
    username
    records(first: 100, after: $after, orderBy: {field: CREATED_AT, direction: DESC}) {
      nodes { createdAt }
      pageInfo { hasNextPage endCursor }
    }
  }
}`
	seen := make(map[string]bool)
	for {
		var envelope struct {
			Data *struct {
				Target *user `json:"target"`
			} `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := c.request(ctx, query, variables, &envelope); err != nil {
			return Result{}, err
		}
		if len(envelope.Errors) > 0 {
			messages := make([]string, 0, len(envelope.Errors))
			for _, e := range envelope.Errors {
				messages = append(messages, e.Message)
			}
			message := strings.Join(messages, "; ")
			if c.Token != "" {
				message = strings.ReplaceAll(message, c.Token, "[REDACTED]")
			}
			return Result{}, fmt.Errorf("GraphQL: %s", message)
		}
		if envelope.Data == nil {
			return Result{}, errors.New("Annict API response is missing data")
		}
		u := envelope.Data.Target
		if u == nil {
			return Result{}, errors.New("user not found or not accessible")
		}
		if u.Records == nil {
			return Result{}, errors.New("could not retrieve episode records")
		}
		result.Username = u.Username
		older := false
		for _, record := range u.Records.Nodes {
			if record == nil {
				continue
			}
			if record.CreatedAt.IsZero() {
				return Result{}, errors.New("episode record is missing its creation timestamp")
			}
			if record.CreatedAt.Before(from) {
				older = true
				continue
			}
			if !record.CreatedAt.Before(until) {
				continue
			}
			result.Days[record.CreatedAt.In(from.Location()).Format(time.DateOnly)]++
			result.Total++
		}
		if older || !u.Records.PageInfo.HasNextPage {
			return result, nil
		}
		cursor := u.Records.PageInfo.EndCursor
		if cursor == "" || seen[cursor] {
			return Result{}, errors.New("Annict API pagination cursor did not advance")
		}
		seen[cursor] = true
		variables["after"] = cursor
	}
}

func (c *Client) request(ctx context.Context, query string, variables map[string]any, dest any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = Endpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("could not create Annict API request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("User-Agent", "anigrass")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("could not connect to Annict API; check your network or try again")
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New("authentication failed; check ANNICT_ACCESS_TOKEN expiration and read permission")
	case http.StatusTooManyRequests:
		return errors.New("Annict API rate limit reached; try again later")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Annict API: HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(dest); err != nil {
		return errors.New("could not decode Annict API JSON response")
	}
	return nil
}
