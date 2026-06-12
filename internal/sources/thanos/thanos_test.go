package thanos

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFetchRulesReturnsRuleGroups(t *testing.T) {
	client := NewClient("http://thanos.example")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/ruler/api/v1/rules" {
			t.Fatalf("path = %q, want /ruler/api/v1/rules", r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
			"status": "success",
			"data": {
				"groups": [{
					"name": "api.rules",
					"rules": [
						{"type": "recording", "name": "job:http_requests:rate5m", "query": "sum by (job) (rate(http_requests_total[5m]))"},
						{"type": "alerting", "name": "HighErrors", "query": "rate(http_errors_total[5m]) > 0"}
					]
				}]
			}
		}`)),
		}, nil
	})}

	got, err := client.FetchRules(context.Background())
	if err != nil {
		t.Fatalf("FetchRules() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(got))
	}
	if got[0].Name != "api.rules" {
		t.Fatalf("first group name = %q", got[0].Name)
	}
	if len(got[0].Rules) != 2 {
		t.Fatalf("len(first group rules) = %d, want 2", len(got[0].Rules))
	}
	if got[0].Rules[0].Type != "recording" || got[0].Rules[0].Name != "job:http_requests:rate5m" {
		t.Fatalf("first rule = %#v", got[0].Rules[0])
	}
	if got[0].Rules[1].Type != "alerting" || got[0].Rules[1].Name != "HighErrors" {
		t.Fatalf("second rule = %#v", got[0].Rules[1])
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
