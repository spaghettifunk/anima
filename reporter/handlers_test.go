package reporter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandlerListMetricsReturnsJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubReportService{
		filters: MetricReportFilters{Tenant: "prod", Day: "2026-06-04", LookbackDays: 30, Limit: 100, Sort: "metric_name", Order: "asc"},
		reportResponse: MetricReportResponse{
			Data: []MetricReportRow{
				{
					Day:            "2026-06-04",
					Tenant:         "prod",
					Cluster:        "a",
					MetricName:     "custom_metric_total",
					Risk:           RiskLow,
					Recommendation: RecommendationDropCandidate,
				},
			},
			Pagination: Pagination{Limit: 100, Total: 1},
			Filters:    MetricReportFilters{Tenant: "prod", Day: "2026-06-04", LookbackDays: 30, Limit: 100},
		},
	}
	router := NewRouter(service)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/reports/metrics?tenant=prod", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}

	var response MetricReportResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].MetricName != "custom_metric_total" {
		t.Fatalf("response data = %#v, want custom_metric_total", response.Data)
	}
}

func TestHandlerInvalidQueryReturnsErrorShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(&stubReportService{parseErr: badRequest("invalid sort field")})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/reports/metrics?sort=nope", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", recorder.Code, recorder.Body.String())
	}

	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("response JSON error = %v", err)
	}
	if response.Error.Code != "bad_request" || response.Error.Message != "invalid sort field" {
		t.Fatalf("error = %#v, want bad_request invalid sort field", response.Error)
	}
}

func TestHandlerReadyzChecksRepository(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(&stubReportService{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
}

type stubReportService struct {
	healthErr        error
	parseErr         error
	filters          MetricReportFilters
	reportResponse   MetricReportResponse
	metric           *MetricReportRow
	evidenceResponse MetricEvidenceResponse
	summaryResponse  MetricSummaryResponse
}

func (s *stubReportService) Health(ctx context.Context) error {
	return s.healthErr
}

func (s *stubReportService) ParseFilters(values url.Values) (MetricReportFilters, error) {
	if s.parseErr != nil {
		return MetricReportFilters{}, s.parseErr
	}
	return s.filters, nil
}

func (s *stubReportService) ListMetricReports(ctx context.Context, filters MetricReportFilters) (MetricReportResponse, error) {
	return s.reportResponse, nil
}

func (s *stubReportService) GetMetricReport(ctx context.Context, filters MetricReportFilters, metricName string) (*MetricReportRow, MetricReportFilters, error) {
	if s.metric != nil {
		return s.metric, s.filters, nil
	}
	row := MetricReportRow{Day: "2026-06-04", Tenant: "prod", MetricName: metricName}
	return &row, s.filters, nil
}

func (s *stubReportService) ListMetricEvidence(ctx context.Context, filters MetricReportFilters, metricName string) (MetricEvidenceResponse, error) {
	return s.evidenceResponse, nil
}

func (s *stubReportService) GetSummary(ctx context.Context, filters MetricReportFilters) (MetricSummaryResponse, error) {
	return s.summaryResponse, nil
}
