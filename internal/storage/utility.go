package storage

import (
	"sort"
	"strings"
	"time"
)

type ObservationRow struct {
	Day               time.Time
	Tenant            string
	Cluster           string
	MetricName        string
	MetricType        string
	Source            string
	ObservedAt        time.Time
	LabelKeys         []string
	ResourceLabelKeys []string
}

func DayUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func ResourceLabelKeys(labels map[string]string) []string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	return NormalizeStringKeys(keys)
}

func NormalizeStringKeys(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = struct{}{}
		}
	}

	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
