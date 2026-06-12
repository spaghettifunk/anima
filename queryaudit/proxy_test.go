package queryaudit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spaghettifunk/anima/internal/config"
	"github.com/spaghettifunk/anima/internal/producer"
	"github.com/spf13/viper"
)

type recordingProducer struct {
	err     error
	packets chan producer.PacketTelemetry
	mu      sync.Mutex
	seen    []producer.PacketTelemetry
	in      chan any
}

func newRecordingProducer(buffer int) *recordingProducer {
	return &recordingProducer{
		packets: make(chan producer.PacketTelemetry, buffer),
		in:      make(chan any),
	}
}

func (p *recordingProducer) Write(packet producer.PacketTelemetry) error {
	p.mu.Lock()
	p.seen = append(p.seen, packet)
	p.mu.Unlock()
	p.packets <- packet
	return p.err
}

func (p *recordingProducer) Health(context.Context) error {
	return nil
}

func (p *recordingProducer) Shutdown() error {
	return nil
}

func (p *recordingProducer) In() chan<- any {
	return p.in
}

func (p *recordingProducer) event(t *testing.T) QueryAuditEvent {
	t.Helper()
	select {
	case packet := <-p.packets:
		var event QueryAuditEvent
		if err := json.Unmarshal(packet.Message, &event); err != nil {
			t.Fatal(err)
		}
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for audit event")
		return QueryAuditEvent{}
	}
}

func testConfig(t *testing.T, upstream string, bufferSize int, logRawQuery bool) config.Config {
	t.Helper()

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("producer.brokers", "kafka1:29092")
	viper.Set("query_audit_proxy.address", "127.0.0.1")
	viper.Set("query_audit_proxy.port", 8080)
	viper.Set("query_audit_proxy.upstream", upstream)
	viper.Set("query_audit_proxy.log_raw_query", logRawQuery)
	viper.Set("query_audit_proxy.event_buffer_size", bufferSize)
	viper.Set("query_audit_proxy.audit_topic", "query-audit-events-v1")

	return *config.New()
}

func newTestProxy(t *testing.T, cfg config.Config, prod producer.Producer) *Proxy {
	t.Helper()

	upstream, err := url.Parse(cfg.QueryAuditProxy.Upstream)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	engine := gin.New()
	p := &Proxy{
		config:   cfg,
		engine:   engine,
		upstream: upstream,
		proxy:    newReverseProxy(upstream),
		server:   newHTTPServer(fmt.Sprintf("%s:%d", cfg.QueryAuditProxy.Address, cfg.QueryAuditProxy.Port), engine),
		events:   make(chan QueryAuditEvent, cfg.QueryAuditProxy.EventBufferSize),
		producer: prod,
		ctx:      ctx,
		cancel:   cancel,
	}
	p.setupRoutes()
	p.wg.Add(1)
	go p.publishLoop()
	return p
}

func TestExtractAuditEventFromGET(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/query_range?query=rate(http_requests_total%5B5m%5D)&start=1&end=2&step=30s&timeout=10s", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	req.Header.Set("X-Grafana-Org-Id", "7")
	req.Header.Set("X-Scope-OrgID", "tenant-a")
	req.Header.Set("X-Grafana-Dashboard-Uid", "dash-a")
	req.Header.Set("X-Dashboard-Uid", "dash-primary")
	req.Header.Set("X-Grafana-Panel-Id", "9")
	req.Header.Set("X-Panel-Id", "3")
	req.Header.Set("X-Grafana-Datasource-Uid", "thanos")
	req.Header.Set("X-Datasource-Uid", "mimir")
	req.Header.Set("User-Agent", "grafana")

	event, audit := ExtractAuditEvent(req, false, now)
	if !audit {
		t.Fatal("audit = false, want true")
	}
	if event.Timestamp != now || event.Method != http.MethodGet || event.Path != "/api/v1/query_range" {
		t.Fatalf("basic event fields = %#v", event)
	}
	if event.RawQuery == "" || event.Query != "" {
		t.Fatalf("RawQuery = %q Query = %q, want raw query and no plain query", event.RawQuery, event.Query)
	}
	wantHash := sha256Hex("rate(http_requests_total[5m])")
	if event.QueryHash != wantHash {
		t.Fatalf("QueryHash = %q, want %q", event.QueryHash, wantHash)
	}
	if got, want := event.ExtractedMetric, []string{"http_requests_total"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ExtractedMetric = %#v, want %#v", got, want)
	}
	if event.Start != "1" || event.End != "2" || event.Step != "30s" || event.Timeout != "10s" {
		t.Fatalf("query params not extracted: %#v", event)
	}
	if event.RemoteAddr != "198.51.100.1" || event.UserAgent != "grafana" {
		t.Fatalf("client fields = remote:%q user_agent:%q", event.RemoteAddr, event.UserAgent)
	}
	if event.OrgID != "tenant-a" || event.DashboardUID != "dash-primary" || event.PanelID != "3" || event.DatasourceUID != "mimir" {
		t.Fatalf("header fields = %#v", event)
	}
}

