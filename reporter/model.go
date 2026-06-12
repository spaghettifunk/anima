package reporter

import "time"

type MetricRecommendation string

const (
	RecommendationKeep                    MetricRecommendation = "keep"
	RecommendationKeepOrReviewAggregation MetricRecommendation = "keep_or_review_aggregation"
	RecommendationReviewDashboardUsage    MetricRecommendation = "review_dashboard_usage"
	RecommendationReviewRecentQueryUsage  MetricRecommendation = "review_recent_query_usage"
	RecommendationDropCandidate           MetricRecommendation = "drop_candidate"
	RecommendationReview                  MetricRecommendation = "review"
)

type MetricRisk string

const (
	RiskCritical MetricRisk = "critical"
	RiskHigh     MetricRisk = "high"
	RiskMedium   MetricRisk = "medium"
	RiskLow      MetricRisk = "low"
	RiskUnknown  MetricRisk = "unknown"
)

type MetricReportRow struct {
	Day                  string                  `json:"day"`
	Tenant               string                  `json:"tenant"`
	Cluster              string                  `json:"cluster"`
	MetricName           string                  `json:"metric_name"`
	MetricType           string                  `json:"metric_type"`
	Source               string                  `json:"source"`
	FirstSeenAt          time.Time               `json:"first_seen_at"`
	LastSeenAt           time.Time               `json:"last_seen_at"`
	ObservedDays         int                     `json:"observed_days"`
	ObservationCount     int64                   `json:"observation_count"`
	ObservedLabelKeys    []string                `json:"observed_label_keys"`
	UsedLabels           []string                `json:"used_labels"`
	UnusedLabelKeys      []string                `json:"unused_label_keys"`
	UsedInAlerts         bool                    `json:"used_in_alerts"`
	UsedInRecordingRules bool                    `json:"used_in_recording_rules"`
	UsedInDashboards     bool                    `json:"used_in_dashboards"`
	UsedInQueryHistory   bool                    `json:"used_in_query_history"`
	LastQueriedAt        *time.Time              `json:"last_queried_at,omitempty"`
	QueryHistoryCount    int64                   `json:"query_history_count"`
	QueriedAt            []time.Time             `json:"queried_at"`
	QueriedAtTruncated   bool                    `json:"queried_at_truncated"`
	QueryUsage           MetricQueryUsageSummary `json:"query_usage"`
	UsageEvidenceCount   int                     `json:"usage_evidence_count"`
	Risk                 MetricRisk              `json:"risk"`
	Recommendation       MetricRecommendation    `json:"recommendation"`
}

type MetricQueryUsageSummary struct {
	LastQueriedAt             *time.Time `json:"last_queried_at,omitempty"`
	QueryCount24h             int64      `json:"query_count_24h"`
	QueryCount7d              int64      `json:"query_count_7d"`
	QueryCount30d             int64      `json:"query_count_30d"`
	DistinctQueryCount30d     int64      `json:"distinct_query_count_30d"`
	DistinctDashboardCount30d int64      `json:"distinct_dashboard_count_30d"`
	DirectQueryCount30d       int64      `json:"direct_query_count_30d"`
	IndirectQueryCount30d     int64      `json:"indirect_query_count_30d"`
	WasQueriedInLast30d       bool       `json:"was_queried_in_last_30d"`
}

type MetricEvidenceRow struct {
	Day               string      `json:"day"`
	Tenant            string      `json:"tenant"`
	MetricName        string      `json:"metric_name"`
	SourceType        string      `json:"source_type"`
	SourceRef         string      `json:"source_ref"`
	SourceName        string      `json:"source_name"`
	ExpressionHash    string      `json:"expression_hash"`
	Expression        string      `json:"expression"`
	UsedLabels        []string    `json:"used_labels"`
	GroupingLabels    []string    `json:"grouping_labels"`
	MatcherLabels     []string    `json:"matcher_labels"`
	IsAlert           bool        `json:"is_alert"`
	IsRecordingRule   bool        `json:"is_recording_rule"`
	IsDashboard       bool        `json:"is_dashboard"`
	IsQueryHistory    bool        `json:"is_query_history"`
	LastQueriedAt     *time.Time  `json:"last_queried_at,omitempty"`
	QueryHistoryCount int64       `json:"query_history_count"`
	QueriedAt         []time.Time `json:"queried_at,omitempty"`
	DiscoveredAt      time.Time   `json:"discovered_at"`
}

type MetricReportResponse struct {
	Data       []MetricReportRow   `json:"data"`
	Pagination Pagination          `json:"pagination"`
	Filters    MetricReportFilters `json:"filters"`
}

type MetricEvidenceResponse struct {
	Data    []MetricEvidenceRow `json:"data"`
	Filters MetricReportFilters `json:"filters"`
}

type MetricSummaryResponse struct {
	Data    MetricReportSummary `json:"data"`
	Filters MetricReportFilters `json:"filters"`
}

type Pagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

type MetricReportFilters struct {
	Tenant               string               `json:"tenant,omitempty"`
	Day                  string               `json:"day,omitempty"`
	LookbackDays         int                  `json:"lookback_days,omitempty"`
	Cluster              string               `json:"cluster,omitempty"`
	MetricName           string               `json:"metric_name,omitempty"`
	Recommendation       MetricRecommendation `json:"recommendation,omitempty"`
	Risk                 MetricRisk           `json:"risk,omitempty"`
	UsedInAlerts         *bool                `json:"used_in_alerts,omitempty"`
	UsedInRecordingRules *bool                `json:"used_in_recording_rules,omitempty"`
	UsedInDashboards     *bool                `json:"used_in_dashboards,omitempty"`
	UsedInQueryHistory   *bool                `json:"used_in_query_history,omitempty"`
	QueriedInLastDays    int                  `json:"queried_in_last_days,omitempty"`
	SourceType           string               `json:"source_type,omitempty"`
	UnusedOnly           bool                 `json:"unused_only,omitempty"`
	DropCandidatesOnly   bool                 `json:"drop_candidates_only,omitempty"`
	Sort                 string               `json:"sort,omitempty"`
	Order                string               `json:"order,omitempty"`
	Limit                int                  `json:"limit,omitempty"`
	Offset               int                  `json:"offset,omitempty"`
}

type MetricReportSummary struct {
	Day                     string `json:"day"`
	Tenant                  string `json:"tenant"`
	TotalMetrics            int    `json:"total_metrics"`
	UsedMetrics             int    `json:"used_metrics"`
	UnusedMetrics           int    `json:"unused_metrics"`
	UsedInAlerts            int    `json:"used_in_alerts"`
	UsedInRecordingRules    int    `json:"used_in_recording_rules"`
	UsedInDashboards        int    `json:"used_in_dashboards"`
	UsedInQueryHistory      int    `json:"used_in_query_history"`
	Keep                    int    `json:"keep"`
	KeepOrReviewAggregation int    `json:"keep_or_review_aggregation"`
	ReviewDashboardUsage    int    `json:"review_dashboard_usage"`
	ReviewRecentQueryUsage  int    `json:"review_recent_query_usage"`
	DropCandidates          int    `json:"drop_candidates"`
	Review                  int    `json:"review"`
	CriticalRisk            int    `json:"critical_risk"`
	HighRisk                int    `json:"high_risk"`
	MediumRisk              int    `json:"medium_risk"`
	LowRisk                 int    `json:"low_risk"`
	UnknownRisk             int    `json:"unknown_risk"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
