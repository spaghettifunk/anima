package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/rs/zerolog/log"
	"github.com/spaghettifunk/anima/internal/storage"
	"github.com/spaghettifunk/anima/internal/usage"
	"github.com/spaghettifunk/anima/queryaudit"
)

const (
	UsageKindDirectQuery         = "direct_query"
	UsageKindRangeQuery          = "range_query"
	UsageKindDashboardQuery      = "dashboard_query"
	UsageKindDashboardRangeQuery = "dashboard_range_query"
	UsageKindSeriesLookup        = "series_lookup"
	UsageKindLabelsLookup        = "labels_lookup"
	UsageKindLabelValuesLookup   = "label_values_lookup"
	UsageKindMetadataLookup      = "metadata_lookup"
	UsageKindRulesLookup         = "rules_lookup"
	UsageKindRecordingRuleQuery  = "recording_rule_query"
	UsageKindUnknown             = "unknown"
)

type RecordingRuleResolver interface {
	ResolveRecordingRule(ctx context.Context, tenant string, metricName string) ([]storage.ResolvedMetricUsage, error)
}

type noopRecordingRuleResolver struct{}

func (noopRecordingRuleResolver) ResolveRecordingRule(context.Context, string, string) ([]storage.ResolvedMetricUsage, error) {
	return nil, nil
}

type timeParseWarning struct {
	Field string
	Value string
	Err   error
}

func DecodeQueryAuditEvent(payload []byte) (queryaudit.QueryAuditEvent, error) {
	var event queryaudit.QueryAuditEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return queryaudit.QueryAuditEvent{}, err
	}
	return event, nil
}

func NormalizeQueryEvent(event queryaudit.QueryAuditEvent, kafkaTimestamp time.Time, now time.Time) (storage.QueryEventRecord, []timeParseWarning, error) {
	id, err := uuid.GenerateUUID()
	if err != nil {
		return storage.QueryEventRecord{}, nil, err
	}
	if now.IsZero() {
		now = time.Now()
	}

	observedAt := event.Timestamp
	if observedAt.IsZero() {
		observedAt = kafkaTimestamp
	}
	if observedAt.IsZero() {
		observedAt = now
		log.Warn().Msg("query audit event missing timestamp; using current time")
	}
	observedAt = observedAt.UTC()

	warnings := make([]timeParseWarning, 0, 3)
	rangeStart := parsePrometheusTimeField("start", event.Start, &warnings)
	rangeEnd := parsePrometheusTimeField("end", event.End, &warnings)
	queryTime := parsePrometheusTimeField("time", event.Time, &warnings)

	queryRaw := optionalString(event.Query)
	queryHash := strings.TrimSpace(event.QueryHash)
	if queryHash == "" {
		if candidate := queryCandidate(event); candidate != "" {
			queryHash = hashQuery(candidate)
		}
	}

	record := storage.QueryEventRecord{
		ID:            id,
		ObservedAt:    observedAt,
		Tenant:        strings.TrimSpace(event.OrgID),
		Source:        "thanos",
		Path:          strings.TrimSpace(event.Path),
		Method:        strings.TrimSpace(event.Method),
		UsageKind:     ClassifyUsageKind(event),
		QueryHash:     queryHash,
		QueryRaw:      queryRaw,
		RangeStart:    rangeStart,
		RangeEnd:      rangeEnd,
		QueryTime:     queryTime,
		Step:          optionalString(event.Step),
		Timeout:       optionalString(event.Timeout),
		StatusCode:    event.StatusCode,
		DurationMs:    event.DurationMs,
		DashboardUID:  optionalString(event.DashboardUID),
		PanelID:       optionalString(event.PanelID),
		DatasourceUID: optionalString(event.DatasourceUID),
		RemoteAddr:    optionalString(event.RemoteAddr),
		UserAgent:     optionalString(event.UserAgent),
		Error:         optionalString(event.Error),
	}

	return record, warnings, nil
}

func BuildMetricQueryUsageRecords(ctx context.Context, event queryaudit.QueryAuditEvent, record storage.QueryEventRecord, resolver RecordingRuleResolver) []storage.MetricQueryUsageRecord {
	if strings.TrimSpace(record.Tenant) == "" {
		log.Warn().
			Str("path", record.Path).
			Str("usage_kind", record.UsageKind).
			Msg("query audit event missing tenant; skipping metric query usage rows")
		return nil
	}
	if !shouldCreateMetricUsage(record.UsageKind) {
		return nil
	}
	if resolver == nil {
		resolver = noopRecordingRuleResolver{}
	}

	metricNames, parseErr := metricNamesForQueryAuditEvent(event)
	if parseErr != nil {
		log.Warn().
			Err(parseErr).
			Str("path", record.Path).
			Msg("failed to parse query audit PromQL; using extracted metrics fallback if available")
	}
	if len(metricNames) == 0 {
		return nil
	}

	rows := make([]storage.MetricQueryUsageRecord, 0, len(metricNames))
	for _, metricName := range metricNames {
		resolved, err := resolver.ResolveRecordingRule(ctx, record.Tenant, metricName)
		if err != nil {
			log.Warn().
				Err(err).
				Str("tenant", record.Tenant).
				Str("metric_name", metricName).
				Msg("failed to resolve recording rule metric usage")
			resolved = nil
		}

		directRow, err := metricQueryUsageRecord(record, metricName, record.UsageKind, len(resolved) > 0, nil)
		if err != nil {
			log.Error().Err(err).Str("metric_name", metricName).Msg("failed to create metric query usage row")
			continue
		}
		rows = append(rows, directRow)

		seenResolved := map[string]struct{}{}
		for _, resolvedMetric := range resolved {
			resolvedMetricName := strings.TrimSpace(resolvedMetric.MetricName)
			if resolvedMetricName == "" {
				continue
			}
			if _, ok := seenResolved[resolvedMetricName]; ok {
				continue
			}
			seenResolved[resolvedMetricName] = struct{}{}

			resolvedFrom := metricName
			indirectRow, err := metricQueryUsageRecord(record, resolvedMetricName, UsageKindRecordingRuleQuery, false, &resolvedFrom)
			if err != nil {
				log.Error().Err(err).Str("metric_name", resolvedMetricName).Msg("failed to create resolved metric query usage row")
				continue
			}
			rows = append(rows, indirectRow)
		}
	}

	return rows
}

func ParsePrometheusTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		if math.IsInf(parsed, 0) || math.IsNaN(parsed) {
			return nil, fmt.Errorf("invalid unix timestamp %q", value)
		}
		seconds, fraction := math.Modf(parsed)
		nanos := int64(math.Round(fraction * float64(time.Second)))
		if nanos == int64(time.Second) {
			seconds++
			nanos = 0
		}
		t := time.Unix(int64(seconds), nanos).UTC()
		return &t, nil
	}

	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		t := parsed.UTC()
		return &t, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		t := parsed.UTC()
		return &t, nil
	}

	return nil, fmt.Errorf("invalid prometheus time %q", value)
}

func ClassifyUsageKind(event queryaudit.QueryAuditEvent) string {
	path := strings.TrimRight(strings.TrimSpace(event.Path), "/")
	hasDashboardMetadata := strings.TrimSpace(event.DashboardUID) != "" || strings.TrimSpace(event.PanelID) != ""

	switch {
	case path == "/api/v1/query":
		if hasDashboardMetadata {
			return UsageKindDashboardQuery
		}
		return UsageKindDirectQuery
	case path == "/api/v1/query_range":
		if hasDashboardMetadata {
			return UsageKindDashboardRangeQuery
		}
		return UsageKindRangeQuery
	case path == "/api/v1/series":
		return UsageKindSeriesLookup
	case path == "/api/v1/labels":
		return UsageKindLabelsLookup
	case strings.HasPrefix(path, "/api/v1/label/") && strings.HasSuffix(path, "/values"):
		return UsageKindLabelValuesLookup
	case path == "/api/v1/metadata":
		return UsageKindMetadataLookup
	case path == "/api/v1/rules":
		return UsageKindRulesLookup
	default:
		return UsageKindUnknown
	}
}

func shouldCreateMetricUsage(usageKind string) bool {
	switch usageKind {
	case UsageKindDirectQuery, UsageKindRangeQuery, UsageKindDashboardQuery, UsageKindDashboardRangeQuery:
		return true
	default:
		return false
	}
}

func parsePrometheusTimeField(field string, value string, warnings *[]timeParseWarning) *time.Time {
	parsed, err := ParsePrometheusTime(value)
	if err != nil {
		*warnings = append(*warnings, timeParseWarning{Field: field, Value: value, Err: err})
		return nil
	}
	return parsed
}

func metricNamesForQueryAuditEvent(event queryaudit.QueryAuditEvent) ([]string, error) {
	candidate := queryCandidate(event)
	if candidate != "" {
		metricNames, err := usage.ExtractMetricNamesWithError(candidate)
		if err == nil && len(metricNames) > 0 {
			return storage.NormalizeStringKeys(metricNames), nil
		}
		if err != nil {
			return storage.NormalizeStringKeys(event.ExtractedMetric), err
		}
	}
	return storage.NormalizeStringKeys(event.ExtractedMetric), nil
}

func queryCandidate(event queryaudit.QueryAuditEvent) string {
	if event.Query != "" {
		return event.Query
	}
	values, err := url.ParseQuery(event.RawQuery)
	if err != nil {
		return ""
	}
	for _, query := range values["query"] {
		if strings.TrimSpace(query) != "" {
			return query
		}
	}
	return ""
}

func metricQueryUsageRecord(record storage.QueryEventRecord, metricName string, usageKind string, isRecordingRule bool, resolvedFrom *string) (storage.MetricQueryUsageRecord, error) {
	id, err := uuid.GenerateUUID()
	if err != nil {
		return storage.MetricQueryUsageRecord{}, err
	}
	return storage.MetricQueryUsageRecord{
		ID:                        id,
		ObservedAt:                record.ObservedAt,
		Tenant:                    record.Tenant,
		Source:                    record.Source,
		MetricName:                strings.TrimSpace(metricName),
		QueryEventID:              record.ID,
		QueryHash:                 record.QueryHash,
		DashboardUID:              record.DashboardUID,
		PanelID:                   record.PanelID,
		DatasourceUID:             record.DatasourceUID,
		UsageKind:                 usageKind,
		IsRecordingRule:           isRecordingRule,
		ResolvedFromRecordingRule: optionalStringPtr(resolvedFrom),
	}, nil
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func optionalStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	return optionalString(*value)
}

func hashQuery(query string) string {
	sum := sha256.Sum256([]byte(query))
	return hex.EncodeToString(sum[:])
}
