package collector

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type MetricObservation struct {
	ID             string            `json:"id"`
	Tenant         string            `json:"tenant,omitempty"`
	Cluster        string            `json:"cluster,omitempty"`
	Namespace      string            `json:"namespace,omitempty"`
	Service        string            `json:"service,omitempty"`
	Source         string            `json:"source"`
	MetricName     string            `json:"metric_name"`
	MetricType     string            `json:"metric_type"`
	BaseMetricName string            `json:"base_metric_name,omitempty"`
	Unit           string            `json:"unit,omitempty"`
	Description    string            `json:"description,omitempty"`
	LabelKeys      []string          `json:"label_keys"`
	ResourceLabels map[string]string `json:"resource_labels,omitempty"`
	ObservedAt     time.Time         `json:"observed_at"`
}

func NormalizeMetricObservations(md pmetric.Metrics, source string, observedAt time.Time) []MetricObservation {
	seen := make(map[string]MetricObservation)

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resourceLabels := attrsToStringMap(rm.Resource().Attributes())

		tenant := firstNonEmpty(
			resourceLabels["tenant_id"],
			resourceLabels["tenant"],
			resourceLabels["tenant.id"],
			resourceLabels["organization"],
			resourceLabels["org"],
		)

		cluster := firstNonEmpty(
			resourceLabels["clusterName"],
			resourceLabels["cluster"],
		)

		namespace := firstNonEmpty(
			resourceLabels["namespace"],
			resourceLabels["service_namespace"],
			resourceLabels["kubernetes_namespace"],
		)

		service := firstNonEmpty(
			resourceLabels["service_name"],
			resourceLabels["job"],
			resourceLabels["service"],
		)

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)

			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)

				metricType := metricTypeName(m)
				labelKeys := collectMetricLabelKeys(m)

				obs := MetricObservation{
					Tenant:         tenant,
					Cluster:        cluster,
					Namespace:      namespace,
					Service:        service,
					Source:         source,
					MetricName:     m.Name(),
					MetricType:     metricType,
					BaseMetricName: baseMetricName(m.Name(), metricType),
					Unit:           m.Unit(),
					Description:    m.Description(),
					LabelKeys:      labelKeys,
					ResourceLabels: resourceLabels,
					ObservedAt:     observedAt.UTC(),
				}

				obs.ID = observationID(obs)

				if existing, ok := seen[obs.ID]; ok {
					existing.LabelKeys = unionSorted(existing.LabelKeys, obs.LabelKeys)
					seen[obs.ID] = existing
					continue
				}

				seen[obs.ID] = obs
			}
		}
	}

	out := make([]MetricObservation, 0, len(seen))
	for _, obs := range seen {
		out = append(out, obs)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})

	return out
}

func collectMetricLabelKeys(m pmetric.Metric) []string {
	keys := map[string]struct{}{}

	addAttrs := func(attrs pcommon.Map) {
		attrs.Range(func(k string, _ pcommon.Value) bool {
			keys[k] = struct{}{}
			return true
		})
	}

	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			addAttrs(dps.At(i).Attributes())
		}

	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			addAttrs(dps.At(i).Attributes())
		}

	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			addAttrs(dps.At(i).Attributes())
			keys["le"] = struct{}{}
		}

	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			addAttrs(dps.At(i).Attributes())
		}

	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			addAttrs(dps.At(i).Attributes())
			keys["quantile"] = struct{}{}
		}
	}

	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)

	return out
}

func attrsToStringMap(attrs pcommon.Map) map[string]string {
	out := make(map[string]string, attrs.Len())

	attrs.Range(func(k string, v pcommon.Value) bool {
		out[k] = v.AsString()
		return true
	})

	return out
}

func metricTypeName(m pmetric.Metric) string {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return "gauge"
	case pmetric.MetricTypeSum:
		if m.Sum().IsMonotonic() {
			return "counter"
		}
		return "sum"
	case pmetric.MetricTypeHistogram:
		return "histogram"
	case pmetric.MetricTypeExponentialHistogram:
		return "exponential_histogram"
	case pmetric.MetricTypeSummary:
		return "summary"
	default:
		return "unknown"
	}
}

func baseMetricName(name, metricType string) string {
	switch metricType {
	case "histogram":
		return trimPromSuffix(name, "_bucket", "_sum", "_count")
	case "summary":
		return trimPromSuffix(name, "_sum", "_count")
	default:
		return name
	}
}

func trimPromSuffix(name string, suffixes ...string) string {
	for _, suffix := range suffixes {
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			return name[:len(name)-len(suffix)]
		}
	}
	return name
}

func observationID(obs MetricObservation) string {
	parts := []string{
		obs.Tenant,
		obs.Cluster,
		obs.Namespace,
		obs.Service,
		obs.MetricName,
		obs.MetricType,
	}

	for _, k := range obs.LabelKeys {
		parts = append(parts, k)
	}

	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}

	return hex.EncodeToString(h.Sum(nil))
}

func unionSorted(a, b []string) []string {
	set := make(map[string]struct{}, len(a)+len(b))

	for _, v := range a {
		set[v] = struct{}{}
	}
	for _, v := range b {
		set[v] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}

	sort.Strings(out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
