package indexer

import (
	"fmt"
	"strings"
	"time"

	"github.com/spaghettifunk/anima/internal/storage"
)

type MetricObservation struct {
	Tenant         string            `json:"tenant"`
	Cluster        string            `json:"cluster"`
	Source         string            `json:"source"`
	MetricName     string            `json:"metric_name"`
	MetricType     string            `json:"metric_type"`
	LabelKeys      []string          `json:"label_keys"`
	ResourceLabels map[string]string `json:"resource_labels"`
	ObservedAt     time.Time         `json:"observed_at"`
}

func (m MetricObservation) Normalize(kafkaTimestamp time.Time) (MetricObservation, error) {
	m.Tenant = strings.TrimSpace(m.Tenant)
	m.Cluster = strings.TrimSpace(m.Cluster)
	m.Source = strings.TrimSpace(m.Source)
	m.MetricName = strings.TrimSpace(m.MetricName)
	m.MetricType = strings.TrimSpace(m.MetricType)
	m.LabelKeys = storage.NormalizeStringKeys(m.LabelKeys)

	if m.Tenant == "" {
		return MetricObservation{}, fmt.Errorf("tenant is required")
	}
	if m.Cluster == "" {
		return MetricObservation{}, fmt.Errorf("cluster is required")
	}
	if m.MetricName == "" {
		return MetricObservation{}, fmt.Errorf("metric_name is required")
	}
	if m.Source == "" {
		m.Source = "kafka"
	}
	if m.ObservedAt.IsZero() {
		m.ObservedAt = kafkaTimestamp
	}
	if m.ObservedAt.IsZero() {
		return MetricObservation{}, fmt.Errorf("observed_at is required")
	}

	m.ObservedAt = m.ObservedAt.UTC()
	return m, nil
}

func (m MetricObservation) StorageRow() storage.ObservationRow {
	observedAt := m.ObservedAt.UTC()
	return storage.ObservationRow{
		Day:               storage.DayUTC(observedAt),
		Tenant:            m.Tenant,
		Cluster:           m.Cluster,
		MetricName:        m.MetricName,
		MetricType:        m.MetricType,
		Source:            m.Source,
		ObservedAt:        observedAt,
		LabelKeys:         storage.NormalizeStringKeys(m.LabelKeys),
		ResourceLabelKeys: storage.ResourceLabelKeys(m.ResourceLabels),
	}
}
