package storage

import "time"

type QueryEventRecord struct {
	ID string

	ObservedAt time.Time

	Tenant string
	Source string

	Path   string
	Method string

	UsageKind string

	QueryHash string
	QueryRaw  *string

	RangeStart *time.Time
	RangeEnd   *time.Time
	QueryTime  *time.Time

	Step    *string
	Timeout *string

	StatusCode int
	DurationMs int64

	DashboardUID  *string
	PanelID       *string
	DatasourceUID *string

	RemoteAddr *string
	UserAgent  *string

	Error *string
}

type MetricQueryUsageRecord struct {
	ID string

	ObservedAt time.Time

	Tenant string
	Source string

	MetricName string

	QueryEventID string
	QueryHash    string

	DashboardUID  *string
	PanelID       *string
	DatasourceUID *string

	UsageKind string

	IsRecordingRule           bool
	ResolvedFromRecordingRule *string
}

type ResolvedMetricUsage struct {
	MetricName        string
	RecordingRuleName string
}
