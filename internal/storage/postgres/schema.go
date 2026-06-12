package postgres

const schemaSQL = `
create table if not exists metric_observation_daily (
    day date not null,
    tenant text not null,
    cluster text not null,
    metric_name text not null,
    metric_type text not null default '',
    source text not null default '',
    first_seen_at timestamptz not null,
    last_seen_at timestamptz not null,
    observation_count bigint not null default 0,
    label_keys text[] not null default '{}',
    resource_label_keys text[] not null default '{}',
    updated_at timestamptz not null default now(),
    primary key (day, tenant, cluster, metric_name)
);

create index if not exists idx_metric_observation_daily_metric_name
on metric_observation_daily(metric_name);

create index if not exists idx_metric_observation_daily_tenant_day
on metric_observation_daily(tenant, day);

create index if not exists idx_metric_observation_daily_cluster_day
on metric_observation_daily(cluster, day);

create index if not exists idx_metric_observation_daily_last_seen
on metric_observation_daily(last_seen_at);

create table if not exists metric_usage_expression_daily (
    day date not null,
    tenant text not null,
    source_type text not null,
    source_ref text not null,
    source_name text not null default '',
    expression_hash text not null,
    expression text not null,
    discovered_at timestamptz not null default now(),
    primary key (day, tenant, source_type, source_ref, expression_hash)
);

create table if not exists metric_usage_evidence_daily (
    day date not null,
    tenant text not null,
    metric_name text not null,
    source_type text not null,
    source_ref text not null,
    source_name text not null default '',
    expression_hash text not null,
    expression text not null,
    used_labels text[] not null default '{}',
    grouping_labels text[] not null default '{}',
    matcher_labels text[] not null default '{}',
    is_alert boolean not null default false,
    is_recording_rule boolean not null default false,
    is_dashboard boolean not null default false,
    is_query_history boolean not null default false,
    last_queried_at timestamptz,
    query_history_count bigint not null default 0,
    queried_at timestamptz[] not null default '{}',
    discovered_at timestamptz not null default now(),
    primary key (day, tenant, metric_name, source_type, source_ref, expression_hash)
);

create index if not exists idx_metric_usage_evidence_daily_metric
on metric_usage_evidence_daily(tenant, day, metric_name);

create index if not exists idx_metric_usage_evidence_daily_source
on metric_usage_evidence_daily(tenant, day, source_type);

create index if not exists idx_metric_usage_evidence_daily_query_history
on metric_usage_evidence_daily(tenant, day, is_query_history, metric_name);

create index if not exists idx_metric_usage_evidence_daily_last_queried
on metric_usage_evidence_daily(tenant, last_queried_at);

create index if not exists idx_metric_usage_expression_daily_source
on metric_usage_expression_daily(tenant, day, source_type);

create table if not exists query_history_daily (
    day date not null,
    tenant text not null,
    datasource_uid text not null,
    query_history_uid text not null,
    query_ref_id text not null default '',
    expression_hash text not null,
    expression text not null,
    created_by bigint,
    created_at timestamptz not null,
    comment text not null default '',
    starred boolean not null default false,
    query_key text not null default '',
    editor_mode text not null default '',
    discovered_at timestamptz not null default now(),
    primary key (day, tenant, datasource_uid, query_history_uid, query_ref_id, expression_hash)
);

create index if not exists idx_query_history_daily_tenant_day
on query_history_daily(tenant, day);

create index if not exists idx_query_history_daily_created_at
on query_history_daily(tenant, created_at);

create index if not exists idx_query_history_daily_expression_hash
on query_history_daily(expression_hash);

create table if not exists query_events (
    id uuid primary key,
    observed_at timestamptz not null,
    tenant text not null default '',
    source text not null default 'thanos',
    path text not null default '',
    method text not null default '',
    usage_kind text not null default 'unknown',
    query_hash text not null default '',
    query_raw text null,
    range_start timestamptz null,
    range_end timestamptz null,
    query_time timestamptz null,
    step text null,
    timeout text null,
    status_code integer not null default 0,
    duration_ms bigint not null default 0,
    dashboard_uid text null,
    panel_id text null,
    datasource_uid text null,
    remote_addr text null,
    user_agent text null,
    error text null,
    inserted_at timestamptz not null default now()
);

create index if not exists idx_query_events_observed_at
on query_events(observed_at desc);

create index if not exists idx_query_events_tenant_observed_at
on query_events(tenant, observed_at desc);

create index if not exists idx_query_events_query_hash
on query_events(query_hash);

create index if not exists idx_query_events_dashboard_uid
on query_events(dashboard_uid)
where dashboard_uid is not null;

create table if not exists metric_query_usage (
    id uuid primary key,
    observed_at timestamptz not null,
    tenant text not null default '',
    source text not null default 'thanos',
    metric_name text not null,
    query_event_id uuid null references query_events(id) on delete cascade,
    query_hash text not null default '',
    dashboard_uid text null,
    panel_id text null,
    datasource_uid text null,
    usage_kind text not null default 'unknown',
    is_recording_rule boolean not null default false,
    resolved_from_recording_rule text null,
    inserted_at timestamptz not null default now()
);

create index if not exists idx_metric_query_usage_metric_observed
on metric_query_usage(metric_name, observed_at desc);

create index if not exists idx_metric_query_usage_tenant_metric_observed
on metric_query_usage(tenant, metric_name, observed_at desc);

create index if not exists idx_metric_query_usage_query_hash
on metric_query_usage(query_hash);

create index if not exists idx_metric_query_usage_dashboard_uid
on metric_query_usage(dashboard_uid)
where dashboard_uid is not null;

create index if not exists idx_metric_query_usage_usage_kind
on metric_query_usage(usage_kind);

create table if not exists service_health_windows (
    service_id text not null,
    window text not null,
    window_start timestamptz not null,
    window_end timestamptz not null,
    request_count bigint not null default 0,
    error_count bigint not null default 0,
    error_rate double precision not null default 0,
    duration_p50_ms double precision not null default 0,
    duration_p95_ms double precision not null default 0,
    duration_p99_ms double precision not null default 0,
    log_error_count bigint not null default 0,
    span_error_count bigint not null default 0,
    source text not null default '',
    updated_at timestamptz not null default now(),
    primary key (service_id, window, window_start)
);

create index if not exists idx_service_health_windows_service_window_time
on service_health_windows(service_id, window, window_start desc);

create table if not exists change_events (
    id text primary key,
    type text not null,
    target_id text not null,
    target_type text not null,
    timestamp timestamptz not null,
    source text not null default '',
    version text not null default '',
    revision text not null default '',
    attributes jsonb not null default '{}'::jsonb,
    inserted_at timestamptz not null default now()
);

create index if not exists idx_change_events_target_time
on change_events(target_id, timestamp desc);
`

