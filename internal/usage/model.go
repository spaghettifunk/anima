package usage

import "time"

type UsageSourceType string

const (
	UsageSourceThanosRule       UsageSourceType = "thanos_rule"
	UsageSourceThanosAlert      UsageSourceType = "thanos_alert"
	UsageSourceGrafanaDashboard UsageSourceType = "grafana_dashboard"
	UsageSourceQueryHistory     UsageSourceType = "query_history"
)

type PromQLExpression struct {
	Tenant         string
	Day            time.Time
	SourceType     UsageSourceType
	SourceRef      string
	SourceName     string
	Expression     string
	ExpressionHash string
	DiscoveredAt   time.Time

	QueriedAt       *time.Time
	QueryHistoryUID string
	QueryRefID      string
	DatasourceUID   string
	CreatedBy       *int64
	Comment         string
	Starred         bool
	QueryKey        string
	EditorMode      string
}

type MetricUsageEvidence struct {
	Tenant            string
	Day               time.Time
	MetricName        string
	SourceType        UsageSourceType
	SourceRef         string
	SourceName        string
	Expression        string
	ExpressionHash    string
	UsedLabels        []string
	GroupingLabels    []string
	MatcherLabels     []string
	IsAlert           bool
	IsRecordingRule   bool
	IsDashboard       bool
	IsQueryHistory    bool
	LastQueriedAt     *time.Time
	QueryHistoryCount int64
	QueriedAt         []time.Time
	DiscoveredAt      time.Time
}

type ParseError struct {
	Expression PromQLExpression
	Err        error
}

type QueryHistoryRow struct {
	Tenant          string
	Day             time.Time
	DatasourceUID   string
	QueryHistoryUID string
	QueryRefID      string
	ExpressionHash  string
	Expression      string
	CreatedBy       *int64
	CreatedAt       time.Time
	Comment         string
	Starred         bool
	QueryKey        string
	EditorMode      string
	DiscoveredAt    time.Time
}
