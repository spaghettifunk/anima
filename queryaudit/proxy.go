package queryaudit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/spaghettifunk/anima/internal/config"
	"github.com/spaghettifunk/anima/internal/producer"
	"github.com/spaghettifunk/anima/internal/usage"
)

type Proxy struct {
	config   config.Config
	engine   *gin.Engine
	upstream *url.URL
	proxy    *httputil.ReverseProxy
	server   *http.Server
	events   chan QueryAuditEvent
	producer producer.Producer

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	droppedEvents atomic.Int64
	publishErrors atomic.Int64
}

type proxyErrorContextKey struct{}

func New(config config.Config) (*Proxy, error) {
	kp, err := producer.NewKafkaProducer(config.Producer.Brokers)
	if err != nil {
		return nil, err
	}
	go kp.ProcessResponse()

	upstream, err := url.Parse(config.QueryAuditProxy.Upstream)
	if err != nil {
		return nil, fmt.Errorf("parse upstream: %w", err)
	}
	if upstream.Scheme == "" || upstream.Host == "" {
		return nil, fmt.Errorf("upstream must include scheme and host")
	}

	ctx, cancel := context.WithCancel(context.Background())
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	p := &Proxy{
		config:   config,
		engine:   router,
		upstream: upstream,
		events:   make(chan QueryAuditEvent, config.QueryAuditProxy.EventBufferSize),
		producer: kp,
		ctx:      ctx,
		cancel:   cancel,
	}

	p.proxy = newReverseProxy(upstream)
	p.setupRoutes()
	p.server = newHTTPServer(p.address(), router)

	p.wg.Add(1)
	go p.publishLoop()

	return p, nil
}

func (p *Proxy) Listen() error {
	if p.server == nil {
		p.server = newHTTPServer(p.address(), p.engine)
	}
	if err := p.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (p *Proxy) Shutdown() error {
	p.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	if p.server != nil {
		err = p.server.Shutdown(ctx)
	}
	p.wg.Wait()
	if p.producer != nil {
		if shutdownErr := p.producer.Shutdown(); shutdownErr != nil && err == nil {
			err = shutdownErr
		}
	}
	return err
}

func (p *Proxy) setupRoutes() {
	for _, method := range []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace,
	} {
		p.engine.Handle(method, "/*proxyPath", p.handle)
	}
}

func (p *Proxy) handle(c *gin.Context) {
	started := time.Now()
	event, audit := ExtractAuditEvent(c.Request, p.config.QueryAuditProxy.LogRawQuery, started)

	var proxyErr string
	ctx := context.WithValue(c.Request.Context(), proxyErrorContextKey{}, &proxyErr)
	p.proxy.ServeHTTP(proxyWriter{writer: c.Writer}, c.Request.WithContext(ctx))

	if !audit {
		return
	}

	status := c.Writer.Status()
	if status == 0 {
		status = http.StatusOK
	}
	event.StatusCode = status
	event.DurationMs = time.Since(started).Milliseconds()
	if proxyErr != "" {
		event.Error = proxyErr
	}
	p.enqueue(event)
}

func (p *Proxy) enqueue(event QueryAuditEvent) bool {
	select {
	case p.events <- event:
		return true
	default:
		dropped := p.droppedEvents.Add(1)
		log.Warn().Int64("dropped_events", dropped).Msg("query audit event queue full; dropping event")
		return false
	}
}

func (p *Proxy) publishLoop() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case event := <-p.events:
			if err := p.publish(event); err != nil {
				failures := p.publishErrors.Add(1)
				log.Error().Err(err).Int64("publish_errors", failures).Msg("failed to publish query audit event")
			}
		}
	}
}

func (p *Proxy) address() string {
	return fmt.Sprintf("%s:%d", p.config.QueryAuditProxy.Address, p.config.QueryAuditProxy.Port)
}

