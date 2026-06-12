package indexer

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/spaghettifunk/anima/internal/storage"
	"github.com/spaghettifunk/anima/queryaudit"
)

func TestDecodeAndNormalizeQueryAuditEvent(t *testing.T) {
	payload := []byte(`{
		"timestamp":"2026-06-10T08:41:22Z",
		"method":"GET",
		"path":"/api/v1/query_range",
		"raw_query":"query=rate%28http_requests_total%5B5m%5D%29&start=1781080000",
		"query":"rate(http_requests_total[5m])",
		"start":"1781080000",
		"end":"1781080060.5",
		"org_id":"prod",
		"dashboard_uid":"dash-a",
		"panel_id":"7",
		"status_code":200,
		"duration_ms":42
	}`)

	event, err := DecodeQueryAuditEvent(payload)
	if err != nil {
		t.Fatalf("DecodeQueryAuditEvent() error = %v", err)
	}
	record, warnings, err := NormalizeQueryEvent(event, time.Time{}, time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NormalizeQueryEvent() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	if record.Tenant != "prod" || record.UsageKind != UsageKindDashboardRangeQuery {
		t.Fatalf("record tenant/kind = %q/%q", record.Tenant, record.UsageKind)
	}
	if record.QueryHash != hashQuery("rate(http_requests_total[5m])") {
		t.Fatalf("QueryHash = %q, want computed query hash", record.QueryHash)
	}
	if record.QueryRaw == nil || *record.QueryRaw != "rate(http_requests_total[5m])" {
		t.Fatalf("QueryRaw = %#v, want PromQL only", record.QueryRaw)
	}
	if record.RangeStart == nil || record.RangeEnd == nil {
		t.Fatalf("range times = %v/%v, want parsed values", record.RangeStart, record.RangeEnd)
	}
}

func TestNormalizeQueryEventDoesNotPersistURLRawQuery(t *testing.T) {
	event := queryaudit.QueryAuditEvent{
		Timestamp: time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC),
		Path:      "/api/v1/query",
		RawQuery:  "query=up",
		OrgID:     "prod",
	}

	record, _, err := NormalizeQueryEvent(event, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("NormalizeQueryEvent() error = %v", err)
	}
	if record.QueryRaw != nil {
		t.Fatalf("QueryRaw = %#v, want nil for URL raw_query", record.QueryRaw)
	}
	if record.QueryHash != hashQuery("up") {
		t.Fatalf("QueryHash = %q, want hash from raw query param", record.QueryHash)
	}
}

