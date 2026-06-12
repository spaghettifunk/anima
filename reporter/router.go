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

	v1 := router.Group("/api/v1/reports")
	v1.GET("/summary", handler.Summary)
	v1.GET("/metrics", handler.ListMetrics)
	v1.GET("/metrics/:metric_name", handler.GetMetric)
	v1.GET("/metrics/:metric_name/evidence", handler.ListMetricEvidence)

	return router
}
