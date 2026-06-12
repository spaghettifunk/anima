package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/spaghettifunk/anima/internal/producer"
	"github.com/spaghettifunk/anima/internal/utils"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	jsonContentType       = "application/json"
	protobufContentType   = "application/x-protobuf"
	contentTypeHeaderName = "Content-Type"
)

func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, "healthy")
}

func (col *Collector) Readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := col.producer.Health(ctx); err != nil {
		log.Error().Err(err).Msg("collector is not ready")
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (col *Collector) Logs(c *gin.Context) {
	contentType := c.Request.Header[contentTypeHeaderName]
	if len(contentType) <= 0 {
		log.Error().Msg("unknown content-type. unable to unmarshal request")
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("missing content-type header of either type `json` or `proto`"))
		return
	}

	defer c.Request.Body.Close()
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	// is the incoming request a JSON
	if utils.Contains(contentType, jsonContentType) {
		decoder := &plog.JSONUnmarshaler{}
		if _, err = decoder.UnmarshalLogs(body); err != nil {
			log.Error().Err(err).Msg("failed to unmarshall JSON request body using plog")
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	}

	// is the incoming request a Proto
	if utils.Contains(contentType, protobufContentType) {
		decoder := &plog.ProtoUnmarshaler{}
		if _, err = decoder.UnmarshalLogs(body); err != nil {
			log.Error().Err(err).Msg("failed to unmarshall JSON request body using plog")
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	}

	// send the telemetry to the producer
	if err := col.producer.Write(producer.PacketTelemetry{
		Name:    col.config.Collector.LogsTopic,
		Message: body,
	}); err != nil {
		fmt.Println("error in sending data to kafka")
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, "ok")
}

func (col *Collector) Metrics(c *gin.Context) {
	contentType := c.Request.Header[contentTypeHeaderName]
	if len(contentType) <= 0 {
		log.Error().Msg("unknown content-type. unable to unmarshal request")
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("missing content-type header of either type `json` or `proto`"))
		return
	}

	defer c.Request.Body.Close()
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error().Err(err).Msg("failed to read request body")
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var md pmetric.Metrics

	// is the incoming request a JSON?
	if utils.Contains(contentType, jsonContentType) {
		decoder := &pmetric.JSONUnmarshaler{}
		if md, err = decoder.UnmarshalMetrics(body); err != nil {
			log.Error().Err(err).Msg("failed to unmarshall JSON request body using pmetric")
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	}

	// is the incoming request a Proto?
	if utils.Contains(contentType, protobufContentType) {
		decoder := &pmetric.ProtoUnmarshaler{}
		if md, err = decoder.UnmarshalMetrics(body); err != nil {
			log.Error().Err(err).Msg("failed to unmarshall JSON request body using pmetric")
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	}

	// we normalize the metric series
	observations := NormalizeMetricObservations(md, "otlp", time.Now())
	payload, err := json.Marshal(observations)
	if err != nil {
		log.Error().Err(err).Msg("failed to Marshal observation")
	}

	// send the telemetry to the producer
	if err := col.producer.Write(producer.PacketTelemetry{
		Name:    col.config.Collector.MetricsTopic,
		Message: payload,
	}); err != nil {
		log.Error().Err(err).Msg("failed to write to kafka")
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, "ok")
}

func (col *Collector) Traces(c *gin.Context) {
	contentType := c.Request.Header[contentTypeHeaderName]
	if len(contentType) <= 0 {
		log.Error().Msg("unknown content-type. unable to unmarshal request")
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("missing content-type header of either type `json` or `proto`"))
		return
	}

	defer c.Request.Body.Close()
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	// is the incoming request a JSON
	if utils.Contains(contentType, jsonContentType) {
		decoder := &ptrace.JSONUnmarshaler{}
		if _, err = decoder.UnmarshalTraces(body); err != nil {
			log.Error().Err(err).Msg("failed to unmarshall JSON request body using ptrace")
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	}

	// is the incoming request a Proto
	if utils.Contains(contentType, protobufContentType) {
		decoder := &ptrace.ProtoUnmarshaler{}
		if _, err := decoder.UnmarshalTraces(body); err != nil {
			log.Error().Err(err).Msg("failed to unmarshall JSON request body using ptrace")
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
	}

	// send the telemetry to the producer
	if err := col.producer.Write(producer.PacketTelemetry{
		Name:    col.config.Collector.TracesTopic,
		Message: body,
	}); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, "ok")
}
