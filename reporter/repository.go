package reporter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/spaghettifunk/anima/entityresolver"
	"github.com/spaghettifunk/anima/internal/storage/postgres"
)

var ErrNotFound = errors.New("not found")

type Repository interface {
	Health(ctx context.Context) error
	LatestDay(ctx context.Context, tenant string) (time.Time, error)
	ListMetricReports(ctx context.Context, filters MetricReportFilters) ([]MetricReportRow, int, error)
	GetMetricReport(ctx context.Context, tenant string, metricName string, day time.Time, lookbackDays int, queriedInLastDays int) (*MetricReportRow, error)
	ListMetricEvidence(ctx context.Context, tenant string, metricName string, day time.Time, lookbackDays int, sourceType string) ([]MetricEvidenceRow, error)
	ListEntities(ctx context.Context, entityType string) ([]entityresolver.Entity, error)
	GetEntity(ctx context.Context, id string) (*entityresolver.Entity, error)
	GetEntityRelationships(ctx context.Context, id string, depth int) ([]entityresolver.Relationship, error)
	GetServiceTopology(ctx context.Context, serviceID string, depth int) (*entityresolver.TopologyGraph, error)
	GetServiceHealth(ctx context.Context, serviceID string, window string, since time.Time) ([]entityresolver.ServiceHealthWindow, error)
	GetChangeEvents(ctx context.Context, targetID string, since time.Time) ([]entityresolver.ChangeEvent, error)
	Close()
}

type PostgresRepository struct {
	store *postgres.PostgresStorage
	graph entityresolver.EntityGraphStore
}

func NewPostgresRepository(store *postgres.PostgresStorage, graph ...entityresolver.EntityGraphStore) *PostgresRepository {
	repo := &PostgresRepository{store: store}
	if len(graph) > 0 {
		repo.graph = graph[0]
	}
	return repo
}

func (r *PostgresRepository) Initialize(ctx context.Context) error {
	if err := r.store.Initialize(); err != nil {
		return err
	}
	if r.graph != nil {
		return r.graph.Initialize(ctx)
	}
	return nil
}

func (r *PostgresRepository) Health(ctx context.Context) error {
	if err := r.store.Ping(ctx); err != nil {
		return err
	}
	if r.graph != nil {
		return r.graph.Health(ctx)
	}
	return nil
}

func (r *PostgresRepository) LatestDay(ctx context.Context, tenant string) (time.Time, error) {
	var day pgtype.Date
	err := r.store.QueryRow(ctx, `
select max(day)
from metric_observation_daily
where tenant = $1
`, tenant).Scan(&day)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, ErrNotFound
		}
		return time.Time{}, err
	}
	if !day.Valid {
		return time.Time{}, ErrNotFound
	}
	return day.Time, nil
}

