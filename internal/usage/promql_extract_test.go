package usage

import (
	"reflect"
	"testing"
	"time"
)

func TestExtractMetricNames(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "range selector",
			query: `rate(container_cpu_usage_seconds_total[5m])`,
			want:  []string{"container_cpu_usage_seconds_total"},
		},
		{
			name: "nested aggregate",
			query: `sum by (namespace, pod) (
  rate(http_requests_total{job="api"}[5m])
)`,
			want: []string{"http_requests_total"},
		},
		{
			name:  "recording rule selector",
			query: `job:http_requests:rate5m`,
			want:  []string{"job:http_requests:rate5m"},
		},
		{
			name:  "invalid promql",
			query: `sum by (job) (`,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractMetricNames(tt.query); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractMetricNames() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestExtractUsageCollectsMetricAndLabelEvidence(t *testing.T) {
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	expressions := []PromQLExpression{{
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceThanosAlert,
		SourceRef:  "rules/HighTraffic",
		SourceName: "HighTraffic",
		Expression: `sum by (job) (rate(http_requests_total{namespace="prod",pod!="x"}[5m])) / ignoring(instance) group_left(service) up{job="api"}`,
	}}

	valid, evidence, errors := ExtractUsage(expressions, now)
	if len(errors) != 0 {
		t.Fatalf("parse errors = %v, want none", errors)
	}
	if len(valid) != 1 {
		t.Fatalf("len(valid) = %d, want 1", len(valid))
	}
	if valid[0].ExpressionHash == "" {
		t.Fatal("ExpressionHash is empty")
	}
	if len(evidence) != 2 {
		t.Fatalf("len(evidence) = %d, want 2", len(evidence))
	}

	byMetric := map[string]MetricUsageEvidence{}
	for _, row := range evidence {
		byMetric[row.MetricName] = row
		if !row.IsAlert || row.IsRecordingRule || row.IsDashboard {
			t.Fatalf("flags = alert:%t recording:%t dashboard:%t", row.IsAlert, row.IsRecordingRule, row.IsDashboard)
		}
	}

	requests := byMetric["http_requests_total"]
	if !reflect.DeepEqual(requests.MatcherLabels, []string{"namespace", "pod"}) {
		t.Fatalf("MatcherLabels = %#v", requests.MatcherLabels)
	}
	if !reflect.DeepEqual(requests.GroupingLabels, []string{"instance", "job", "service"}) {
		t.Fatalf("GroupingLabels = %#v", requests.GroupingLabels)
	}
	if !reflect.DeepEqual(requests.UsedLabels, []string{"instance", "job", "namespace", "pod", "service"}) {
		t.Fatalf("UsedLabels = %#v", requests.UsedLabels)
	}

	up := byMetric["up"]
	if !reflect.DeepEqual(up.MatcherLabels, []string{"job"}) {
		t.Fatalf("up MatcherLabels = %#v", up.MatcherLabels)
	}
}

func TestExtractUsageSkipsInvalidPromQL(t *testing.T) {
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	expressions := []PromQLExpression{{
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceGrafanaDashboard,
		SourceRef:  "dash/1/A",
		Expression: `sum by (job) (`,
	}}

	valid, evidence, errors := ExtractUsage(expressions, now)
	if len(valid) != 0 {
		t.Fatalf("len(valid) = %d, want 0", len(valid))
	}
	if len(evidence) != 0 {
		t.Fatalf("len(evidence) = %d, want 0", len(evidence))
	}
	if len(errors) != 1 {
		t.Fatalf("len(errors) = %d, want 1", len(errors))
	}
}

func TestExtractUsageSupportsExplicitMetricNameMatcher(t *testing.T) {
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	expressions := []PromQLExpression{{
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceGrafanaDashboard,
		SourceRef:  "dash/1/A",
		Expression: `{__name__="custom_metric_total",cluster="west"}`,
	}}

	_, evidence, errors := ExtractUsage(expressions, now)
	if len(errors) != 0 {
		t.Fatalf("parse errors = %v, want none", errors)
	}
	if len(evidence) != 1 {
		t.Fatalf("len(evidence) = %d, want 1", len(evidence))
	}
	if evidence[0].MetricName != "custom_metric_total" {
		t.Fatalf("MetricName = %q, want custom_metric_total", evidence[0].MetricName)
	}
	if !reflect.DeepEqual(evidence[0].MatcherLabels, []string{"cluster"}) {
		t.Fatalf("MatcherLabels = %#v", evidence[0].MatcherLabels)
	}
}

func TestExtractUsageSupportsGrafanaTemplatedPromQL(t *testing.T) {
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	expressions := []PromQLExpression{{
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceGrafanaDashboard,
		SourceRef:  "dash/1/A",
		Expression: `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{environment=~"$environment", clusterName=~"$cluster"}[$__rate_interval])) by (le))`,
	}, {
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceGrafanaDashboard,
		SourceRef:  "dash/2/A",
		Expression: `rate(agones_k8s_client_http_request_total{$DevRole}[5m])`,
	}, {
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceGrafanaDashboard,
		SourceRef:  "dash/3/A",
		Expression: `region=~"$region", severity=~"$severity", alertname=~"$alertname"`,
	}, {
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceGrafanaDashboard,
		SourceRef:  "dash/4/A",
		Expression: `histogram_quantile($quantiles, sum by (pod, le) (rate(envoy_http_downstream_rq_time_bucket{namespace=~"$Namespace"}[$__rate_interval])))`,
	}, {
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceGrafanaDashboard,
		SourceRef:  "dash/5/A",
		Expression: `sum(round(increase(argocd_app_sync_total{namespace=~"$namespace"}[$interval]))) by ($grouping)`,
	}, {
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceGrafanaDashboard,
		SourceRef:  "dash/6/A",
		Expression: `slo:current_burn_rate:ratio > ${min_burning_rate}`,
	}, {
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceGrafanaDashboard,
		SourceRef:  "dash/7/A",
		Expression: `month() == bool vector(${__to:date:M})`,
	}, {
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceGrafanaDashboard,
		SourceRef:  "dash/8/A",
		Expression: `slo:sli_error:ratio_rate${sli_window}{sloth_service="${service}"}`,
	}, {
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceGrafanaDashboard,
		SourceRef:  "dash/9/A",
		Expression: `sum_over_time(increase(loki_discarded_samples_total{cluster="$cluster"}[1m])[$__range:1m])`,
	}, {
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceGrafanaDashboard,
		SourceRef:  "dash/10/A",
		Expression: `foo_total * on($join_label) group_left($include_label) bar_total`,
	}}

	valid, evidence, errors := ExtractUsage(expressions, now)
	if len(errors) != 0 {
		t.Fatalf("parse errors = %v, want none", errors)
	}
	if len(valid) != 10 {
		t.Fatalf("len(valid) = %d, want 10", len(valid))
	}
	if valid[0].Expression != expressions[0].Expression || valid[1].Expression != expressions[1].Expression {
		t.Fatalf("valid expressions = %#v, want original expression text preserved", valid)
	}

	byMetric := map[string]MetricUsageEvidence{}
	for _, row := range evidence {
		byMetric[row.MetricName] = row
	}
	if _, ok := byMetric["http_request_duration_seconds_bucket"]; !ok {
		t.Fatalf("evidence = %#v, want histogram bucket metric", evidence)
	}
	if _, ok := byMetric["agones_k8s_client_http_request_total"]; !ok {
		t.Fatalf("evidence = %#v, want metric with bare Grafana matcher variable", evidence)
	}
	if _, ok := byMetric["envoy_http_downstream_rq_time_bucket"]; !ok {
		t.Fatalf("evidence = %#v, want metric with Grafana scalar parameter", evidence)
	}
	if _, ok := byMetric["argocd_app_sync_total"]; !ok {
		t.Fatalf("evidence = %#v, want metric with Grafana grouping variable", evidence)
	}
	if _, ok := byMetric["slo:current_burn_rate:ratio"]; !ok {
		t.Fatalf("evidence = %#v, want metric with Grafana threshold variable", evidence)
	}
	if _, ok := byMetric["loki_discarded_samples_total"]; !ok {
		t.Fatalf("evidence = %#v, want metric with Grafana subquery range variable", evidence)
	}
	if _, ok := byMetric["foo_total"]; !ok {
		t.Fatalf("evidence = %#v, want lhs metric with Grafana vector matching variables", evidence)
	}
	if _, ok := byMetric["bar_total"]; !ok {
		t.Fatalf("evidence = %#v, want rhs metric with Grafana vector matching variables", evidence)
	}
	if len(evidence) != 8 {
		t.Fatalf("len(evidence) = %d, want selector fragments and templated metric names to create no metric-specific evidence", len(evidence))
	}
}

func TestExtractUsageSetsQueryHistoryEvidenceFields(t *testing.T) {
	now := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)
	queriedAt := time.Date(2026, 6, 8, 15, 27, 44, 0, time.UTC)
	expressions := []PromQLExpression{{
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceQueryHistory,
		SourceRef:  "history-1/A",
		Expression: "container_cpu_usage_seconds_total{namespace=\"prod\"}",
		QueriedAt:  &queriedAt,
	}}

	_, evidence, errors := ExtractUsage(expressions, now)
	if len(errors) != 0 {
		t.Fatalf("parse errors = %v, want none", errors)
	}
	if len(evidence) != 1 {
		t.Fatalf("len(evidence) = %d, want 1", len(evidence))
	}
	row := evidence[0]
	if !row.IsQueryHistory || row.QueryHistoryCount != 1 {
		t.Fatalf("query history fields = is:%t count:%d", row.IsQueryHistory, row.QueryHistoryCount)
	}
	if row.LastQueriedAt == nil || !row.LastQueriedAt.Equal(queriedAt) {
		t.Fatalf("LastQueriedAt = %v, want %s", row.LastQueriedAt, queriedAt)
	}
	if len(row.QueriedAt) != 1 || !row.QueriedAt[0].Equal(queriedAt) {
		t.Fatalf("QueriedAt = %#v, want queried timestamp", row.QueriedAt)
	}
}

func TestExtractUsageSelectorOnlyQueryCreatesNoEvidence(t *testing.T) {
	now := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)
	expressions := []PromQLExpression{{
		Tenant:     "prod",
		Day:        now,
		SourceType: UsageSourceQueryHistory,
		SourceRef:  "history-1/A",
		Expression: `count by (__name__) ({job="tokenrouter"})`,
	}}

	valid, evidence, errors := ExtractUsage(expressions, now)
	if len(errors) != 0 {
		t.Fatalf("parse errors = %v, want none", errors)
	}
	if len(valid) != 1 {
		t.Fatalf("len(valid) = %d, want expression stored", len(valid))
	}
	if len(evidence) != 0 {
		t.Fatalf("evidence = %#v, want no metric-specific rows", evidence)
	}
}