const upsertSQL = `
insert into metric_observation_daily (
    day,
    tenant,
    cluster,
    metric_name,
    metric_type,
    source,
    first_seen_at,
    last_seen_at,
    observation_count,
    label_keys,
    resource_label_keys,
    updated_at
) values (
    $1, $2, $3, $4, $5, $6, $7, $7, 1, $8, $9, now()
)
on conflict (day, tenant, cluster, metric_name) do update set
    metric_type = case
        when excluded.metric_type <> '' then excluded.metric_type
        else metric_observation_daily.metric_type
    end,
    source = case
        when excluded.source <> '' then excluded.source
        else metric_observation_daily.source
    end,
    first_seen_at = least(metric_observation_daily.first_seen_at, excluded.first_seen_at),
    last_seen_at = greatest(metric_observation_daily.last_seen_at, excluded.last_seen_at),
    observation_count = metric_observation_daily.observation_count + 1,
    label_keys = (
        select array(
            select distinct unnest(metric_observation_daily.label_keys || excluded.label_keys)
            order by 1
        )
    ),
    resource_label_keys = (
        select array(
            select distinct unnest(metric_observation_daily.resource_label_keys || excluded.resource_label_keys)
            order by 1
        )
    ),
    updated_at = now();
`

const upsertUsageExpressionSQL = `
insert into metric_usage_expression_daily (
    day,
    tenant,
    source_type,
    source_ref,
    source_name,
    expression_hash,
    expression,
    discovered_at
) values (
    $1, $2, $3, $4, $5, $6, $7, $8
)
on conflict (day, tenant, source_type, source_ref, expression_hash) do update set
    source_name = excluded.source_name,
    expression = excluded.expression,
    discovered_at = excluded.discovered_at;
`