func TestExtractAuditEventFromPOSTFormRestoresBodyAndCanLogRawQuery(t *testing.T) {
	body := "query=sum+by+%28job%29+%28rate%28http_requests_total%5B5m%5D%29%29&time=123&timeout=5s"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query?step=15s", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")

	event, audit := ExtractAuditEvent(req, true, time.Now())
	if !audit {
		t.Fatal("audit = false, want true")
	}
	if event.Query != "sum by (job) (rate(http_requests_total[5m]))" {
		t.Fatalf("Query = %q", event.Query)
	}
	if event.QueryHash != sha256Hex(event.Query) {
		t.Fatalf("QueryHash = %q, want hash of raw query", event.QueryHash)
	}
	if event.Time != "123" || event.Step != "15s" || event.Timeout != "5s" {
		t.Fatalf("params = time:%q step:%q timeout:%q", event.Time, event.Step, event.Timeout)
	}

	restored, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != body {
		t.Fatalf("restored body = %q, want %q", string(restored), body)
	}
}

func TestExtractAuditEventSkipsNonPrometheusAPIPaths(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/graph?query=up", nil)
	if _, audit := ExtractAuditEvent(req, false, time.Now()); audit {
		t.Fatal("audit = true, want false")
	}
}

func TestProxyForwardsRequestAndPublishesAuditEvent(t *testing.T) {
	var gotMethod, gotPath, gotRawQuery, gotHeader, gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		gotHeader = r.Header.Get("X-Test-Header")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		gotBody = string(body)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("upstream-ok"))
	}))
	defer upstream.Close()

	prod := newRecordingProducer(1)
	proxy := newTestProxy(t, testConfig(t, upstream.URL, 4, true), prod)
	defer proxy.Shutdown()

	body := "query=rate%28container_cpu_usage_seconds_total%5B5m%5D%29&start=1"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query_range?end=2", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Test-Header", "kept")
	rr := httptest.NewRecorder()

	proxy.engine.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted || rr.Body.String() != "upstream-ok" {
		t.Fatalf("response = %d %q", rr.Code, rr.Body.String())
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/query_range" || gotRawQuery != "end=2" || gotHeader != "kept" || gotBody != body {
		t.Fatalf("forwarded request = method:%q path:%q raw:%q header:%q body:%q", gotMethod, gotPath, gotRawQuery, gotHeader, gotBody)
	}

	event := prod.event(t)
	if event.StatusCode != http.StatusAccepted || event.DurationMs < 0 {
		t.Fatalf("event response fields = %#v", event)
	}
	if event.Query != "rate(container_cpu_usage_seconds_total[5m])" || event.Start != "1" || event.End != "2" {
		t.Fatalf("event params = %#v", event)
	}
}

func TestProxyUpstreamErrorStillPublishesAuditEvent(t *testing.T) {
	prod := newRecordingProducer(1)
	proxy := newTestProxy(t, testConfig(t, "http://127.0.0.1:1", 2, false), prod)
	defer proxy.Shutdown()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/query?query=up", nil)
	rr := httptest.NewRecorder()

	proxy.engine.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rr.Code)
	}
	event := prod.event(t)
	if event.StatusCode != http.StatusBadGateway || event.Error == "" {
		t.Fatalf("event = %#v, want status and error", event)
	}
}

func TestQueueFullDropsEvent(t *testing.T) {
	proxy := &Proxy{events: make(chan QueryAuditEvent, 1)}

	if !proxy.enqueue(QueryAuditEvent{Path: "/api/v1/query"}) {
		t.Fatal("first enqueue failed")
	}
	if proxy.enqueue(QueryAuditEvent{Path: "/api/v1/query"}) {
		t.Fatal("second enqueue succeeded, want drop")
	}
	if got := proxy.droppedEvents.Load(); got != 1 {
		t.Fatalf("droppedEvents = %d, want 1", got)
	}
}

func TestProducerErrorDoesNotAffectProxyResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	prod := newRecordingProducer(1)
	prod.err = errors.New("kafka unavailable")
	proxy := newTestProxy(t, testConfig(t, upstream.URL, 2, false), prod)
	defer proxy.Shutdown()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/query?query=up", nil)
	rr := httptest.NewRecorder()
	proxy.engine.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	_ = prod.event(t)
	deadline := time.After(2 * time.Second)
	for proxy.publishErrors.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for publish error")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestFirstValuePrefersQueryStringOverForm(t *testing.T) {
	query := url.Values{"query": {"up"}}
	form := url.Values{"query": {"http_requests_total"}}
	if got := firstValue("query", query, form); got != "up" {
		t.Fatalf("firstValue = %q, want query string value", got)
	}
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
