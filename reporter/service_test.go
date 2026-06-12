package reporter

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/spaghettifunk/anima/entityresolver"
)

func TestParseFiltersDefaultsAndLimitClamp(t *testing.T) {
	service := NewService(&fakeRepository{}, Defaults{
		Tenant:       "default-tenant",
		LookbackDays: 30,
		Limit:        100,
		MaxLimit:     250,
	})

	values := url.Values{
		"limit":          {"999"},
		"offset":         {"10"},
		"sort":           {"last_seen_at"},
		"order":          {"desc"},
		"used_in_alerts": {"true"},
	}
	filters, err := service.ParseFilters(values)
	if err != nil {
		t.Fatalf("ParseFilters() error = %v", err)
	}

	if filters.Tenant != "default-tenant" {
		t.Fatalf("Tenant = %q, want default", filters.Tenant)
	}
	if filters.LookbackDays != 30 {
		t.Fatalf("LookbackDays = %d, want default", filters.LookbackDays)
	}
	if filters.Limit != 250 {
		t.Fatalf("Limit = %d, want max clamp", filters.Limit)
	}
	if filters.Offset != 10 {
		t.Fatalf("Offset = %d, want query value", filters.Offset)
	}
	if filters.Sort != "last_seen_at" || filters.Order != "desc" {
		t.Fatalf("sort/order = %q/%q, want last_seen_at/desc", filters.Sort, filters.Order)
	}
	if filters.UsedInAlerts == nil || !*filters.UsedInAlerts {
		t.Fatal("UsedInAlerts filter was not parsed")
	}
}

func TestParseFiltersRejectsInvalidSort(t *testing.T) {
	service := NewService(&fakeRepository{}, Defaults{Tenant: "default-tenant", LookbackDays: 30, Limit: 100, MaxLimit: 250})

	_, err := service.ParseFilters(url.Values{"sort": {";drop table metrics"}})
	var apiErr apiError
	if !errors.As(err, &apiErr) || apiErr.code != "bad_request" {
		t.Fatalf("ParseFilters() error = %v, want bad_request", err)
	}
}

func TestRecommendationPriorityAndProtectedMetrics(t *testing.T) {
	tests := []struct {
		name          string
		row           MetricReportRow
		wantRisk      MetricRisk
		wantRecommend MetricRecommendation
	}{
		{
			name:          "alert wins",
			row:           MetricReportRow{MetricName: "up", UsedInAlerts: true, UsedInRecordingRules: true, UsedInDashboards: true},
			wantRisk:      RiskCritical,
			wantRecommend: RecommendationKeep,
		},
		{
			name:          "recording rule",
			row:           MetricReportRow{MetricName: "http_requests_total", UsedInRecordingRules: true, UsedInDashboards: true},
			wantRisk:      RiskHigh,
			wantRecommend: RecommendationKeepOrReviewAggregation,
		},
		{
			name:          "query usage",
			row:           MetricReportRow{MetricName: "http_requests_total", QueryUsage: MetricQueryUsageSummary{WasQueriedInLast30d: true}},
			wantRisk:      RiskHigh,
			wantRecommend: RecommendationKeep,
		},
		{
			name:          "dashboard only",
			row:           MetricReportRow{MetricName: "http_requests_total", UsedInDashboards: true},
			wantRisk:      RiskMedium,
			wantRecommend: RecommendationReviewDashboardUsage,
		},
		{
			name:          "query history only",
			row:           MetricReportRow{MetricName: "http_requests_total", UsedInQueryHistory: true},
			wantRisk:      RiskMedium,
			wantRecommend: RecommendationReviewRecentQueryUsage,
		},
		{
			name:          "protected unused",
			row:           MetricReportRow{MetricName: "process_cpu_seconds_total"},
			wantRisk:      RiskUnknown,
			wantRecommend: RecommendationReview,
		},
		{
			name:          "unused custom metric",
			row:           MetricReportRow{MetricName: "custom_metric_total"},
			wantRisk:      RiskLow,
			wantRecommend: RecommendationDropCandidate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRisk, gotRecommendation := recommendMetric(tt.row)
			if gotRisk != tt.wantRisk || gotRecommendation != tt.wantRecommend {
				t.Fatalf("recommendMetric() = %s/%s, want %s/%s", gotRisk, gotRecommendation, tt.wantRisk, tt.wantRecommend)
			}
		})
	}
}

