package enricher

import (
	"testing"
	"time"

	"github.com/spaghettifunk/anima/internal/sources/grafana"
	"github.com/spaghettifunk/anima/internal/sources/thanos"
	"github.com/spaghettifunk/anima/internal/usage"
)

func TestThanosExpressionsBuildSourceRefsAndTypes(t *testing.T) {
	day := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	got := thanosExpressions("prod", day, []thanos.RuleGroup{{
		Name: "api.rules",
		Rules: []thanos.Rule{
			{Type: "recording", Name: "job:http_requests:rate5m", Query: "sum(rate(http_requests_total[5m]))"},
			{Type: "alerting", Name: "HighErrors", Query: "rate(http_errors_total[5m]) > 0"},
		},
	}})

	if len(got) != 2 {
		t.Fatalf("len(expressions) = %d, want 2", len(got))
	}
	if got[0].SourceType != usage.UsageSourceThanosRule || got[0].SourceRef != "api.rules/job:http_requests:rate5m" {
		t.Fatalf("first expression = %#v", got[0])
	}
	if got[1].SourceType != usage.UsageSourceThanosAlert || got[1].SourceRef != "api.rules/HighErrors" {
		t.Fatalf("second expression = %#v", got[1])
	}
}

func TestGrafanaExpressionsWalkPanelsAndBuildSourceRefs(t *testing.T) {
	day := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	got := grafanaExpressions("prod", day, []grafana.DashboardDocument{{
		UID:   "dash-1",
		Title: "Service dashboard",
		Dashboard: grafana.Dashboard{
			Panels: []grafana.Panel{{
				ID:    float64(7),
				Title: "Requests",
				Targets: []grafana.Target{
					{Datasource: grafana.QueryDatasource{Type: "prometheus", UID: "thanos"}, Expr: "sum(rate(http_requests_total[5m]))", RefID: "A"},
					{Expr: ""},
				},
			}, {
				Title: "Row",
				Panels: []grafana.Panel{{
					Title:   "Nested",
					Targets: []grafana.Target{{Expr: "up"}},
				}},
			}},
		},
	}})

	if len(got) != 2 {
		t.Fatalf("len(expressions) = %d, want 2", len(got))
	}
	if got[0].SourceType != usage.UsageSourceGrafanaDashboard || got[0].SourceRef != "dash-1/7/A" {
		t.Fatalf("first expression = %#v", got[0])
	}
	if got[0].SourceName != "Service dashboard/Requests" {
		t.Fatalf("first SourceName = %q", got[0].SourceName)
	}
	if got[1].SourceRef != "dash-1/panel-1-0/target-0" {
		t.Fatalf("second SourceRef = %q", got[1].SourceRef)
	}
}

func TestGrafanaExpressionsSkipNonPrometheusAndLogQLTargets(t *testing.T) {
	day := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	got := grafanaExpressions("prod", day, []grafana.DashboardDocument{{
		UID:   "dash-1",
		Title: "Service dashboard",
		Dashboard: grafana.Dashboard{
			Panels: []grafana.Panel{{
				ID:         float64(7),
				Title:      "Mixed",
				Datasource: grafana.QueryDatasource{Type: "loki", UID: "logs"},
				Targets: []grafana.Target{
					{Datasource: grafana.QueryDatasource{Type: "prometheus", UID: "metrics"}, Expr: `sum(rate(http_requests_total{verb!~"WATCH|LIST"}[$__rate_interval]))`, RefID: "A"},
					{Expr: `{namespace="prod"} |= "error"`, RefID: "B"},
					{Datasource: grafana.QueryDatasource{Type: "loki", UID: "logs"}, Expr: `{namespace="prod"} | json`, RefID: "C"},
				},
			}},
		},
	}})

	if len(got) != 1 {
		t.Fatalf("len(expressions) = %d, want 1", len(got))
	}
	if got[0].SourceRef != "dash-1/7/A" {
		t.Fatalf("SourceRef = %q, want Prometheus target kept", got[0].SourceRef)
	}
}

