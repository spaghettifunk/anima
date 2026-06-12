package thanos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	rulesURL   string
	httpClient *http.Client
}

func NewClient(rulesURL string) *Client {
	return &Client{
		rulesURL: strings.TrimSpace(rulesURL),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type RuleGroup struct {
	Name  string `json:"name"`
	Rules []Rule `json:"rules"`
}

type Rule struct {
	Name   string `json:"name"`
	Alert  string `json:"alert"`
	Record string `json:"record"`
	Query  string `json:"query"`
	Type   string `json:"type"`
}

func (c *Client) FetchRules(ctx context.Context) ([]RuleGroup, error) {
	requestURL, err := buildRulesURL(c.rulesURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create thanos rules request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch thanos rules: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch thanos rules: unexpected status %s", resp.Status)
	}

	var payload rulesResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode thanos rules response: %w", err)
	}
	if payload.Status != "" && payload.Status != "success" {
		return nil, fmt.Errorf("fetch thanos rules: api status %q", payload.Status)
	}

	return payload.Data.Groups, nil
}

type rulesResponse struct {
	Status string `json:"status"`
	Data   struct {
		Groups []RuleGroup `json:"groups"`
	} `json:"data"`
}

func buildRulesURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("thanos rules url is required")
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse thanos rules url: %w", err)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("thanos rules url must include a host")
	}
	if strings.HasSuffix(strings.TrimRight(parsed.Path, "/"), "/ruler/api/v1/rules") {
		return parsed.String(), nil
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/ruler/api/v1/rules"
	parsed.RawQuery = ""
	return parsed.String(), nil
}
