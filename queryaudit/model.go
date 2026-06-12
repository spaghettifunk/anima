package queryaudit

import "time"

type QueryAuditEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	RawQuery  string    `json:"raw_query,omitempty"`

	Query     string `json:"query,omitempty"`
	QueryHash string `json:"query_hash,omitempty"`

	Start   string `json:"start,omitempty"`
	End     string `json:"end,omitempty"`
	Time    string `json:"time,omitempty"`
	Step    string `json:"step,omitempty"`
	Timeout string `json:"timeout,omitempty"`

	RemoteAddr string `json:"remote_addr,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`

	OrgID         string `json:"org_id,omitempty"`
	DashboardUID  string `json:"dashboard_uid,omitempty"`
	PanelID       string `json:"panel_id,omitempty"`
	DatasourceUID string `json:"datasource_uid,omitempty"`

	ExtractedMetric []string `json:"extracted_metrics,omitempty"`

	StatusCode int   `json:"status_code,omitempty"`
	DurationMs int64 `json:"duration_ms,omitempty"`

	Error string `json:"error,omitempty"`
}
