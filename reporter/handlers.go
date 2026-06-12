package reporter

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type reportService interface {
	Health(ctx context.Context) error
	ParseFilters(values url.Values) (MetricReportFilters, error)
	ListMetricReports(ctx context.Context, filters MetricReportFilters) (MetricReportResponse, error)
	GetMetricReport(ctx context.Context, filters MetricReportFilters, metricName string) (*MetricReportRow, MetricReportFilters, error)
	ListMetricEvidence(ctx context.Context, filters MetricReportFilters, metricName string) (MetricEvidenceResponse, error)
	GetSummary(ctx context.Context, filters MetricReportFilters) (MetricSummaryResponse, error)
}

type Handler struct {
	service reportService
}

func NewHandler(service reportService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) Readyz(c *gin.Context) {
	if err := h.service.Health(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (h *Handler) ListMetrics(c *gin.Context) {
	filters, err := h.service.ParseFilters(c.Request.URL.Query())
	if err != nil {
		writeError(c, err)
		return
	}

	response, err := h.service.ListMetricReports(c.Request.Context(), filters)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) GetMetric(c *gin.Context) {
	filters, err := h.service.ParseFilters(c.Request.URL.Query())
	if err != nil {
		writeError(c, err)
		return
	}

	response, resolvedFilters, err := h.service.GetMetricReport(c.Request.Context(), filters, c.Param("metric_name"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": response, "filters": resolvedFilters})
}

func (h *Handler) ListMetricEvidence(c *gin.Context) {
	filters, err := h.service.ParseFilters(c.Request.URL.Query())
	if err != nil {
		writeError(c, err)
		return
	}

	response, err := h.service.ListMetricEvidence(c.Request.Context(), filters, c.Param("metric_name"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) Summary(c *gin.Context) {
	filters, err := h.service.ParseFilters(c.Request.URL.Query())
	if err != nil {
		writeError(c, err)
		return
	}

	response, err := h.service.GetSummary(c.Request.Context(), filters)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

type apiError struct {
	code    string
	message string
}

func (e apiError) Error() string {
	return e.message
}

func badRequest(message string) error {
	return apiError{code: "bad_request", message: message}
}

func writeError(c *gin.Context, err error) {
	var apiErr apiError
	if errors.As(err, &apiErr) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: ErrorBody{Code: apiErr.code, Message: apiErr.message}})
		return
	}
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: ErrorBody{Code: "not_found", Message: "metric not found"}})
		return
	}

	log.Error().Err(err).Msg("reporter request failed")
	c.JSON(http.StatusInternalServerError, ErrorResponse{Error: ErrorBody{Code: "internal_error", Message: "internal server error"}})
}
