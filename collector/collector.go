package collector

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	ginzip "github.com/gin-contrib/gzip"

	"github.com/gin-gonic/gin"
	"github.com/spaghettifunk/anima/internal/config"
	"github.com/spaghettifunk/anima/internal/producer"

	ginprometheus "github.com/zsais/go-gin-prometheus"
)

type Collector struct {
	engine   *gin.Engine
	server   *http.Server
	address  string
	config   config.Config
	producer producer.Producer
}

func New(config config.Config) (*Collector, error) {
	kp, err := producer.NewKafkaProducer(config.Producer.Brokers)
	if err != nil {
		return nil, err
	}

	// process the responses asynchronous
	go func() {
		kp.ProcessResponse()
	}()

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(ginzip.Gzip(ginzip.DefaultCompression, ginzip.WithDecompressFn(ginzip.DefaultDecompressHandle)))

	p := ginprometheus.NewPrometheus("gin")
	p.Use(router)

	// server
	addr := fmt.Sprintf("%s:%d", config.Collector.Address, config.Collector.Port)
	c := &Collector{
		engine:   router,
		server:   newHTTPServer(addr, router),
		address:  addr,
		config:   config,
		producer: kp,
	}

	c.setupRoutes()

	return c, nil
}

func (c *Collector) Listen() error {
	if c.server == nil {
		c.server = newHTTPServer(c.address, c.engine)
	}

	if err := c.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (c *Collector) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	if c.server != nil {
		err = c.server.Shutdown(ctx)
	}
	if c.producer != nil {
		if shutdownErr := c.producer.Shutdown(); shutdownErr != nil && err == nil {
			err = shutdownErr
		}
	}

	return err
}

func (c *Collector) setupRoutes() {
	c.engine.GET("/healthz", Healthz)
	c.engine.GET("/readyz", c.Readyz)

	g := c.engine.Group("/telemetry/v1")
	g.POST("/logs", c.Logs)
	g.POST("/metrics", c.Metrics)
	g.POST("/traces", c.Traces)
}

func newHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
