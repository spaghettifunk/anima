package reporter

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const dayLayout = "2006-01-02"

var (
	protectedMetricPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^up$`),
		regexp.MustCompile(`^scrape_.*`),
		regexp.MustCompile(`^prometheus_.*`),
		regexp.MustCompile(`^thanos_.*`),
		regexp.MustCompile(`^grafana_.*`),
		regexp.MustCompile(`^alertmanager_.*`),
		regexp.MustCompile(`^go_.*`),
		regexp.MustCompile(`^process_.*`),
	}

	allowedSortFields = map[string]struct{}{
		"metric_name":          {},
		"last_seen_at":         {},
		"last_queried_at":      {},
		"observation_count":    {},
		"query_history_count":  {},
		"usage_evidence_count": {},
		"risk":                 {},
		"recommendation":       {},
	}

	allowedRisks = map[MetricRisk]struct{}{
		RiskCritical: {},
		RiskHigh:     {},
		RiskMedium:   {},
		RiskLow:      {},
		RiskUnknown:  {},
	}

	allowedRecommendations = map[MetricRecommendation]struct{}{
		RecommendationKeep:                    {},
		RecommendationKeepOrReviewAggregation: {},
		RecommendationReviewDashboardUsage:    {},
		RecommendationReviewRecentQueryUsage:  {},
		RecommendationDropCandidate:           {},
		RecommendationReview:                  {},
	}
)

type Defaults struct {
	Tenant              string
	LookbackDays        int
	Limit               int
	MaxLimit            int
	MaxQueriedAtEntries int
}

type Service struct {
	repo     Repository
	defaults Defaults
}

func NewService(repo Repository, defaults Defaults) *Service {
	if defaults.LookbackDays <= 0 {
		defaults.LookbackDays = 30
	}
	if defaults.Limit <= 0 {
		defaults.Limit = 100
	}
	if defaults.MaxLimit <= 0 {
		defaults.MaxLimit = 1000
	}
	if defaults.MaxQueriedAtEntries <= 0 {
		defaults.MaxQueriedAtEntries = 50
	}
	if defaults.Limit > defaults.MaxLimit {
		defaults.Limit = defaults.MaxLimit
	}
	return &Service{repo: repo, defaults: defaults}
}

func (s *Service) Health(ctx context.Context) error {
	return s.repo.Health(ctx)
}

func (s *Service) ParseFilters(values url.Values) (MetricReportFilters, error) {
	filters := MetricReportFilters{
		Tenant:            firstNonEmpty(values.Get("tenant"), s.defaults.Tenant),
		LookbackDays:      s.defaults.LookbackDays,
		QueriedInLastDays: s.defaults.LookbackDays,
		Limit:             s.defaults.Limit,
		Offset:            0,
		Sort:              "metric_name",
		Order:             "asc",
	}

	if day := strings.TrimSpace(values.Get("day")); day != "" {
		if _, err := parseDay(day); err != nil {
			return filters, badRequest("invalid day, expected YYYY-MM-DD")
		}
		filters.Day = day
	}
	if value := strings.TrimSpace(values.Get("lookback_days")); value != "" {
		parsed, err := parsePositiveInt(value)
		if err != nil {
			return filters, badRequest("invalid lookback_days, expected a positive integer")
		}
		filters.LookbackDays = parsed
		filters.QueriedInLastDays = parsed
	}
	if value := strings.TrimSpace(values.Get("queried_in_last_days")); value != "" {
		parsed, err := parsePositiveInt(value)
		if err != nil {
			return filters, badRequest("invalid queried_in_last_days, expected a positive integer")
		}
		filters.QueriedInLastDays = parsed
	}

	filters.Cluster = strings.TrimSpace(values.Get("cluster"))
	filters.MetricName = strings.TrimSpace(values.Get("metric_name"))

	if value := strings.TrimSpace(values.Get("recommendation")); value != "" {
		recommendation := MetricRecommendation(value)
		if _, ok := allowedRecommendations[recommendation]; !ok {
			return filters, badRequest("invalid recommendation")
		}
		filters.Recommendation = recommendation
	}
	if value := strings.TrimSpace(values.Get("risk")); value != "" {
		risk := MetricRisk(value)
		if _, ok := allowedRisks[risk]; !ok {
			return filters, badRequest("invalid risk")
		}
		filters.Risk = risk
	}

	var err error
	if filters.UsedInAlerts, err = optionalBool(values, "used_in_alerts"); err != nil {
		return filters, err
	}
	if filters.UsedInRecordingRules, err = optionalBool(values, "used_in_recording_rules"); err != nil {
		return filters, err
	}
	if filters.UsedInDashboards, err = optionalBool(values, "used_in_dashboards"); err != nil {
		return filters, err
	}
	if filters.UsedInQueryHistory, err = optionalBool(values, "used_in_query_history"); err != nil {
		return filters, err
	}
	filters.SourceType = strings.TrimSpace(values.Get("source_type"))
	if filters.UnusedOnly, err = boolWithDefault(values, "unused_only", false); err != nil {
		return filters, err
	}
	if filters.DropCandidatesOnly, err = boolWithDefault(values, "drop_candidates_only", false); err != nil {
		return filters, err
	}

	if value := strings.TrimSpace(values.Get("sort")); value != "" {
		if _, ok := allowedSortFields[value]; !ok {
			return filters, badRequest("invalid sort field")
		}
		filters.Sort = value
	}
	if value := strings.TrimSpace(values.Get("order")); value != "" {
		value = strings.ToLower(value)
		if value != "asc" && value != "desc" {
			return filters, badRequest("invalid order, expected asc or desc")
		}
		filters.Order = value
	}
	if value := strings.TrimSpace(values.Get("limit")); value != "" {
		parsed, err := parsePositiveInt(value)
		if err != nil {
			return filters, badRequest("invalid limit, expected a positive integer")
		}
		filters.Limit = parsed
	}
	if filters.Limit > s.defaults.MaxLimit {
		filters.Limit = s.defaults.MaxLimit
	}
	if value := strings.TrimSpace(values.Get("offset")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return filters, badRequest("invalid offset, expected a non-negative integer")
		}
		filters.Offset = parsed
	}

	if strings.TrimSpace(filters.Tenant) == "" {
		return filters, badRequest("tenant is required")
	}

	return filters, nil
}

func (s *Service) ListMetricReports(ctx context.Context, filters MetricReportFilters) (MetricReportResponse, error) {
	resolved, day, err := s.resolveDay(ctx, filters)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			resolved.Day = ""
			return MetricReportResponse{
				Data:       []MetricReportRow{},
				Pagination: Pagination{Limit: resolved.Limit, Offset: resolved.Offset, Total: 0, HasMore: false},
				Filters:    resolved,
			}, nil
		}
		return MetricReportResponse{}, err
	}

	rows, _, err := s.repo.ListMetricReports(ctx, resolved)
	if err != nil {
		return MetricReportResponse{}, err
	}
	rows = s.decorateRows(rows)
	rows = applyFilters(rows, resolved)
	sortRows(rows, resolved.Sort, resolved.Order)

	total := len(rows)
	rows = paginateRows(rows, resolved.Limit, resolved.Offset)
	return MetricReportResponse{
		Data: rows,
		Pagination: Pagination{
			Limit:   resolved.Limit,
			Offset:  resolved.Offset,
			Total:   total,
			HasMore: resolved.Offset+len(rows) < total,
		},
		Filters: resolvedWithDay(resolved, day),
	}, nil
}

func (s *Service) GetMetricReport(ctx context.Context, filters MetricReportFilters, metricName string) (*MetricReportRow, MetricReportFilters, error) {
	if strings.TrimSpace(metricName) == "" {
		return nil, filters, badRequest("metric_name is required")
	}

	resolved, day, err := s.resolveDay(ctx, filters)
	if err != nil {
		return nil, resolved, err
	}
	row, err := s.repo.GetMetricReport(ctx, resolved.Tenant, metricName, day, resolved.LookbackDays, resolved.QueriedInLastDays)
	if err != nil {
		return nil, resolved, err
	}
	rows := s.decorateRows([]MetricReportRow{*row})
	return &rows[0], resolvedWithDay(resolved, day), nil
}

func (s *Service) ListMetricEvidence(ctx context.Context, filters MetricReportFilters, metricName string) (MetricEvidenceResponse, error) {
	if strings.TrimSpace(metricName) == "" {
		return MetricEvidenceResponse{}, badRequest("metric_name is required")
	}

	resolved, day, err := s.resolveDay(ctx, filters)
	if err != nil {
		return MetricEvidenceResponse{}, err
	}
	rows, err := s.repo.ListMetricEvidence(ctx, resolved.Tenant, metricName, day, resolved.LookbackDays, resolved.SourceType)
	if err != nil {
		return MetricEvidenceResponse{}, err
	}
	return MetricEvidenceResponse{
		Data:    rows,
		Filters: resolvedWithDay(resolved, day),
	}, nil
}

func (s *Service) GetSummary(ctx context.Context, filters MetricReportFilters) (MetricSummaryResponse, error) {
	resolved, day, err := s.resolveDay(ctx, filters)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			resolved.Day = ""
			return MetricSummaryResponse{Data: MetricReportSummary{Tenant: resolved.Tenant}, Filters: resolved}, nil
		}
		return MetricSummaryResponse{}, err
	}

	rows, _, err := s.repo.ListMetricReports(ctx, resolved)
	if err != nil {
		return MetricSummaryResponse{}, err
	}
	rows = s.decorateRows(rows)
	rows = applyFilters(rows, resolved)

	summary := summarizeRows(rows, resolvedWithDay(resolved, day))
	return MetricSummaryResponse{Data: summary, Filters: resolvedWithDay(resolved, day)}, nil
}

func (s *Service) resolveDay(ctx context.Context, filters MetricReportFilters) (MetricReportFilters, time.Time, error) {
	if filters.LookbackDays <= 0 {
		filters.LookbackDays = s.defaults.LookbackDays
	}
	if filters.QueriedInLastDays <= 0 {
		filters.QueriedInLastDays = filters.LookbackDays
	}
	if filters.Limit <= 0 {
		filters.Limit = s.defaults.Limit
	}
	if filters.Limit > s.defaults.MaxLimit {
		filters.Limit = s.defaults.MaxLimit
	}
	if filters.Sort == "" {
		filters.Sort = "metric_name"
	}
	if filters.Order == "" {
		filters.Order = "asc"
	}
	if filters.Tenant == "" {
		filters.Tenant = s.defaults.Tenant
	}

	if strings.TrimSpace(filters.Day) != "" {
		day, err := parseDay(filters.Day)
		return filters, day, err
	}

	day, err := s.repo.LatestDay(ctx, filters.Tenant)
	if err != nil {
		return filters, time.Time{}, err
	}
	filters.Day = formatDay(day)
	return filters, day, nil
}

func (s *Service) decorateRows(rows []MetricReportRow) []MetricReportRow {
	out := make([]MetricReportRow, len(rows))
	for i, row := range rows {
		row.ObservedLabelKeys = normalizeLabels(row.ObservedLabelKeys)
		row.UsedLabels = normalizeLabels(row.UsedLabels)
		row.UnusedLabelKeys = unusedLabels(row.ObservedLabelKeys, row.UsedLabels)
		row.QueriedAt, row.QueriedAtTruncated = limitQueriedAt(row.QueriedAt, s.defaults.MaxQueriedAtEntries)
		row.Risk, row.Recommendation = recommendMetric(row)
		out[i] = row
	}
	return out
}

func recommendMetric(row MetricReportRow) (MetricRisk, MetricRecommendation) {
	switch {
	case row.UsedInAlerts:
		return RiskCritical, RecommendationKeep
	case row.QueryUsage.WasQueriedInLast30d:
		return RiskHigh, RecommendationKeep
	case row.UsedInRecordingRules:
		return RiskHigh, RecommendationKeepOrReviewAggregation
	case row.UsedInDashboards:
		return RiskMedium, RecommendationReviewDashboardUsage
	case row.UsedInQueryHistory:
		return RiskMedium, RecommendationReviewRecentQueryUsage
	case isProtectedMetric(row.MetricName):
		return RiskUnknown, RecommendationReview
	default:
		return RiskLow, RecommendationDropCandidate
	}
}

func isProtectedMetric(metricName string) bool {
	for _, pattern := range protectedMetricPatterns {
		if pattern.MatchString(metricName) {
			return true
		}
	}
	return false
}

func normalizeLabels(labels []string) []string {
	seen := make(map[string]struct{}, len(labels))
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

func limitQueriedAt(values []time.Time, limit int) ([]time.Time, bool) {
	if len(values) == 0 {
		return []time.Time{}, false
	}
	out := append([]time.Time(nil), values...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].After(out[j])
	})
	if limit <= 0 || len(out) <= limit {
		return out, false
	}
	return out[:limit], true
}

func unusedLabels(observed []string, used []string) []string {
	usedSet := make(map[string]struct{}, len(used))
	for _, label := range used {
		usedSet[label] = struct{}{}
	}
	out := make([]string, 0, len(observed))
	for _, label := range observed {
		if _, ok := usedSet[label]; !ok {
			out = append(out, label)
		}
	}
	return normalizeLabels(out)
}

func applyFilters(rows []MetricReportRow, filters MetricReportFilters) []MetricReportRow {
	out := make([]MetricReportRow, 0, len(rows))
	for _, row := range rows {
		if filters.Recommendation != "" && row.Recommendation != filters.Recommendation {
			continue
		}
		if filters.Risk != "" && row.Risk != filters.Risk {
			continue
		}
		if filters.UsedInAlerts != nil && row.UsedInAlerts != *filters.UsedInAlerts {
			continue
		}
		if filters.UsedInRecordingRules != nil && row.UsedInRecordingRules != *filters.UsedInRecordingRules {
			continue
		}
		if filters.UsedInDashboards != nil && row.UsedInDashboards != *filters.UsedInDashboards {
			continue
		}
		if filters.UsedInQueryHistory != nil && row.UsedInQueryHistory != *filters.UsedInQueryHistory {
			continue
		}
		if filters.UnusedOnly && (row.UsageEvidenceCount > 0 || row.QueryUsage.WasQueriedInLast30d) {
			continue
		}
		if filters.DropCandidatesOnly && row.Recommendation != RecommendationDropCandidate {
			continue
		}
		out = append(out, row)
	}
	return out
}

func sortRows(rows []MetricReportRow, field string, order string) {
	desc := strings.EqualFold(order, "desc")
	sort.SliceStable(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]
		var cmp int
		switch field {
		case "last_seen_at":
			cmp = compareTime(left.LastSeenAt, right.LastSeenAt)
		case "last_queried_at":
			cmp = compareOptionalTime(left.LastQueriedAt, right.LastQueriedAt)
		case "observation_count":
			cmp = compareInt64(left.ObservationCount, right.ObservationCount)
		case "query_history_count":
			cmp = compareInt64(left.QueryHistoryCount, right.QueryHistoryCount)
		case "usage_evidence_count":
			cmp = compareInt(left.UsageEvidenceCount, right.UsageEvidenceCount)
		case "risk":
			cmp = compareInt(riskRank(left.Risk), riskRank(right.Risk))
		case "recommendation":
			cmp = strings.Compare(string(left.Recommendation), string(right.Recommendation))
		default:
			if left.MetricName == right.MetricName {
				cmp = strings.Compare(left.Cluster, right.Cluster)
			} else {
				cmp = strings.Compare(left.MetricName, right.MetricName)
			}
		}
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareTime(left time.Time, right time.Time) int {
	switch {
	case left.Before(right):
		return -1
	case left.After(right):
		return 1
	default:
		return 0
	}
}

func compareOptionalTime(left *time.Time, right *time.Time) int {
	switch {
	case left == nil && right == nil:
		return 0
	case left == nil:
		return -1
	case right == nil:
		return 1
	default:
		return compareTime(*left, *right)
	}
}

func compareInt(left int, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareInt64(left int64, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func riskRank(risk MetricRisk) int {
	switch risk {
	case RiskCritical:
		return 5
	case RiskHigh:
		return 4
	case RiskMedium:
		return 3
	case RiskLow:
		return 2
	case RiskUnknown:
		return 1
	default:
		return 0
	}
}

func paginateRows(rows []MetricReportRow, limit int, offset int) []MetricReportRow {
	if offset >= len(rows) {
		return []MetricReportRow{}
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[offset:end]
}

func summarizeRows(rows []MetricReportRow, filters MetricReportFilters) MetricReportSummary {
	summary := MetricReportSummary{Day: filters.Day, Tenant: filters.Tenant, TotalMetrics: len(rows)}
	for _, row := range rows {
		if row.UsageEvidenceCount > 0 || row.QueryUsage.WasQueriedInLast30d {
			summary.UsedMetrics++
		} else {
			summary.UnusedMetrics++
		}
		if row.UsedInAlerts {
			summary.UsedInAlerts++
		}
		if row.UsedInRecordingRules {
			summary.UsedInRecordingRules++
		}
		if row.UsedInDashboards {
			summary.UsedInDashboards++
		}
		if row.UsedInQueryHistory {
			summary.UsedInQueryHistory++
		}
		switch row.Recommendation {
		case RecommendationKeep:
			summary.Keep++
		case RecommendationKeepOrReviewAggregation:
			summary.KeepOrReviewAggregation++
		case RecommendationReviewDashboardUsage:
			summary.ReviewDashboardUsage++
		case RecommendationReviewRecentQueryUsage:
			summary.ReviewRecentQueryUsage++
		case RecommendationDropCandidate:
			summary.DropCandidates++
		case RecommendationReview:
			summary.Review++
		}
		switch row.Risk {
		case RiskCritical:
			summary.CriticalRisk++
		case RiskHigh:
			summary.HighRisk++
		case RiskMedium:
			summary.MediumRisk++
		case RiskLow:
			summary.LowRisk++
		case RiskUnknown:
			summary.UnknownRisk++
		}
	}
	return summary
}

func parseDay(value string) (time.Time, error) {
	day, err := time.Parse(dayLayout, value)
	if err != nil {
		return time.Time{}, err
	}
	return day, nil
}

func formatDay(day time.Time) string {
	return day.UTC().Format(dayLayout)
}

func resolvedWithDay(filters MetricReportFilters, day time.Time) MetricReportFilters {
	filters.Day = formatDay(day)
	return filters
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parsePositiveInt(value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("expected positive integer")
	}
	return parsed, nil
}

func optionalBool(values url.Values, key string) (*bool, error) {
	value := strings.TrimSpace(values.Get(key))
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, badRequest(fmt.Sprintf("invalid %s, expected bool", key))
	}
	return &parsed, nil
}

func boolWithDefault(values url.Values, key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(values.Get(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, badRequest(fmt.Sprintf("invalid %s, expected bool", key))
	}
	return parsed, nil
}
