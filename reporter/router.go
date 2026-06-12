package reporter

import (
	ginzip "github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func NewRouter(service reportService) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(ginzip.Gzip(ginzip.DefaultCompression, ginzip.WithDecompressFn(ginzip.DefaultDecompressHandle)))

	handler := NewHandler(service)
	router.GET("/healthz", handler.Healthz)
	router.GET("/readyz", handler.Readyz)

	api := router.Group("/api/v1")
	reports := api.Group("/reports")
	reports.GET("/summary", handler.Summary)
	reports.GET("/metrics", handler.ListMetrics)
	reports.GET("/metrics/:metric_name", handler.GetMetric)
	reports.GET("/metrics/:metric_name/evidence", handler.ListMetricEvidence)
	api.GET("/entities", handler.ListEntities)
	api.GET("/entities/:id", handler.GetEntity)
	api.GET("/entities/:id/relationships", handler.GetEntityRelationships)
	api.GET("/services/:id/topology", handler.GetServiceTopology)
	api.GET("/services/:id/health", handler.GetServiceHealth)
	api.GET("/changes", handler.GetChangeEvents)

	return router
}