func (r *PostgresRepository) ListMetricReports(ctx context.Context, filters MetricReportFilters) ([]MetricReportRow, int, error) {
	day, err := parseDay(filters.Day)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.queryMetricReports(ctx, metricReportQuery{
		Tenant:            filters.Tenant,
		Day:               day,
		LookbackDays:      filters.LookbackDays,
		QueriedInLastDays: filters.QueriedInLastDays,
		Cluster:           filters.Cluster,
		MetricName:        filters.MetricName,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, len(rows), nil
}

func (r *PostgresRepository) GetMetricReport(ctx context.Context, tenant string, metricName string, day time.Time, lookbackDays int, queriedInLastDays int) (*MetricReportRow, error) {
	rows, err := r.queryMetricReports(ctx, metricReportQuery{
		Tenant:            tenant,
		Day:               day,
		LookbackDays:      lookbackDays,
		QueriedInLastDays: queriedInLastDays,
		MetricName:        metricName,
		ExactMetric:       true,
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	return &rows[0], nil
}

func (r *PostgresRepository) ListMetricEvidence(ctx context.Context, tenant string, metricName string, day time.Time, lookbackDays int, sourceType string) ([]MetricEvidenceRow, error) {
	startDay := day.AddDate(0, 0, -lookbackDays+1)
	args := []any{tenant, metricName, startDay, day}
	sourceFilter := ""
	if strings.TrimSpace(sourceType) != "" {
		args = append(args, strings.TrimSpace(sourceType))
		sourceFilter = fmt.Sprintf("  and source_type = $%d\n", len(args))
	}
	rows, err := r.store.Query(ctx, fmt.Sprintf(`
select
    day,
    tenant,
    metric_name,
    source_type,
    source_ref,
    source_name,
    expression_hash,
    expression,
    used_labels,
    grouping_labels,
    matcher_labels,
    is_alert,
    is_recording_rule,
    is_dashboard,
    is_query_history,
    last_queried_at,
    query_history_count,
    queried_at,
    discovered_at
from metric_usage_evidence_daily
where tenant = $1
  and metric_name = $2
  and day between $3 and $4
%s
order by day desc, source_type asc, source_ref asc, source_name asc
`, sourceFilter), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]MetricEvidenceRow, 0)
	for rows.Next() {
		var row MetricEvidenceRow
		var rowDay time.Time
		var lastQueriedAt pgtype.Timestamptz
		if err := rows.Scan(
			&rowDay,
			&row.Tenant,
			&row.MetricName,
			&row.SourceType,
			&row.SourceRef,
			&row.SourceName,
			&row.ExpressionHash,
			&row.Expression,
			&row.UsedLabels,
			&row.GroupingLabels,
			&row.MatcherLabels,
			&row.IsAlert,
			&row.IsRecordingRule,
			&row.IsDashboard,
			&row.IsQueryHistory,
			&lastQueriedAt,
			&row.QueryHistoryCount,
			&row.QueriedAt,
			&row.DiscoveredAt,
		); err != nil {
			return nil, err
		}
		row.Day = formatDay(rowDay)
		row.UsedLabels = normalizeLabels(row.UsedLabels)
		row.GroupingLabels = normalizeLabels(row.GroupingLabels)
		row.MatcherLabels = normalizeLabels(row.MatcherLabels)
		row.LastQueriedAt = timestamptzPtr(lastQueriedAt)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (r *PostgresRepository) ListEntities(ctx context.Context, entityType string) ([]entityresolver.Entity, error) {
	if r.graph == nil {
		return nil, fmt.Errorf("entity graph store is not configured")
	}
	return r.graph.ListEntities(ctx, entityType)
}

func (r *PostgresRepository) GetEntity(ctx context.Context, id string) (*entityresolver.Entity, error) {
	if r.graph == nil {
		return nil, fmt.Errorf("entity graph store is not configured")
	}
	entity, err := r.graph.GetEntity(ctx, id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return entity, nil
}

func (r *PostgresRepository) GetEntityRelationships(ctx context.Context, id string, depth int) ([]entityresolver.Relationship, error) {
	if r.graph == nil {
		return nil, fmt.Errorf("entity graph store is not configured")
	}
	return r.graph.GetEntityRelationships(ctx, id, depth)
}

func (r *PostgresRepository) GetServiceTopology(ctx context.Context, serviceID string, depth int) (*entityresolver.TopologyGraph, error) {
	if r.graph == nil {
		return nil, fmt.Errorf("entity graph store is not configured")
	}
	return r.graph.GetServiceTopology(ctx, serviceID, depth)
}

func (r *PostgresRepository) GetServiceHealth(ctx context.Context, serviceID string, window string, since time.Time) ([]entityresolver.ServiceHealthWindow, error) {
	return r.store.GetServiceHealth(ctx, serviceID, window, since)
}

func (r *PostgresRepository) GetChangeEvents(ctx context.Context, targetID string, since time.Time) ([]entityresolver.ChangeEvent, error) {
	return r.store.GetChangeEvents(ctx, targetID, since)
}

func (r *PostgresRepository) Close() {
	if r.graph != nil {
		_ = r.graph.Shutdown(context.Background())
	}
	if r.store != nil {
		_ = r.store.Shutdown()
	}
}

type metricReportQuery struct {
	Tenant            string
	Day               time.Time
	LookbackDays      int
	QueriedInLastDays int
	Cluster           string
	MetricName        string
	ExactMetric       bool
}

func (r *PostgresRepository) queryMetricReports(ctx context.Context, query metricReportQuery) ([]MetricReportRow, error) {
	startDay := query.Day.AddDate(0, 0, -query.LookbackDays+1)
	if query.QueriedInLastDays <= 0 {
		query.QueriedInLastDays = query.LookbackDays
	}
	queryHistoryStartDay := query.Day.AddDate(0, 0, -query.QueriedInLastDays+1)
	args := []any{query.Tenant, startDay, query.Day, queryHistoryStartDay}
	where := []string{"o.tenant = $1", "o.day between $2 and $3"}
	if strings.TrimSpace(query.Cluster) != "" {
		args = append(args, query.Cluster)
		where = append(where, fmt.Sprintf("o.cluster = $%d", len(args)))
	}
	if strings.TrimSpace(query.MetricName) != "" {
		args = append(args, query.MetricName)
		if query.ExactMetric {
			where = append(where, fmt.Sprintf("o.metric_name = $%d", len(args)))
		} else {
			where = append(where, fmt.Sprintf("o.metric_name ilike '%%' || $%d || '%%'", len(args)))
		}
	}
	whereSQL := strings.Join(where, "\n      and ")

	sql := fmt.Sprintf(`
with observation as (
    select
        o.tenant,
        o.cluster,
        o.metric_name,
        coalesce(max(nullif(o.metric_type, '')), '') as metric_type,
        coalesce(max(nullif(o.source, '')), '') as source,
        min(o.first_seen_at) as first_seen_at,
        max(o.last_seen_at) as last_seen_at,
        count(distinct o.day)::int as observed_days,
        coalesce(sum(o.observation_count), 0)::bigint as observation_count
    from metric_observation_daily o
    where %s
    group by o.tenant, o.cluster, o.metric_name
),
observation_labels as (
    select
        o.tenant,
        o.cluster,
        o.metric_name,
        array_agg(distinct label_key order by label_key) as observed_label_keys
    from metric_observation_daily o
    cross join lateral unnest(o.label_keys) as labels(label_key)
    where %s
      and label_key is not null
      and label_key <> ''
    group by o.tenant, o.cluster, o.metric_name
),
evidence as (
    select
        tenant,
        metric_name,
        bool_or(is_alert) as used_in_alerts,
        bool_or(is_recording_rule) as used_in_recording_rules,
        bool_or(is_dashboard) as used_in_dashboards,
        bool_or(is_query_history and day between $4 and $3) as used_in_query_history,
        max(last_queried_at) filter (where is_query_history and day between $4 and $3) as last_queried_at,
        coalesce(sum(query_history_count) filter (where is_query_history and day between $4 and $3), 0)::bigint as query_history_count,
        count(*)::int as usage_evidence_count
    from metric_usage_evidence_daily
    where tenant = $1
      and day between $2 and $3
    group by tenant, metric_name
),
query_history_times as (
    select
        e.tenant,
        e.metric_name,
        array_agg(distinct q order by q desc) as queried_at
    from metric_usage_evidence_daily e
    cross join lateral unnest(e.queried_at) as times(q)
    where e.tenant = $1
      and e.day between $4 and $3
      and e.is_query_history = true
      and q is not null
    group by e.tenant, e.metric_name
),
query_usage as (
    select
        tenant,
        metric_name,
        max(observed_at) as last_queried_at,
        count(*) filter (
            where observed_at >= now() - interval '24 hours'
        )::bigint as query_count_24h,
        count(*) filter (
            where observed_at >= now() - interval '7 days'
        )::bigint as query_count_7d,
        count(*) filter (
            where observed_at >= now() - interval '30 days'
        )::bigint as query_count_30d,
        count(distinct query_hash) filter (
            where observed_at >= now() - interval '30 days'
              and query_hash <> ''
        )::bigint as distinct_query_count_30d,
        count(distinct dashboard_uid) filter (
            where observed_at >= now() - interval '30 days'
              and dashboard_uid is not null
              and dashboard_uid <> ''
        )::bigint as distinct_dashboard_count_30d,
        count(*) filter (
            where observed_at >= now() - interval '30 days'
              and usage_kind in (
                  'direct_query',
                  'range_query',
                  'dashboard_query',
                  'dashboard_range_query'
              )
        )::bigint as direct_query_count_30d,
        count(*) filter (
            where observed_at >= now() - interval '30 days'
              and usage_kind = 'recording_rule_query'
        )::bigint as indirect_query_count_30d
    from metric_query_usage
    where tenant = $1
    group by tenant, metric_name
),
evidence_labels as (
    select
        e.tenant,
        e.metric_name,
        array_agg(distinct used_label order by used_label) as used_labels
    from metric_usage_evidence_daily e
    cross join lateral unnest(e.used_labels) as labels(used_label)
    where e.tenant = $1
      and e.day between $2 and $3
      and used_label is not null
      and used_label <> ''
    group by e.tenant, e.metric_name
)
select
    $3::date as day,
    o.tenant,
    o.cluster,
    o.metric_name,
    o.metric_type,
    o.source,
    o.first_seen_at,
    o.last_seen_at,
    o.observed_days,
    o.observation_count,
    coalesce(ol.observed_label_keys, '{}'::text[]) as observed_label_keys,
    coalesce(el.used_labels, '{}'::text[]) as used_labels,
    coalesce(e.used_in_alerts, false) as used_in_alerts,
    coalesce(e.used_in_recording_rules, false) as used_in_recording_rules,
    coalesce(e.used_in_dashboards, false) as used_in_dashboards,
    coalesce(e.used_in_query_history, false) as used_in_query_history,
    e.last_queried_at,
    coalesce(e.query_history_count, 0)::bigint as query_history_count,
    coalesce(qht.queried_at, '{}'::timestamptz[]) as queried_at,
    coalesce(e.usage_evidence_count, 0)::int as usage_evidence_count,
    qu.last_queried_at as query_usage_last_queried_at,
    coalesce(qu.query_count_24h, 0)::bigint as query_usage_count_24h,
    coalesce(qu.query_count_7d, 0)::bigint as query_usage_count_7d,
    coalesce(qu.query_count_30d, 0)::bigint as query_usage_count_30d,
    coalesce(qu.distinct_query_count_30d, 0)::bigint as query_usage_distinct_query_count_30d,
    coalesce(qu.distinct_dashboard_count_30d, 0)::bigint as query_usage_distinct_dashboard_count_30d,
    coalesce(qu.direct_query_count_30d, 0)::bigint as query_usage_direct_query_count_30d,
    coalesce(qu.indirect_query_count_30d, 0)::bigint as query_usage_indirect_query_count_30d,
    coalesce(qu.query_count_30d, 0) > 0 as query_usage_was_queried_in_last_30d
from observation o
left join observation_labels ol
  on ol.tenant = o.tenant
 and ol.cluster = o.cluster
 and ol.metric_name = o.metric_name
left join evidence e
  on e.tenant = o.tenant
 and e.metric_name = o.metric_name
left join evidence_labels el
  on el.tenant = o.tenant
 and el.metric_name = o.metric_name
left join query_history_times qht
  on qht.tenant = o.tenant
 and qht.metric_name = o.metric_name
left join query_usage qu
  on qu.tenant = o.tenant
 and qu.metric_name = o.metric_name
order by o.metric_name asc, o.cluster asc
`, whereSQL, whereSQL)

	rows, err := r.store.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]MetricReportRow, 0)
	for rows.Next() {
		var row MetricReportRow
		var day time.Time
		var lastQueriedAt pgtype.Timestamptz
		var queryUsageLastQueriedAt pgtype.Timestamptz
		if err := rows.Scan(
			&day,
			&row.Tenant,
			&row.Cluster,
			&row.MetricName,
			&row.MetricType,
			&row.Source,
			&row.FirstSeenAt,
			&row.LastSeenAt,
			&row.ObservedDays,
			&row.ObservationCount,
			&row.ObservedLabelKeys,
			&row.UsedLabels,
			&row.UsedInAlerts,
			&row.UsedInRecordingRules,
			&row.UsedInDashboards,
			&row.UsedInQueryHistory,
			&lastQueriedAt,
			&row.QueryHistoryCount,
			&row.QueriedAt,
			&row.UsageEvidenceCount,
			&queryUsageLastQueriedAt,
			&row.QueryUsage.QueryCount24h,
			&row.QueryUsage.QueryCount7d,
			&row.QueryUsage.QueryCount30d,
			&row.QueryUsage.DistinctQueryCount30d,
			&row.QueryUsage.DistinctDashboardCount30d,
			&row.QueryUsage.DirectQueryCount30d,
			&row.QueryUsage.IndirectQueryCount30d,
			&row.QueryUsage.WasQueriedInLast30d,
		); err != nil {
			return nil, err
		}
		row.Day = formatDay(day)
		row.LastQueriedAt = timestamptzPtr(lastQueriedAt)
		row.QueryUsage.LastQueriedAt = timestamptzPtr(queryUsageLastQueriedAt)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func timestamptzPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}