func TestParseFiltersSupportsQueryHistoryFilters(t *testing.T) {
	service := NewService(&fakeRepository{}, Defaults{Tenant: "default-tenant", LookbackDays: 30, Limit: 100, MaxLimit: 250})

	filters, err := service.ParseFilters(url.Values{
		"used_in_query_history": {"true"},
		"queried_in_last_days":  {"7"},
		"source_type":           {"query_history"},
		"sort":                  {"query_history_count"},
	})
	if err != nil {
		t.Fatalf("ParseFilters() error = %v", err)
	}
	if filters.UsedInQueryHistory == nil || !*filters.UsedInQueryHistory {
		t.Fatal("UsedInQueryHistory filter was not parsed")
	}
	if filters.QueriedInLastDays != 7 {
		t.Fatalf("QueriedInLastDays = %d, want 7", filters.QueriedInLastDays)
	}
	if filters.SourceType != "query_history" {
		t.Fatalf("SourceType = %q, want query_history", filters.SourceType)
	}
}

func TestUnusedLabelCalculationNormalizesDedupesAndSorts(t *testing.T) {
	got := unusedLabels(
		[]string{"pod", "namespace", "pod", "", "container"},
		[]string{"namespace", "namespace"},
	)
	want := []string{"container", "pod"}
	if len(got) != len(want) {
		t.Fatalf("unusedLabels() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unusedLabels() = %#v, want %#v", got, want)
		}
	}
}

func TestListMetricReportsAppliesRecommendationFiltersAndPagination(t *testing.T) {
	latest := mustDay(t, "2026-06-04")
	repo := &fakeRepository{
		latestDay: latest,
		reports: []MetricReportRow{
			{Day: "2026-06-04", Tenant: "default-tenant", Cluster: "a", MetricName: "custom_a_total", ObservedLabelKeys: []string{"pod"}},
			{Day: "2026-06-04", Tenant: "default-tenant", Cluster: "a", MetricName: "up", ObservedLabelKeys: []string{"job"}},
			{Day: "2026-06-04", Tenant: "default-tenant", Cluster: "a", MetricName: "custom_b_total", UsedInDashboards: true},
		},
	}
	service := NewService(repo, Defaults{Tenant: "default-tenant", LookbackDays: 30, Limit: 1, MaxLimit: 100})

	response, err := service.ListMetricReports(context.Background(), MetricReportFilters{
		Tenant:             "default-tenant",
		DropCandidatesOnly: true,
		Limit:              1,
		Offset:             0,
		Sort:               "metric_name",
		Order:              "asc",
	})
	if err != nil {
		t.Fatalf("ListMetricReports() error = %v", err)
	}
	if response.Pagination.Total != 1 || response.Pagination.HasMore {
		t.Fatalf("pagination = %#v, want one filtered result", response.Pagination)
	}
	if len(response.Data) != 1 || response.Data[0].MetricName != "custom_a_total" {
		t.Fatalf("data = %#v, want only custom_a_total", response.Data)
	}
	if len(response.Data[0].UnusedLabelKeys) != 1 || response.Data[0].UnusedLabelKeys[0] != "pod" {
		t.Fatalf("UnusedLabelKeys = %#v, want pod", response.Data[0].UnusedLabelKeys)
	}
}

