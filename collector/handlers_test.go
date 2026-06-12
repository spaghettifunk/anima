package collector

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spaghettifunk/anima/internal/producer"
)

type fakeProducer struct {
	healthErr error
}

func (f fakeProducer) Write(producer.PacketTelemetry) error {
	return nil
}

func (f fakeProducer) Health(context.Context) error {
	return f.healthErr
}

func (f fakeProducer) Shutdown() error {
	return nil
}

func (f fakeProducer) In() chan<- any {
	return make(chan any)
}

func TestCollectorReadyz(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		healthErr  error
		wantStatus int
	}{
		{
			name:       "ready",
			wantStatus: http.StatusOK,
		},
		{
			name:       "not ready",
			healthErr:  errors.New("kafka unavailable"),
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			col := &Collector{
				engine:   router,
				producer: fakeProducer{healthErr: tt.healthErr},
			}
			col.setupRoutes()

			request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}

func TestCollectorShutdownStopsListen(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	col := &Collector{
		engine:   router,
		server:   newHTTPServer("127.0.0.1:0", router),
		producer: fakeProducer{},
	}
	col.setupRoutes()

	errCh := make(chan error, 1)
	go func() {
		errCh <- col.Listen()
	}()

	time.Sleep(50 * time.Millisecond)

	if err := col.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			if strings.Contains(err.Error(), "operation not permitted") {
				t.Skipf("skipping listen shutdown test because binding sockets is not permitted: %v", err)
			}
			t.Fatalf("listen: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listen did not stop after shutdown")
	}
}
