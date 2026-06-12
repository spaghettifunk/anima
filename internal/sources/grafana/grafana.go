package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return NewClientWithTimeout(baseURL, token, 30*time.Second)
}

func NewClientWithTimeout(baseURL, token string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		baseURL: strings.TrimSpace(baseURL),
		token:   strings.TrimSpace(token),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type DashboardDocument struct {
	UID       string
	Title     string
	Dashboard Dashboard
}

func (c *Client) FetchDashboards(ctx context.Context) ([]DashboardDocument, error) {
	searchURL, err := c.buildURL("/api/search", "type=dash-db")
	if err != nil {
		return nil, err
	}

	var dashboards []searchResult
	if err := c.getJSON(ctx, searchURL, &dashboards); err != nil {
		return nil, fmt.Errorf("fetch grafana dashboard search: %w", err)
	}

	out := make([]DashboardDocument, 0)
	for _, dashboard := range dashboards {
		uid := strings.TrimSpace(dashboard.UID)
		if uid == "" {
			continue
		}

		dashboardURL, err := c.buildURL("/api/dashboards/uid/"+url.PathEscape(uid), "")
		if err != nil {
			return nil, err
		}

		var detail dashboardResponse
		if err := c.getJSON(ctx, dashboardURL, &detail); err != nil {
			return nil, fmt.Errorf("fetch grafana dashboard %q: %w", uid, err)
		}

		title := firstNonEmpty(detail.Dashboard.Title, dashboard.Title, uid)
		out = append(out, DashboardDocument{
			UID:       uid,
			Title:     title,
			Dashboard: detail.Dashboard,
		})
	}

	return out, nil
}

func (c *Client) getJSON(ctx context.Context, requestURL string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("create grafana request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) buildURL(path string, rawQuery string) (string, error) {
	rawBaseURL := c.baseURL
	if rawBaseURL == "" {
		return "", fmt.Errorf("grafana base url is required")
	}
	if !strings.Contains(rawBaseURL, "://") {
		rawBaseURL = "https://" + rawBaseURL
	}

	parsed, err := url.Parse(rawBaseURL)
	if err != nil {
		return "", fmt.Errorf("parse grafana base url: %w", err)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("grafana base url must include a host")
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = rawQuery
	return parsed.String(), nil
}

type searchResult struct {
	UID   string `json:"uid"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

type dashboardResponse struct {
	Dashboard Dashboard `json:"dashboard"`
}

type Dashboard struct {
	Title  string  `json:"title"`
	Panels []Panel `json:"panels"`
}

type Panel struct {
	ID         any             `json:"id"`
	Title      string          `json:"title"`
	Datasource QueryDatasource `json:"datasource"`
	Targets    []Target        `json:"targets"`
	Panels     []Panel         `json:"panels"`
}

type Target struct {
	Datasource QueryDatasource `json:"datasource"`
	Expr       string          `json:"expr"`
	RefID      string          `json:"refId"`
}

type QueryHistoryRequest struct {
	DatasourceUID string
	From          time.Time
	To            time.Time
	Limit         int
	Page          int
	Sort          string
}

type QueryHistoryResponse struct {
	TotalCount int
	Items      []QueryHistoryItem
}

type QueryHistoryItem struct {
	UID           string
	DatasourceUID string
	CreatedBy     *int64
	CreatedAt     time.Time
	Comment       string
	Queries       []QueryHistoryQuery
	Starred       bool
}

type QueryHistoryQuery struct {
	Datasource QueryDatasource `json:"datasource"`
	EditorMode string          `json:"editorMode"`
	Expr       string          `json:"expr"`
	Key        string          `json:"key"`
	RefID      string          `json:"refId"`
}

type QueryDatasource struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

func (d *QueryDatasource) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var object struct {
		Type string `json:"type"`
		UID  string `json:"uid"`
	}
	if err := json.Unmarshal(data, &object); err == nil {
		d.Type = object.Type
		d.UID = object.UID
		return nil
	}

	var legacyName string
	if err := json.Unmarshal(data, &legacyName); err != nil {
		return err
	}
	d.UID = legacyName
	return nil
}

func (c *Client) FetchQueryHistory(ctx context.Context, req QueryHistoryRequest) (*QueryHistoryResponse, error) {
	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if strings.TrimSpace(req.Sort) == "" {
		req.Sort = "time-desc"
	}

	query := url.Values{}
	query.Set("datasourceUid", strings.TrimSpace(req.DatasourceUID))
	query.Set("sort", strings.TrimSpace(req.Sort))
	query.Set("to", fmt.Sprintf("%d", req.To.UTC().UnixMilli()))
	query.Set("from", fmt.Sprintf("%d", req.From.UTC().UnixMilli()))
	query.Set("limit", fmt.Sprintf("%d", req.Limit))
	query.Set("page", fmt.Sprintf("%d", req.Page))

	requestURL, err := c.buildURL("/api/query-history", query.Encode())
	if err != nil {
		return nil, err
	}

	var raw queryHistoryAPIResponse
	if err := c.getJSON(ctx, requestURL, &raw); err != nil {
		return nil, fmt.Errorf("fetch grafana query history page %d: %w", req.Page, err)
	}

	out := &QueryHistoryResponse{
		TotalCount: raw.Result.TotalCount,
		Items:      make([]QueryHistoryItem, 0, len(raw.Result.QueryHistory)),
	}
	for _, item := range raw.Result.QueryHistory {
		out.Items = append(out.Items, QueryHistoryItem{
			UID:           item.UID,
			DatasourceUID: item.DatasourceUID,
			CreatedBy:     item.CreatedBy.Int64Ptr(),
			CreatedAt:     unixSecondsOrMillis(item.CreatedAt),
			Comment:       item.Comment,
			Queries:       item.Queries,
			Starred:       item.Starred,
		})
	}

	return out, nil
}

func (c *Client) FetchAllQueryHistory(ctx context.Context, req QueryHistoryRequest, maxPages int) ([]QueryHistoryItem, error) {
	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if maxPages <= 0 {
		maxPages = 100
	}

	out := make([]QueryHistoryItem, 0)
	for pagesFetched := 0; pagesFetched < maxPages; pagesFetched++ {
		response, err := c.FetchQueryHistory(ctx, req)
		if err != nil {
			return nil, err
		}
		out = append(out, response.Items...)
		if len(response.Items) == 0 {
			return out, nil
		}
		if req.Page*req.Limit >= response.TotalCount {
			return out, nil
		}
		req.Page++
	}

	log.Warn().
		Int("max_pages", maxPages).
		Int("items", len(out)).
		Msg("stopped grafana query history pagination at max pages")
	return out, nil
}

type queryHistoryAPIResponse struct {
	Result struct {
		TotalCount   int                   `json:"totalCount"`
		QueryHistory []queryHistoryRawItem `json:"queryHistory"`
	} `json:"result"`
}

type queryHistoryRawItem struct {
	UID           string              `json:"uid"`
	DatasourceUID string              `json:"datasourceUid"`
	CreatedBy     nullableInt64       `json:"createdBy"`
	CreatedAt     int64               `json:"createdAt"`
	Comment       string              `json:"comment"`
	Queries       []QueryHistoryQuery `json:"queries"`
	Starred       bool                `json:"starred"`
}

type nullableInt64 struct {
	value int64
	valid bool
}

func (n *nullableInt64) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.valid = false
		n.value = 0
		return nil
	}
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	n.value = value
	n.valid = true
	return nil
}

func (n nullableInt64) Int64Ptr() *int64 {
	if !n.valid {
		return nil
	}
	value := n.value
	return &value
}

func unixSecondsOrMillis(value int64) time.Time {
	if value > 9999999999 || value < -9999999999 {
		return time.UnixMilli(value).UTC()
	}
	return time.Unix(value, 0).UTC()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