func TestListMetricReportsTruncatesQueriedAtAndExcludesDropCandidates(t *testing.T) {
	latest := mustDay(t, "2026-06-09")
	newest := time.Date(2026, 6, 8, 15, 0, 0, 0, time.UTC)
	older := time.Date(2026, 6, 7, 15, 0, 0, 0, time.UTC)
	oldest := time.Date(2026, 6, 6, 15, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		latestDay: latest,
		reports: []MetricReportRow{
			{Day: "2026-06-09", Tenant: "default-tenant", Cluster: "a", MetricName: "custom_a_total"},
			{
				Day:                "2026-06-09",
				Tenant:             "default-tenant",
				Cluster:            "a",
				MetricName:         "custom_b_total",
				UsedInQueryHistory: true,
				QueryHistoryCount:  3,
				QueriedAt:          []time.Time{older, oldest, newest},
			},
		},
	}
	service := NewService(repo, Defaults{Tenant: "default-tenant", LookbackDays: 30, Limit: 100, MaxLimit: 100, MaxQueriedAtEntries: 2})

	response, err := service.ListMetricReports(context.Background(), MetricReportFilters{
		Tenant: "default-tenant",
		Limit:  100,
		Sort:   "metric_name",
		Order:  "asc",
	})
	if err != nil {
		t.Fatalf("ListMetricReports() error = %v", err)
	}
	queryRow := response.Data[1]
	if queryRow.Recommendation != RecommendationReviewRecentQueryUsage || queryRow.Risk != RiskMedium {
		t.Fatalf("recommendation = %s/%s, want query history review", queryRow.Risk, queryRow.Recommendation)
	}
	if len(queryRow.QueriedAt) != 2 || !queryRow.QueriedAt[0].Equal(newest) || !queryRow.QueriedAtTruncated {
		t.Fatalf("QueriedAt = %#v truncated=%t, want newest-first truncated", queryRow.QueriedAt, queryRow.QueriedAtTruncated)
	}

	dropOnly, err := service.ListMetricReports(context.Background(), MetricReportFilters{
		Tenant:             "default-tenant",
		Limit:              100,
		Sort:               "metric_name",
		Order:              "asc",
		DropCandidatesOnly: true,
	})
	if err != nil {
		t.Fatalf("ListMetricReports(drop candidates) error = %v", err)
	}
	if len(dropOnly.Data) != 1 || dropOnly.Data[0].MetricName != "custom_a_total" {
		t.Fatalf("drop candidates = %#v, want only unqueried metric", dropOnly.Data)
	}
}

func TestListMetricReportsKeepsQueryUsageMetrics(t *testing.T) {
	latest := mustDay(t, "2026-06-10")
	lastQueried := time.Date(2026, 6, 10, 8, 41, 22, 0, time.UTC)
	repo := &fakeRepository{
		latestDay: latest,
		reports: []MetricReportRow{
			{Day: "2026-06-10", Tenant: "default-tenant", Cluster: "a", MetricName: "custom_unused_total"},
			{
				Day:        "2026-06-10",
				Tenant:     "default-tenant",
				Cluster:    "a",
				MetricName: "custom_queried_total",
				QueryUsage: MetricQueryUsageSummary{
					LastQueriedAt:             &lastQueried,
					QueryCount24h:             2,
					QueryCount7d:              4,
					QueryCount30d:             6,
					DistinctQueryCount30d:     3,
					DistinctDashboardCount30d: 1,
					DirectQueryCount30d:       5,
					IndirectQueryCount30d:     1,
					WasQueriedInLast30d:       true,
				},
			},
		},
	}
	service := NewService(repo, Defaults{Tenant: "default-tenant", LookbackDays: 30, Limit: 100, MaxLimit: 100})

	response, err := service.ListMetricReports(context.Background(), MetricReportFilters{
		Tenant: "default-tenant",
		Limit:  100,
		Sort:   "metric_name",
		Order:  "asc",
	})
	if err != nil {
		t.Fatalf("ListMetricReports() error = %v", err)
	}
	queried := response.Data[0]
	if queried.MetricName != "custom_queried_total" || queried.Recommendation != RecommendationKeep || queried.Risk != RiskHigh {
		t.Fatalf("queried row = %#v, want high-risk keep", queried)
	}
	if queried.QueryUsage.QueryCount30d != 6 || queried.QueryUsage.IndirectQueryCount30d != 1 {
		t.Fatalf("QueryUsage = %#v, want populated summary", queried.QueryUsage)
	}

	dropOnly, err := service.ListMetricReports(context.Background(), MetricReportFilters{
		Tenant:             "default-tenant",
		Limit:              100,
		Sort:               "metric_name",
		Order:              "asc",
		DropCandidatesOnly: true,
	})
	if err != nil {
		t.Fatalf("ListMetricReports(drop candidates) error = %v", err)
	}
	if len(dropOnly.Data) != 1 || dropOnly.Data[0].MetricName != "custom_unused_total" {
		t.Fatalf("drop candidates = %#v, want only unqueried metric", dropOnly.Data)
	}

	unusedOnly, err := service.ListMetricReports(context.Background(), MetricReportFilters{
		Tenant:     "default-tenant",
		Limit:      100,
		Sort:       "metric_name",
		Order:      "asc",
		UnusedOnly: true,
	})
	if err != nil {
		t.Fatalf("ListMetricReports(unused only) error = %v", err)
	}
	if len(unusedOnly.Data) != 1 || unusedOnly.Data[0].MetricName != "custom_unused_total" {
		t.Fatalf("unused metrics = %#v, want only unqueried metric", unusedOnly.Data)
	}
}

