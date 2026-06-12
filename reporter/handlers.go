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
	ListEntities(ctx context.Context, entityType string) (any, error)
	GetEntity(ctx context.Context, id string) (any, error)
	GetEntityRelationships(ctx context.Context, id string, depth int) (any, error)
	GetServiceTopology(ctx context.Context, id string, depth int) (any, error)
	GetServiceHealth(ctx context.Context, id string, window string, since string) (any, error)
	GetChangeEvents(ctx context.Context, targetID string, since string) (any, error)
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

func (h *Handler) ListEntities(c *gin.Context) {
	response, err := h.service.ListEntities(c.Request.Context(), c.Query("type"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": response})
}

func (h *Handler) GetEntity(c *gin.Context) {
	response, err := h.service.GetEntity(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": response})
}

func (h *Handler) GetEntityRelationships(c *gin.Context) {
	depth, err := parseDepthQuery(c.Query("depth"))
	if err != nil {
		writeError(c, err)
		return
	}
	response, err := h.service.GetEntityRelationships(c.Request.Context(), c.Param("id"), depth)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": response})
}

func (h *Handler) GetServiceTopology(c *gin.Context) {
	depth, err := parseDepthQuery(c.Query("depth"))
	if err != nil {
		writeError(c, err)
		return
	}
	response, err := h.service.GetServiceTopology(c.Request.Context(), c.Param("id"), depth)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": response})
}

func (h *Handler) GetServiceHealth(c *gin.Context) {
	response, err := h.service.GetServiceHealth(c.Request.Context(), c.Param("id"), c.DefaultQuery("window", "5m"), c.Query("since"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": response})
}

func (h *Handler) GetChangeEvents(c *gin.Context) {
	response, err := h.service.GetChangeEvents(c.Request.Context(), c.Query("target_id"), c.Query("since"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": response})
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

func parseDepthQuery(value string) (int, error) {
	if value == "" {
		return 1, nil
	}
	depth, err := parsePositiveInt(value)
	if err != nil {
		return 0, badRequest("invalid depth, expected a positive integer")
	}
	if depth > 3 {
		depth = 3
	}
	return depth, nil
}
