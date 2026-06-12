package grafana

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestFetchDashboardsSearchesAndFetchesDashboardDocuments(t *testing.T) {
	client := NewClient("https://grafana.example", "secret")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}

		switch r.URL.Path {
		case "/api/search":
			if got := r.URL.Query().Get("type"); got != "dash-db" {
				t.Fatalf("search type = %q, want dash-db", got)
			}
			return jsonResponse(`[{"uid":"dash-1","title":"Service dashboard","type":"dash-db"}]`), nil
		case "/api/dashboards/uid/dash-1":
			return jsonResponse(`{
				"dashboard": {
					"title": "Service dashboard",
					"panels": [{
						"id": 7,
						"title": "Requests",
							"targets": [
							{"datasource": {"type": "prometheus", "uid": "thanos"}, "expr": "sum(rate(http_requests_total{job=\"api\"}[5m]))", "refId": "A"},
							{"expr": ""}
						]
					}, {
						"title": "Row",
						"datasource": "legacy-prometheus",
						"panels": [{
							"title": "Nested",
							"targets": [{"expr": "up", "refId": ""}]
						}]
					}]
				}
			}`), nil
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
			return nil, nil
		}
	})}

	got, err := client.FetchDashboards(context.Background())
	if err != nil {
		t.Fatalf("FetchDashboards() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(dashboards) = %d, want 1", len(got))
	}
	if got[0].UID != "dash-1" || got[0].Title != "Service dashboard" {
		t.Fatalf("dashboard = %#v", got[0])
	}
	if len(got[0].Dashboard.Panels) != 2 {
		t.Fatalf("len(panels) = %d, want 2", len(got[0].Dashboard.Panels))
	}
	if got[0].Dashboard.Panels[0].Targets[0].Datasource.Type != "prometheus" || got[0].Dashboard.Panels[0].Targets[0].Datasource.UID != "thanos" {
		t.Fatalf("target datasource = %#v", got[0].Dashboard.Panels[0].Targets[0].Datasource)
	}
	if got[0].Dashboard.Panels[1].Datasource.UID != "legacy-prometheus" {
		t.Fatalf("legacy panel datasource = %#v", got[0].Dashboard.Panels[1].Datasource)
	}
}

func TestFetchQueryHistoryEncodesRequestAndConvertsTimestamps(t *testing.T) {
	client := NewClient("https://grafana.example", "secret")
	from := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/query-history" {
			t.Fatalf("path = %q, want /api/query-history", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("datasourceUid") != "thanos" || query.Get("sort") != "time-desc" || query.Get("limit") != "100" || query.Get("page") != "1" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		if query.Get("from") != "1780876800000" || query.Get("to") != "1780963200000" {
			t.Fatalf("time query = from %s to %s", query.Get("from"), query.Get("to"))
		}
		return jsonResponse(`{"result":{"totalCount":1,"queryHistory":[{"uid":"h1","datasourceUid":"thanos","createdBy":4,"createdAt":1780950000,"queries":[{"datasource":{"type":"prometheus","uid":"thanos"},"expr":"up","refId":"A"}]}]}}`), nil
	})}

	got, err := client.FetchQueryHistory(context.Background(), QueryHistoryRequest{
		DatasourceUID: "thanos",
		From:          from,
		To:            to,
		Limit:         100,
		Page:          1,
		Sort:          "time-desc",
	})
	if err != nil {
		t.Fatalf("FetchQueryHistory() error = %v", err)
	}
	if got.TotalCount != 1 || len(got.Items) != 1 {
		t.Fatalf("response = %#v, want one item", got)
	}
	if got.Items[0].CreatedAt.Unix() != 1780950000 {
		t.Fatalf("CreatedAt = %s, want unix seconds conversion", got.Items[0].CreatedAt)
	}
}

func TestFetchAllQueryHistoryPaginatesAndStopsAtTotalCount(t *testing.T) {
	client := NewClient("https://grafana.example", "secret")
	pages := []string{}
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		pages = append(pages, r.URL.Query().Get("page"))
		switch r.URL.Query().Get("page") {
		case "1":
			return jsonResponse(`{"result":{"totalCount":3,"queryHistory":[{"uid":"h1","createdAt":1780950000000,"queries":[{"expr":"up","refId":"A"}]},{"uid":"h2","createdAt":1780950000000,"queries":[{"expr":"up","refId":"A"}]}]}}`), nil
		case "2":
			return jsonResponse(`{"result":{"totalCount":3,"queryHistory":[{"uid":"h3","createdAt":1780950000000,"queries":[{"expr":"up","refId":"A"}]}]}}`), nil
		default:
			t.Fatalf("unexpected page %s", r.URL.Query().Get("page"))
			return nil, nil
		}
	})}

	got, err := client.FetchAllQueryHistory(context.Background(), QueryHistoryRequest{DatasourceUID: "thanos", Limit: 2}, 10)
	if err != nil {
		t.Fatalf("FetchAllQueryHistory() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(got))
	}
	if strings.Join(pages, ",") != "1,2" {
		t.Fatalf("pages = %v, want 1,2", pages)
	}
	if got[0].CreatedAt.UnixMilli() != 1780950000000 {
		t.Fatalf("CreatedAt = %s, want unix milliseconds conversion", got[0].CreatedAt)
	}
}

func TestFetchAllQueryHistoryStopsOnEmptyPageAndMaxPages(t *testing.T) {
	t.Run("empty page", func(t *testing.T) {
		client := NewClient("https://grafana.example", "secret")
		pages := 0
		client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			pages++
			return jsonResponse(`{"result":{"totalCount":10,"queryHistory":[]}}`), nil
		})}
		got, err := client.FetchAllQueryHistory(context.Background(), QueryHistoryRequest{DatasourceUID: "thanos", Limit: 2}, 10)
		if err != nil {
			t.Fatalf("FetchAllQueryHistory() error = %v", err)
		}
		if len(got) != 0 || pages != 1 {
			t.Fatalf("items/pages = %d/%d, want 0/1", len(got), pages)
		}
	})

	t.Run("max pages", func(t *testing.T) {
		client := NewClient("https://grafana.example", "secret")
		pages := 0
		client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			pages++
			return jsonResponse(`{"result":{"totalCount":100,"queryHistory":[{"uid":"h","createdAt":1780950000,"queries":[{"expr":"up"}]}]}}`), nil
		})}
		got, err := client.FetchAllQueryHistory(context.Background(), QueryHistoryRequest{DatasourceUID: "thanos", Limit: 1}, 2)
		if err != nil {
			t.Fatalf("FetchAllQueryHistory() error = %v", err)
		}
		if len(got) != 2 || pages != 2 {
			t.Fatalf("items/pages = %d/%d, want 2/2", len(got), pages)
		}
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