func TestSummaryCountsQueryUsageAsUsed(t *testing.T) {
	latest := mustDay(t, "2026-06-10")
	repo := &fakeRepository{
		latestDay: latest,
		reports: []MetricReportRow{
			{Day: "2026-06-10", Tenant: "default-tenant", Cluster: "a", MetricName: "custom_unused_total"},
			{Day: "2026-06-10", Tenant: "default-tenant", Cluster: "a", MetricName: "custom_queried_total", QueryUsage: MetricQueryUsageSummary{WasQueriedInLast30d: true}},
		},
	}
	service := NewService(repo, Defaults{Tenant: "default-tenant", LookbackDays: 30, Limit: 100, MaxLimit: 100})

	response, err := service.GetSummary(context.Background(), MetricReportFilters{Tenant: "default-tenant", Limit: 100})
	if err != nil {
		t.Fatalf("GetSummary() error = %v", err)
	}
	if response.Data.UsedMetrics != 1 || response.Data.UnusedMetrics != 1 || response.Data.Keep != 1 || response.Data.DropCandidates != 1 {
		t.Fatalf("summary = %#v, want one used/keep and one unused/drop", response.Data)
	}
}

type fakeRepository struct {
	healthErr error
	latestDay time.Time
	latestErr error
	reports   []MetricReportRow
	evidence  []MetricEvidenceRow
}

func (f *fakeRepository) Health(ctx context.Context) error {
	return f.healthErr
}

func (f *fakeRepository) LatestDay(ctx context.Context, tenant string) (time.Time, error) {
	if f.latestErr != nil {
		return time.Time{}, f.latestErr
	}
	if f.latestDay.IsZero() {
		return time.Time{}, ErrNotFound
	}
	return f.latestDay, nil
}

func (f *fakeRepository) ListMetricReports(ctx context.Context, filters MetricReportFilters) ([]MetricReportRow, int, error) {
	rows := append([]MetricReportRow(nil), f.reports...)
	return rows, len(rows), nil
}

func (f *fakeRepository) GetMetricReport(ctx context.Context, tenant string, metricName string, day time.Time, lookbackDays int, queriedInLastDays int) (*MetricReportRow, error) {
	for _, row := range f.reports {
		if row.MetricName == metricName {
			rowCopy := row
			return &rowCopy, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepository) ListMetricEvidence(ctx context.Context, tenant string, metricName string, day time.Time, lookbackDays int, sourceType string) ([]MetricEvidenceRow, error) {
	rows := append([]MetricEvidenceRow(nil), f.evidence...)
	return rows, nil
}

func (f *fakeRepository) ListEntities(ctx context.Context, entityType string) ([]entityresolver.Entity, error) {
	return nil, nil
}

func (f *fakeRepository) GetEntity(ctx context.Context, id string) (*entityresolver.Entity, error) {
	return &entityresolver.Entity{ID: id}, nil
}

func (f *fakeRepository) GetEntityRelationships(ctx context.Context, id string, depth int) ([]entityresolver.Relationship, error) {
	return nil, nil
}

func (f *fakeRepository) GetServiceTopology(ctx context.Context, serviceID string, depth int) (*entityresolver.TopologyGraph, error) {
	return &entityresolver.TopologyGraph{}, nil
}

func (f *fakeRepository) GetServiceHealth(ctx context.Context, serviceID string, window string, since time.Time) ([]entityresolver.ServiceHealthWindow, error) {
	return nil, nil
}

func (f *fakeRepository) GetChangeEvents(ctx context.Context, targetID string, since time.Time) ([]entityresolver.ChangeEvent, error) {
	return nil, nil
}

func (f *fakeRepository) Close() {}

func mustDay(t *testing.T, value string) time.Time {
	t.Helper()
	day, err := parseDay(value)
	if err != nil {
		t.Fatalf("parseDay(%q) error = %v", value, err)
	}
	return day
}