const upsertUsageEvidenceSQL = `
insert into metric_usage_evidence_daily (
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
) values (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
)
on conflict (day, tenant, metric_name, source_type, source_ref, expression_hash) do update set
    source_name = excluded.source_name,
    expression = excluded.expression,
    used_labels = excluded.used_labels,
    grouping_labels = excluded.grouping_labels,
    matcher_labels = excluded.matcher_labels,
    is_alert = excluded.is_alert,
    is_recording_rule = excluded.is_recording_rule,
    is_dashboard = excluded.is_dashboard,
    is_query_history = excluded.is_query_history,
    last_queried_at = case
        when metric_usage_evidence_daily.last_queried_at is null then excluded.last_queried_at
        when excluded.last_queried_at is null then metric_usage_evidence_daily.last_queried_at
        else greatest(metric_usage_evidence_daily.last_queried_at, excluded.last_queried_at)
    end,
    query_history_count = greatest(metric_usage_evidence_daily.query_history_count, excluded.query_history_count),
    queried_at = (
        select coalesce(array_agg(distinct q order by q), '{}'::timestamptz[])
        from unnest(metric_usage_evidence_daily.queried_at || excluded.queried_at) as times(q)
    ),
    discovered_at = excluded.discovered_at;
`

const upsertQueryHistorySQL = `
insert into query_history_daily (
    day,
    tenant,
    datasource_uid,
    query_history_uid,
    query_ref_id,
    expression_hash,
    expression,
    created_by,
    created_at,
    comment,
    starred,
    query_key,
    editor_mode,
    discovered_at
) values (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
on conflict (day, tenant, datasource_uid, query_history_uid, query_ref_id, expression_hash) do update set
    expression = excluded.expression,
    created_by = excluded.created_by,
    created_at = excluded.created_at,
    comment = excluded.comment,
    starred = excluded.starred,
    query_key = excluded.query_key,
    editor_mode = excluded.editor_mode,
    discovered_at = excluded.discovered_at;
`

const insertQueryEventSQL = `
insert into query_events (
    id,
    observed_at,
    tenant,
    source,
    path,
    method,
    usage_kind,
    query_hash,
    query_raw,
    range_start,
    range_end,
    query_time,
    step,
    timeout,
    status_code,
    duration_ms,
    dashboard_uid,
    panel_id,
    datasource_uid,
    remote_addr,
    user_agent,
    error
) values (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
    $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
);
`

const insertMetricQueryUsageSQL = `
insert into metric_query_usage (
    id,
    observed_at,
    tenant,
    source,
    metric_name,
    query_event_id,
    query_hash,
    dashboard_uid,
    panel_id,
    datasource_uid,
    usage_kind,
    is_recording_rule,
    resolved_from_recording_rule
) values (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
);
`

const resolveRecordingRuleSQL = `
select distinct
    metric_name,
    coalesce(nullif(source_name, ''), $2) as source_name
from metric_usage_evidence_daily
where tenant = $1
  and source_type = 'thanos_rule'
  and source_name = $2
  and metric_name <> $2
order by metric_name;
`
