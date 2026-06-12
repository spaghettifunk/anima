package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spaghettifunk/anima/internal/config"
	"github.com/spaghettifunk/anima/internal/storage"
	"github.com/spaghettifunk/anima/internal/usage"
)

type PostgresStorage struct {
	dsn  string
	pool *pgxpool.Pool
}

func NewFromConfig(cfg config.Config) *PostgresStorage {
	sslMode := "disable"
	if cfg.Store.TLS {
		sslMode = "require"
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Store.Username,
		cfg.Store.Password,
		cfg.Store.Address,
		cfg.Store.Port,
		cfg.Store.DBName,
		sslMode,
	)
	return New(dsn)
}

func New(dsn string) *PostgresStorage {
	return &PostgresStorage{dsn: dsn}
}

func (p *PostgresStorage) Ping(ctx context.Context) error {
	if err := p.requireInitialized(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return p.pool.Ping(ctx)
}

func (p *PostgresStorage) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if err := p.requireInitialized(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return p.pool.Query(ctx, sql, args...)
}

func (p *PostgresStorage) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if err := p.requireInitialized(); err != nil {
		return errorRow{err: err}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return p.pool.QueryRow(ctx, sql, args...)
}

func (p *PostgresStorage) Initialize() error {
	if strings.TrimSpace(p.dsn) == "" {
		return fmt.Errorf("postgres DSN is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, p.dsn)
	if err != nil {
		return fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("ping postgres: %w", err)
	}

	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		pool.Close()
		return fmt.Errorf("run postgres schema migration: %w", err)
	}

	p.pool = pool
	return nil
}

func (p *PostgresStorage) Insert(rows []storage.ObservationRow) error {
	if len(rows) == 0 {
		return nil
	}
	if p.pool == nil {
		return fmt.Errorf("postgres storage is not initialized")
	}

	ctx := context.Background()
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin postgres transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	for _, row := range rows {
		// TODO: optionally deduplicate by Kafka topic/partition/offset or observation hash
		// if exact observation_count semantics become important.
		if _, err := tx.Exec(ctx, upsertSQL,
			row.Day,
			row.Tenant,
			row.Cluster,
			row.MetricName,
			row.MetricType,
			row.Source,
			row.ObservedAt,
			row.LabelKeys,
			row.ResourceLabelKeys,
		); err != nil {
			return fmt.Errorf("upsert metric observation daily row: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit postgres transaction: %w", err)
	}

	return nil
}

func (p *PostgresStorage) InsertQueryEventWithUsage(ctx context.Context, event storage.QueryEventRecord, usageRows []storage.MetricQueryUsageRecord) error {
	if err := p.requireInitialized(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin postgres transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, insertQueryEventSQL,
		event.ID,
		event.ObservedAt,
		event.Tenant,
		event.Source,
		event.Path,
		event.Method,
		event.UsageKind,
		event.QueryHash,
		nullableString(event.QueryRaw),
		nullableTime(event.RangeStart),
		nullableTime(event.RangeEnd),
		nullableTime(event.QueryTime),
		nullableString(event.Step),
		nullableString(event.Timeout),
		event.StatusCode,
		event.DurationMs,
		nullableString(event.DashboardUID),
		nullableString(event.PanelID),
		nullableString(event.DatasourceUID),
		nullableString(event.RemoteAddr),
		nullableString(event.UserAgent),
		nullableString(event.Error),
	); err != nil {
		return fmt.Errorf("insert query event row: %w", err)
	}

	for _, row := range usageRows {
		if _, err := tx.Exec(ctx, insertMetricQueryUsageSQL,
			row.ID,
			row.ObservedAt,
			row.Tenant,
			row.Source,
			row.MetricName,
			nullableUUID(row.QueryEventID),
			row.QueryHash,
			nullableString(row.DashboardUID),
			nullableString(row.PanelID),
			nullableString(row.DatasourceUID),
			row.UsageKind,
			row.IsRecordingRule,
			nullableString(row.ResolvedFromRecordingRule),
		); err != nil {
			return fmt.Errorf("insert metric query usage row: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit postgres transaction: %w", err)
	}

	return nil
}

func (p *PostgresStorage) InsertExpressions(ctx context.Context, expressions []usage.PromQLExpression) error {
	if len(expressions) == 0 {
		return nil
	}
	if p.pool == nil {
		return fmt.Errorf("postgres storage is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin postgres transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	for _, expression := range expressions {
		if _, err := tx.Exec(ctx, upsertUsageExpressionSQL,
			expression.Day,
			expression.Tenant,
			string(expression.SourceType),
			expression.SourceRef,
			expression.SourceName,
			expression.ExpressionHash,
			expression.Expression,
			expression.DiscoveredAt,
		); err != nil {
			return fmt.Errorf("upsert metric usage expression daily row: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit postgres transaction: %w", err)
	}

	return nil
}

func (p *PostgresStorage) InsertEvidence(ctx context.Context, evidence []usage.MetricUsageEvidence) error {
	if len(evidence) == 0 {
		return nil
	}
	if p.pool == nil {
		return fmt.Errorf("postgres storage is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin postgres transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	for _, row := range evidence {
		queriedAt := row.QueriedAt
		if queriedAt == nil {
			queriedAt = []time.Time{}
		}
		if _, err := tx.Exec(ctx, upsertUsageEvidenceSQL,
			row.Day,
			row.Tenant,
			row.MetricName,
			string(row.SourceType),
			row.SourceRef,
			row.SourceName,
			row.ExpressionHash,
			row.Expression,
			row.UsedLabels,
			row.GroupingLabels,
			row.MatcherLabels,
			row.IsAlert,
			row.IsRecordingRule,
			row.IsDashboard,
			row.IsQueryHistory,
			row.LastQueriedAt,
			row.QueryHistoryCount,
			queriedAt,
			row.DiscoveredAt,
		); err != nil {
			return fmt.Errorf("upsert metric usage evidence daily row: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit postgres transaction: %w", err)
	}

	return nil
}

func (p *PostgresStorage) InsertQueryHistory(ctx context.Context, rows []usage.QueryHistoryRow) error {
	if len(rows) == 0 {
		return nil
	}
	if p.pool == nil {
		return fmt.Errorf("postgres storage is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin postgres transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	for _, row := range rows {
		if _, err := tx.Exec(ctx, upsertQueryHistorySQL,
			row.Day,
			row.Tenant,
			row.DatasourceUID,
			row.QueryHistoryUID,
			row.QueryRefID,
			row.ExpressionHash,
			row.Expression,
			row.CreatedBy,
			row.CreatedAt,
			row.Comment,
			row.Starred,
			row.QueryKey,
			row.EditorMode,
			row.DiscoveredAt,
		); err != nil {
			return fmt.Errorf("upsert grafana query history daily row: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit postgres transaction: %w", err)
	}

	return nil
}

func (p *PostgresStorage) ResolveRecordingRule(ctx context.Context, tenant string, metricName string) ([]storage.ResolvedMetricUsage, error) {
	if err := p.requireInitialized(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	rows, err := p.pool.Query(ctx, resolveRecordingRuleSQL, tenant, metricName)
	if err != nil {
		return nil, fmt.Errorf("resolve recording rule: %w", err)
	}
	defer rows.Close()

	out := make([]storage.ResolvedMetricUsage, 0)
	for rows.Next() {
		var row storage.ResolvedMetricUsage
		if err := rows.Scan(&row.MetricName, &row.RecordingRuleName); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (p *PostgresStorage) Shutdown() error {
	if p.pool != nil {
		p.pool.Close()
		p.pool = nil
	}
	return nil
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func nullableUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (p *PostgresStorage) requireInitialized() error {
	if p == nil || p.pool == nil {
		return fmt.Errorf("postgres storage is not initialized")
	}
	return nil
}

type errorRow struct {
	err error
}

func (r errorRow) Scan(dest ...any) error {
	return r.err
}
