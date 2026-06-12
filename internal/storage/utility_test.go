package storage

import (
	"reflect"
	"testing"
	"time"
)

func TestDayUTC(t *testing.T) {
	input := time.Date(2026, 6, 3, 23, 30, 0, 0, time.FixedZone("CEST", 2*60*60))

	got := DayUTC(input)
	want := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("DayUTC() = %s, want %s", got, want)
	}
}

func TestNormalizeStringKeys(t *testing.T) {
	got := NormalizeStringKeys([]string{"pod", " job ", "", "pod"})
	want := []string{"job", "pod"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeStringKeys() = %#v, want %#v", got, want)
	}
}

func TestResourceLabelKeys(t *testing.T) {
	got := ResourceLabelKeys(map[string]string{
		" service.name ":     "checkout",
		"k8s.namespace.name": "payments",
		"":                   "ignored",
	})
	want := []string{"k8s.namespace.name", "service.name"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResourceLabelKeys() = %#v, want %#v", got, want)
	}
}
