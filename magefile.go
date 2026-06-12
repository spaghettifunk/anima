//go:build mage

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
)

const (
	profileFixturePath = "profiling/metrics.json"
	defaultDuration    = "30s"
	defaultRate        = "50/s"
	defaultCollector   = "http://localhost:8500"
	defaultPprof       = "http://127.0.0.1:6060"
)

// Profile contains local collector profiling targets.
type Profile mg.Namespace

// Build contains the go build operations
type Build mg.Namespace

// Build runs the go build command and generates the binary in `bin` folder
func (Build) Build() error {
	return run("go", "build", "-o", "bin/anima", ".")
}

// Clean runs the removal of the `bin` folder
func (Build) Clean() error {
	return run("rm", "-rf", "bin")
}

// Test runs the full Go test suite.
func Test() error {
	return run("go", "test", "./...")
}

// Collector starts the anima collector service.
func Collector() error {
	return runService("collector")
}

// Indexer starts the metric observation and query audit indexer service.
func Indexer() error {
	return runService("indexer")
}

// EntityResolver starts the OTLP entity resolver service.
func EntityResolver() error {
	return runService("entityresolver")
}

// Enricher starts the usage evidence enricher service.
func Enricher() error {
	return runService("enricher")
}

// Reporter starts the metric usage reporter service.
func Reporter() error {
	return runService("reporter")
}

// QueryAuditProxy starts the Thanos query audit proxy service.
func QueryAuditProxy() error {
	return runService("query-audit-proxy")
}

