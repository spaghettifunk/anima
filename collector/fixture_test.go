package collector

import (
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestFixtureUnmarshals(t *testing.T) {
	body, err := os.ReadFile("../profiling/metrics.json")
	if err != nil {
		t.Fatalf("read profiling fixture: %v", err)
	}

	md, err := (&pmetric.JSONUnmarshaler{}).UnmarshalMetrics(body)
	if err != nil {
		t.Fatalf("unmarshal profiling fixture: %v", err)
	}

	observations := NormalizeMetricObservations(md, "alloy", time.Unix(0, 0))
	if len(observations) == 0 {
		t.Fatal("profiling fixture produced no metric observations")
	}
}
