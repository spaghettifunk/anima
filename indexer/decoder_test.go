package indexer

import (
	"reflect"
	"testing"
	"time"

	"github.com/spaghettifunk/anima/internal/storage"
)

func TestDecodeObservationsSingleObject(t *testing.T) {
	payload := []byte(`{
		"tenant": "default-tenant ",
		"cluster": "eu-west-1-prod",
		"source": "otlp",
		"metric_name": " http_requests_total ",
		"metric_type": "counter",
		"label_keys": ["pod", " job ", "", "job"],
		"resource_labels": {"service.name": "checkout"},
		"observed_at": "2026-06-03T06:00:00Z"
	}`)

	observations, err := DecodeObservations(payload, time.Time{})
	if err != nil {
		t.Fatalf("DecodeObservations() error = %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("len(observations) = %d, want 1", len(observations))
	}

	got := observations[0]
	if got.Tenant != "default-tenant" {
		t.Fatalf("Tenant = %q, want default-tenant", got.Tenant)
	}
	if got.MetricName != "http_requests_total" {
		t.Fatalf("MetricName = %q, want http_requests_total", got.MetricName)
	}
	if !reflect.DeepEqual(got.LabelKeys, []string{"job", "pod"}) {
		t.Fatalf("LabelKeys = %#v, want job,pod", got.LabelKeys)
	}
}

func TestDecodeObservationsArray(t *testing.T) {
	payload := []byte(`[
		{"tenant": "default-tenant", "cluster": "a", "metric_name": "requests", "observed_at": "2026-06-03T06:00:00Z"},
		{"tenant": "default-tenant", "cluster": "b", "metric_name": "latency", "observed_at": "2026-06-03T07:00:00Z"}
	]`)

	observations, err := DecodeObservations(payload, time.Time{})
	if err != nil {
		t.Fatalf("DecodeObservations() error = %v", err)
	}
	if len(observations) != 2 {
		t.Fatalf("len(observations) = %d, want 2", len(observations))
	}
}

func TestDecodeObservationsRejectsInvalidMessages(t *testing.T) {
	payload := []byte(`{"tenant": "default-tenant", "cluster": "a", "metric_name": " "}`)

	if _, err := DecodeObservations(payload, time.Now()); err == nil {
		t.Fatal("DecodeObservations() error = nil, want error")
	}
}

func TestDecodeObservationsDefaultsSourceAndObservedAt(t *testing.T) {
	kafkaTimestamp := time.Date(2026, 6, 3, 6, 0, 0, 0, time.UTC)
	payload := []byte(`{"tenant": "default-tenant", "cluster": "a", "metric_name": "requests"}`)

	observations, err := DecodeObservations(payload, kafkaTimestamp)
	if err != nil {
		t.Fatalf("DecodeObservations() error = %v", err)
	}

	got := observations[0]
	if got.Source != "kafka" {
		t.Fatalf("Source = %q, want kafka", got.Source)
	}
	if !got.ObservedAt.Equal(kafkaTimestamp) {
		t.Fatalf("ObservedAt = %s, want %s", got.ObservedAt, kafkaTimestamp)
	}
}

func TestResourceLabelKeys(t *testing.T) {
	got := storage.ResourceLabelKeys(map[string]string{
		" service.name ":     "checkout",
		"k8s.namespace.name": "payments",
		"":                   "ignored",
	})

	want := []string{"k8s.namespace.name", "service.name"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("storage.ResourceLabelKeys() = %#v, want %#v", got, want)
	}
}