func newReverseProxy(upstream *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if proxyErr, ok := r.Context().Value(proxyErrorContextKey{}).(*string); ok {
			*proxyErr = err.Error()
		}
		log.Warn().Err(err).Str("upstream", upstream.String()).Msg("query audit proxy upstream request failed")
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
	}
	return proxy
}

func newHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

type proxyWriter struct {
	writer gin.ResponseWriter
}

func (w proxyWriter) Header() http.Header {
	return w.writer.Header()
}

func (w proxyWriter) WriteHeader(statusCode int) {
	w.writer.WriteHeader(statusCode)
}

func (w proxyWriter) Write(body []byte) (int, error) {
	return w.writer.Write(body)
}

func (w proxyWriter) Flush() {
	w.writer.Flush()
}

func (p *Proxy) publish(event QueryAuditEvent) error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.producer.Write(producer.PacketTelemetry{
		Name:    p.config.QueryAuditProxy.AuditTopic,
		Message: payload,
		Metadata: map[string]string{
			"method": event.Method,
			"path":   event.Path,
		},
	})
}

func ExtractAuditEvent(r *http.Request, logRawQuery bool, timestamp time.Time) (QueryAuditEvent, bool) {
	event := QueryAuditEvent{
		Timestamp:  timestamp.UTC(),
		Method:     r.Method,
		Path:       r.URL.Path,
		RawQuery:   r.URL.RawQuery,
		UserAgent:  r.UserAgent(),
		RemoteAddr: firstNonEmptyHeader(r, "X-Forwarded-For"),
		OrgID: firstNonEmptyHeader(
			r,
			"X-Scope-OrgID",
			"X-Grafana-Org-Id",
		),
		DashboardUID: firstNonEmptyHeader(
			r,
			"X-Dashboard-Uid",
			"X-Grafana-Dashboard-Uid",
		),
		PanelID: firstNonEmptyHeader(
			r,
			"X-Panel-Id",
			"X-Grafana-Panel-Id",
		),
		DatasourceUID: firstNonEmptyHeader(
			r,
			"X-Datasource-Uid",
			"X-Grafana-Datasource-Uid",
		),
	}
	if event.RemoteAddr == "" {
		event.RemoteAddr = r.RemoteAddr
	}

	if !strings.HasPrefix(r.URL.Path, "/api/v1") {
		return event, false
	}

	queryValues := r.URL.Query()
	formValues, err := readFormValues(r)
	if err != nil {
		event.Error = err.Error()
	}

	query := firstValue("query", queryValues, formValues)
	if query != "" {
		if logRawQuery {
			event.Query = query
		}
		event.QueryHash = hashQuery(query)
		event.ExtractedMetric = usage.ExtractMetricNames(query)
	}
	event.Start = firstValue("start", queryValues, formValues)
	event.End = firstValue("end", queryValues, formValues)
	event.Time = firstValue("time", queryValues, formValues)
	event.Step = firstValue("step", queryValues, formValues)
	event.Timeout = firstValue("timeout", queryValues, formValues)

	return event, true
}

func readFormValues(r *http.Request) (url.Values, error) {
	if r.Body == nil || !strings.EqualFold(r.Method, http.MethodPost) || !isFormURLEncoded(r.Header.Get("Content-Type")) {
		return nil, nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if closeErr := r.Body.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))

	values, parseErr := url.ParseQuery(string(body))
	if parseErr != nil {
		return values, parseErr
	}
	return values, err
}

func isFormURLEncoded(contentType string) bool {
	contentType, _, _ = strings.Cut(contentType, ";")
	return strings.EqualFold(strings.TrimSpace(contentType), "application/x-www-form-urlencoded")
}

func firstValue(name string, sources ...url.Values) string {
	for _, source := range sources {
		for _, value := range source[name] {
			if strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return ""
}

func firstNonEmptyHeader(r *http.Request, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(r.Header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func hashQuery(query string) string {
	sum := sha256.Sum256([]byte(query))
	return hex.EncodeToString(sum[:])
}