func TestQueryHistoryExpressionsFilterAndBuildMetadata(t *testing.T) {
	day := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 6, 8, 15, 27, 44, 0, time.UTC)
	createdBy := int64(4)

	got := queryHistoryExpressions("prod", day, "thanos", []grafana.QueryHistoryItem{{
		UID:           "history-1",
		DatasourceUID: "thanos",
		CreatedBy:     &createdBy,
		CreatedAt:     createdAt,
		Comment:       "debug",
		Starred:       true,
		Queries: []grafana.QueryHistoryQuery{
			{Datasource: grafana.QueryDatasource{Type: "prometheus", UID: "thanos"}, Expr: "up", RefID: "A", Key: "key-a", EditorMode: "code"},
			{Datasource: grafana.QueryDatasource{Type: "prometheus", UID: "thanos"}, Expr: "http_requests_total", Key: "key-b"},
			{Datasource: grafana.QueryDatasource{Type: "prometheus", UID: "thanos"}, Expr: "process_cpu_seconds_total"},
			{Datasource: grafana.QueryDatasource{Type: "loki", UID: "thanos"}, Expr: "not_promql", RefID: "D"},
			{Datasource: grafana.QueryDatasource{Type: "prometheus", UID: "other"}, Expr: "other_metric", RefID: "E"},
			{Datasource: grafana.QueryDatasource{Type: "prometheus", UID: "thanos"}, Expr: ""},
		},
	}})

	if len(got) != 3 {
		t.Fatalf("len(expressions) = %d, want 3", len(got))
	}
	if got[0].SourceType != usage.UsageSourceQueryHistory || got[0].SourceRef != "history-1/A" {
		t.Fatalf("first expression = %#v", got[0])
	}
	if got[1].SourceRef != "history-1/key-b" {
		t.Fatalf("second SourceRef = %q, want key fallback", got[1].SourceRef)
	}
	if got[2].SourceRef != "history-1/query-2" {
		t.Fatalf("third SourceRef = %q, want index fallback", got[2].SourceRef)
	}
	if got[0].SourceName != "grafana-query-history/thanos" || got[0].QueriedAt == nil || !got[0].QueriedAt.Equal(createdAt) {
		t.Fatalf("metadata = %#v", got[0])
	}
}

func TestQueryHistoryRowsAndSelectorOnlyCount(t *testing.T) {
	now := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)
	day := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	expressions := []usage.PromQLExpression{
		{
			Tenant:          "prod",
			Day:             day,
			SourceType:      usage.UsageSourceQueryHistory,
			SourceRef:       "history-1/A",
			SourceName:      "grafana-query-history/thanos",
			Expression:      "up",
			QueryHistoryUID: "history-1",
			QueryRefID:      "A",
			DatasourceUID:   "thanos",
			QueriedAt:       &now,
		},
		{
			Tenant:          "prod",
			Day:             day,
			SourceType:      usage.UsageSourceQueryHistory,
			SourceRef:       "history-2/A",
			SourceName:      "grafana-query-history/thanos",
			Expression:      `count by (__name__) ({job="api"})`,
			QueryHistoryUID: "history-2",
			QueryRefID:      "A",
			DatasourceUID:   "thanos",
			QueriedAt:       &now,
		},
	}

	valid, evidence, parseErrors := usage.ExtractUsage(expressions, now)
	if len(parseErrors) != 0 {
		t.Fatalf("parse errors = %v, want none", parseErrors)
	}
	if len(evidence) != 1 || evidence[0].MetricName != "up" || !evidence[0].IsQueryHistory {
		t.Fatalf("evidence = %#v, want only concrete query-history metric", evidence)
	}
	if got := selectorOnlyQueryHistoryCount(valid, evidence); got != 1 {
		t.Fatalf("selectorOnlyQueryHistoryCount() = %d, want 1", got)
	}
	rows := queryHistoryRows(valid)
	if len(rows) != 2 {
		t.Fatalf("len(raw rows) = %d, want 2", len(rows))
	}
	if rows[0].ExpressionHash == "" || rows[0].CreatedAt.IsZero() {
		t.Fatalf("raw row = %#v, want hash and created_at", rows[0])
	}
}