// Fixture writes the deterministic OTLP metrics payload used by profiling.
func (Profile) Fixture() error {
	if err := os.MkdirAll(filepath.Dir(profileFixturePath), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(profileFixturePath, []byte(metricsFixture), 0o644); err != nil {
		return err
	}

	fmt.Printf("wrote %s\n", profileFixturePath)
	return nil
}

// Clean removes generated profiling outputs.
func (Profile) Clean() error {
	return os.RemoveAll(filepath.Join("profiling", "out"))
}

// Collector profiles a running local collector using Vegeta and pprof.
func (p Profile) Collector() error {
	if err := p.Fixture(); err != nil {
		return err
	}

	if _, err := exec.LookPath("vegeta"); err != nil {
		return fmt.Errorf("vegeta is required for profiling: install it and ensure it is in PATH")
	}

	durationText := envDefault("PROFILE_DURATION", defaultDuration)
	duration, err := time.ParseDuration(durationText)
	if err != nil {
		return fmt.Errorf("invalid PROFILE_DURATION %q: %w", durationText, err)
	}
	if duration <= 0 {
		return fmt.Errorf("PROFILE_DURATION must be positive")
	}

	rate := envDefault("PROFILE_RATE", defaultRate)
	collectorURL := strings.TrimRight(envDefault("PROFILE_COLLECTOR_URL", defaultCollector), "/")
	pprofURL := strings.TrimRight(envDefault("PROFILE_PPROF_URL", defaultPprof), "/")

	if err := requireHTTPStatus(joinURL(collectorURL, "/healthz"), http.StatusOK); err != nil {
		return fmt.Errorf("collector is not healthy at %s: %w", collectorURL, err)
	}
	if err := requireHTTPStatus(joinURL(pprofURL, "/debug/pprof/"), http.StatusOK); err != nil {
		return fmt.Errorf("collector pprof is not available at %s: %w", pprofURL, err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	outDir := filepath.Join("profiling", "out", timestamp)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	fixturePath, err := filepath.Abs(profileFixturePath)
	if err != nil {
		return err
	}

	targetsPath := filepath.Join(outDir, "targets.txt")
	targets := fmt.Sprintf("POST %s\nContent-Type: application/json\n@%s\n\n", joinURL(collectorURL, "/telemetry/v1/metrics"), fixturePath)
	if err := os.WriteFile(targetsPath, []byte(targets), 0o644); err != nil {
		return err
	}

	cpuPath := filepath.Join(outDir, "cpu.pprof")
	profileErr := make(chan error, 1)
	go func() {
		seconds := pprofSeconds(duration)
		timeout := duration + 15*time.Second
		profileErr <- downloadFile(fmt.Sprintf("%s/debug/pprof/profile?seconds=%d", pprofURL, seconds), cpuPath, timeout)
	}()

	time.Sleep(500 * time.Millisecond)

	vegetaBin := filepath.Join(outDir, "vegeta.bin")
	if err := run(
		"vegeta", "attack",
		"-duration", durationText,
		"-rate", rate,
		"-targets", targetsPath,
		"-output", vegetaBin,
	); err != nil {
		return err
	}

	if err := <-profileErr; err != nil {
		return err
	}

	if err := downloadFile(joinURL(pprofURL, "/debug/pprof/heap"), filepath.Join(outDir, "heap.pprof"), 10*time.Second); err != nil {
		return err
	}
	if err := downloadFile(joinURL(pprofURL, "/debug/pprof/allocs"), filepath.Join(outDir, "alloc_space.pprof"), 10*time.Second); err != nil {
		return err
	}
	if err := downloadFile(joinURL(pprofURL, "/debug/pprof/goroutine?debug=0"), filepath.Join(outDir, "goroutine.pprof"), 10*time.Second); err != nil {
		return err
	}

	if err := run(
		"vegeta", "report",
		"--type", "text",
		"--output", filepath.Join(outDir, "vegeta.txt"),
		vegetaBin,
	); err != nil {
		return err
	}
	if err := run(
		"vegeta", "report",
		"--type", "json",
		"--output", filepath.Join(outDir, "vegeta.json"),
		vegetaBin,
	); err != nil {
		return err
	}

	fmt.Printf("\nprofiling artifacts written to %s\n", outDir)
	fmt.Printf("CPU profile:        go tool pprof -http=:0 %s\n", cpuPath)
	fmt.Printf("Heap profile:       go tool pprof -http=:0 %s\n", filepath.Join(outDir, "heap.pprof"))
	fmt.Printf("Alloc-space profile: go tool pprof -http=:0 -sample_index=alloc_space %s\n", filepath.Join(outDir, "alloc_space.pprof"))
	fmt.Printf("Vegeta report:      %s\n", filepath.Join(outDir, "vegeta.txt"))
	return nil
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

func pprofSeconds(duration time.Duration) int {
	seconds := int((duration + time.Second - time.Nanosecond) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}

func requireHTTPStatus(url string, want int) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != want {
		return fmt.Errorf("status %d, want %d", resp.StatusCode, want)
	}

	return nil
}

func downloadFile(url, path string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}

	return nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runService(name string) error {
	return run("go", "run", "main.go", name)
}

const metricsFixture = `{
  "resourceMetrics": [
    {
      "resource": {
        "attributes": [
          {"key": "tenant_id", "value": {"stringValue": "default-tenant"}},
          {"key": "cluster", "value": {"stringValue": "eu-west-1"}},
          {"key": "namespace", "value": {"stringValue": "checkout"}},
          {"key": "service.name", "value": {"stringValue": "checkout-api"}},
          {"key": "service_name", "value": {"stringValue": "checkout-api"}},
          {"key": "deployment.environment", "value": {"stringValue": "profiling"}}
        ]
      },
      "scopeMetrics": [
        {
          "scope": {"name": "anima.profile.generator", "version": "1.0.0"},
          "metrics": [
            {
              "name": "checkout_active_requests",
              "description": "Active checkout requests",
              "unit": "1",
              "gauge": {
                "dataPoints": [
                  {
                    "attributes": [
                      {"key": "route", "value": {"stringValue": "/checkout"}},
                      {"key": "method", "value": {"stringValue": "POST"}}
                    ],
                    "timeUnixNano": "1718000000000000000",
                    "asInt": "12"
                  },
                  {
                    "attributes": [
                      {"key": "route", "value": {"stringValue": "/cart"}},
                      {"key": "method", "value": {"stringValue": "GET"}}
                    ],
                    "timeUnixNano": "1718000000000000000",
                    "asInt": "4"
                  }
                ]
              }
            },
            {
              "name": "checkout_requests_total",
              "description": "Total checkout requests",
              "unit": "1",
              "sum": {
                "aggregationTemporality": 2,
                "isMonotonic": true,
                "dataPoints": [
                  {
                    "attributes": [
                      {"key": "route", "value": {"stringValue": "/checkout"}},
                      {"key": "method", "value": {"stringValue": "POST"}},
                      {"key": "status_code", "value": {"stringValue": "200"}}
                    ],
                    "startTimeUnixNano": "1717999900000000000",
                    "timeUnixNano": "1718000000000000000",
                    "asInt": "4242"
                  },
                  {
                    "attributes": [
                      {"key": "route", "value": {"stringValue": "/checkout"}},
                      {"key": "method", "value": {"stringValue": "POST"}},
                      {"key": "status_code", "value": {"stringValue": "500"}}
                    ],
                    "startTimeUnixNano": "1717999900000000000",
                    "timeUnixNano": "1718000000000000000",
                    "asInt": "17"
                  }
                ]
              }
            },
            {
              "name": "checkout_request_duration_seconds",
              "description": "Checkout request duration",
              "unit": "s",
              "histogram": {
                "aggregationTemporality": 2,
                "dataPoints": [
                  {
                    "attributes": [
                      {"key": "route", "value": {"stringValue": "/checkout"}},
                      {"key": "method", "value": {"stringValue": "POST"}}
                    ],
                    "startTimeUnixNano": "1717999900000000000",
                    "timeUnixNano": "1718000000000000000",
                    "count": "42",
                    "sum": 8.25,
                    "bucketCounts": ["12", "20", "8", "2"],
                    "explicitBounds": [0.05, 0.1, 0.25]
                  }
                ]
              }
            }
          ]
        },
        {
          "scope": {"name": "anima.profile.generator.duplicate", "version": "1.0.0"},
          "metrics": [
            {
              "name": "checkout_requests_total",
              "description": "Total checkout requests duplicate scope",
              "unit": "1",
              "sum": {
                "aggregationTemporality": 2,
                "isMonotonic": true,
                "dataPoints": [
                  {
                    "attributes": [
                      {"key": "route", "value": {"stringValue": "/checkout"}},
                      {"key": "method", "value": {"stringValue": "POST"}},
                      {"key": "status_code", "value": {"stringValue": "200"}}
                    ],
                    "startTimeUnixNano": "1717999900000000000",
                    "timeUnixNano": "1718000005000000000",
                    "asInt": "4260"
                  }
                ]
              }
            }
          ]
        }
      ]
    },
    {
      "resource": {
        "attributes": [
          {"key": "tenant_id", "value": {"stringValue": "default-tenant"}},
          {"key": "cluster", "value": {"stringValue": "eu-west-1"}},
          {"key": "namespace", "value": {"stringValue": "payments"}},
          {"key": "service.name", "value": {"stringValue": "payments-api"}},
          {"key": "service_name", "value": {"stringValue": "payments-api"}},
          {"key": "deployment.environment", "value": {"stringValue": "profiling"}}
        ]
      },
      "scopeMetrics": [
        {
          "scope": {"name": "anima.profile.generator", "version": "1.0.0"},
          "metrics": [
            {
              "name": "payment_queue_depth",
              "description": "Payment queue depth",
              "unit": "1",
              "gauge": {
                "dataPoints": [
                  {
                    "attributes": [
                      {"key": "queue", "value": {"stringValue": "capture"}},
                      {"key": "priority", "value": {"stringValue": "high"}}
                    ],
                    "timeUnixNano": "1718000000000000000",
                    "asInt": "31"
                  }
                ]
              }
            },
            {
              "name": "payment_batch_size",
              "description": "Payment batch size distribution",
              "unit": "1",
              "summary": {
                "dataPoints": [
                  {
                    "attributes": [
                      {"key": "queue", "value": {"stringValue": "capture"}}
                    ],
                    "startTimeUnixNano": "1717999900000000000",
                    "timeUnixNano": "1718000000000000000",
                    "count": "20",
                    "sum": 200,
                    "quantileValues": [
                      {"quantile": 0.5, "value": 8},
                      {"quantile": 0.9, "value": 16},
                      {"quantile": 0.99, "value": 24}
                    ]
                  }
                ]
              }
            }
          ]
        }
      ]
    }
  ]
}
`