func TestMetricNamesForQueryAuditEventPrefersParsedPromQLAndFallsBack(t *testing.T) {
	tests := []struct {
		name  string
		event queryaudit.QueryAuditEvent
		want  []string
	}{
		{
			name: "simple query",
			event: queryaudit.QueryAuditEvent{
				Query:           "up",
				ExtractedMetric: []string{"fallback_metric"},
			},
			want: []string{"up"},
		},
		{
			name: "aggregate rate query",
			event: queryaudit.QueryAuditEvent{
				Query: "sum by (job) (rate(http_requests_total[5m]))",
			},
			want: []string{"http_requests_total"},
		},
		{
			name: "raw query param",
			event: queryaudit.QueryAuditEvent{
				RawQuery: "query=rate%28container_cpu_usage_seconds_total%5B5m%5D%29",
			},
			want: []string{"container_cpu_usage_seconds_total"},
		},
		{
			name: "fallback extracted metrics",
			event: queryaudit.QueryAuditEvent{
				ExtractedMetric: []string{" http_requests_total ", "", "http_requests_total"},
			},
			want: []string{"http_requests_total"},
		},
		{
			name: "parse error fallback",
			event: queryaudit.QueryAuditEvent{
				Query:           "sum by (job) (",
				ExtractedMetric: []string{"proxy_metric_total"},
			},
			want: []string{"proxy_metric_total"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := metricNamesForQueryAuditEvent(tt.event)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("metricNamesForQueryAuditEvent() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestClassifyUsageKind(t *testing.T) {
	tests := []struct {
		name  string
		event queryaudit.QueryAuditEvent
		want  string
	}{
		{name: "direct query", event: queryaudit.QueryAuditEvent{Path: "/api/v1/query"}, want: UsageKindDirectQuery},
		{name: "range query", event: queryaudit.QueryAuditEvent{Path: "/api/v1/query_range"}, want: UsageKindRangeQuery},
		{name: "dashboard query", event: queryaudit.QueryAuditEvent{Path: "/api/v1/query", DashboardUID: "dash"}, want: UsageKindDashboardQuery},
		{name: "dashboard range query", event: queryaudit.QueryAuditEvent{Path: "/api/v1/query_range", PanelID: "1"}, want: UsageKindDashboardRangeQuery},
		{name: "series", event: queryaudit.QueryAuditEvent{Path: "/api/v1/series"}, want: UsageKindSeriesLookup},
		{name: "labels", event: queryaudit.QueryAuditEvent{Path: "/api/v1/labels"}, want: UsageKindLabelsLookup},
		{name: "label values", event: queryaudit.QueryAuditEvent{Path: "/api/v1/label/job/values"}, want: UsageKindLabelValuesLookup},
		{name: "metadata", event: queryaudit.QueryAuditEvent{Path: "/api/v1/metadata"}, want: UsageKindMetadataLookup},
		{name: "rules", event: queryaudit.QueryAuditEvent{Path: "/api/v1/rules"}, want: UsageKindRulesLookup},
		{name: "unknown", event: queryaudit.QueryAuditEvent{Path: "/api/v1/status/buildinfo"}, want: UsageKindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyUsageKind(tt.event); got != tt.want {
				t.Fatalf("ClassifyUsageKind() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsePrometheusTime(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Time
	}{
		{
			name:  "unix seconds",
			value: "1781080882",
			want:  time.Unix(1781080882, 0).UTC(),
		},
		{
			name:  "unix fractional seconds",
			value: "1781080882.25",
			want:  time.Unix(1781080882, 250000000).UTC(),
		},
		{
			name:  "rfc3339",
			value: "2026-06-10T08:41:22Z",
			want:  time.Date(2026, 6, 10, 8, 41, 22, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePrometheusTime(tt.value)
			if err != nil {
				t.Fatalf("ParsePrometheusTime() error = %v", err)
			}
			if got == nil || !got.Equal(tt.want) {
				t.Fatalf("ParsePrometheusTime() = %v, want %s", got, tt.want)
			}
		})
	}

	got, err := ParsePrometheusTime("")
	if err != nil || got != nil {
		t.Fatalf("ParsePrometheusTime(empty) = %v, %v; want nil, nil", got, err)
	}
	if _, err := ParsePrometheusTime("not-a-time"); err == nil {
		t.Fatal("ParsePrometheusTime(invalid) error = nil, want error")
	}
}

func TestBuildMetricQueryUsageRecordsSkipsMissingTenant(t *testing.T) {
	event := queryaudit.QueryAuditEvent{Query: "up"}
	record := storage.QueryEventRecord{ID: "event-id", UsageKind: UsageKindDirectQuery, ObservedAt: time.Now(), Source: "thanos"}

	rows := BuildMetricQueryUsageRecords(context.Background(), event, record, nil)
	if len(rows) != 0 {
		t.Fatalf("rows = %#v, want none without tenant", rows)
	}
}

func TestBuildMetricQueryUsageRecordsAddsRecordingRuleRows(t *testing.T) {
	event := queryaudit.QueryAuditEvent{Query: "job:http_requests:rate5m"}
	record := storage.QueryEventRecord{
		ID:         "00000000-0000-0000-0000-000000000001",
		ObservedAt: time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC),
		Tenant:     "prod",
		Source:     "thanos",
		UsageKind:  UsageKindDirectQuery,
		QueryHash:  "hash-a",
	}
	resolver := fakeRecordingRuleResolver{
		rows: []storage.ResolvedMetricUsage{{
			MetricName:        "http_requests_total",
			RecordingRuleName: "job:http_requests:rate5m",
		}},
	}

	rows := BuildMetricQueryUsageRecords(context.Background(), event, record, resolver)
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want direct and indirect rows: %#v", len(rows), rows)
	}
	if rows[0].MetricName != "job:http_requests:rate5m" || !rows[0].IsRecordingRule || rows[0].UsageKind != UsageKindDirectQuery {
		t.Fatalf("direct row = %#v", rows[0])
	}
	if rows[1].MetricName != "http_requests_total" || rows[1].UsageKind != UsageKindRecordingRuleQuery {
		t.Fatalf("indirect row = %#v", rows[1])
	}
	if rows[1].ResolvedFromRecordingRule == nil || *rows[1].ResolvedFromRecordingRule != "job:http_requests:rate5m" {
		t.Fatalf("ResolvedFromRecordingRule = %#v", rows[1].ResolvedFromRecordingRule)
	}
}

type fakeRecordingRuleResolver struct {
	rows []storage.ResolvedMetricUsage
}

func (f fakeRecordingRuleResolver) ResolveRecordingRule(context.Context, string, string) ([]storage.ResolvedMetricUsage, error) {
	return f.rows, nil
}
