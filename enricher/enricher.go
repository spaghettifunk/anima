package enricher

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spaghettifunk/anima/internal/config"
	"github.com/spaghettifunk/anima/internal/sources/grafana"
	"github.com/spaghettifunk/anima/internal/sources/thanos"
	"github.com/spaghettifunk/anima/internal/storage"
	"github.com/spaghettifunk/anima/internal/storage/postgres"
	"github.com/spaghettifunk/anima/internal/usage"
)

type Enricher struct {
	ctx     context.Context
	cancel  context.CancelFunc
	config  config.Config
	store   *postgres.PostgresStorage
	thanos  *thanos.Client
	grafana *grafana.Client
}

func New(cfg config.Config) (*Enricher, error) {
	if cfg.Sources == nil {
		return nil, fmt.Errorf("sources config is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	grafanaTimeout := time.Duration(cfg.Sources.GrafanaTimeoutSeconds) * time.Second
	return &Enricher{
		ctx:     ctx,
		cancel:  cancel,
		config:  cfg,
		store:   postgres.NewFromConfig(cfg),
		thanos:  thanos.NewClient(cfg.Sources.RulesURL),
		grafana: grafana.NewClientWithTimeout(cfg.Sources.GrafanaURL, cfg.Sources.GrafanaToken, grafanaTimeout),
	}, nil
}

func (e *Enricher) Initialize() error {
	return e.store.Initialize()
}

func (e *Enricher) Start() error {
	start := time.Now()
	now := time.Now().UTC()
	day := storage.DayUTC(now)
	tenant := e.config.Sources.Tenant

	thanosRules, err := e.thanos.FetchRules(e.ctx)
	if err != nil {
		return err
	}
	grafanaDashboards, err := e.grafana.FetchDashboards(e.ctx)
	if err != nil {
		return err
	}
	var queryHistoryItems []grafana.QueryHistoryItem
	queryHistoryPagesFetched := 0
	if e.config.Sources.QueryHistoryEnabled {
		queryHistoryRequest := grafana.QueryHistoryRequest{
			DatasourceUID: e.config.Sources.QueryHistoryDatasource,
			From:          now.Add(-time.Duration(e.config.Sources.QueryHistoryLookbackHrs) * time.Hour),
			To:            now,
			Limit:         e.config.Sources.QueryHistoryLimit,
			Page:          1,
			Sort:          e.config.Sources.QueryHistorySort,
		}
		queryHistoryItems, err = e.grafana.FetchAllQueryHistory(e.ctx, queryHistoryRequest, e.config.Sources.QueryHistoryMaxPages)
		if err != nil {
			return err
		}
		if len(queryHistoryItems) > 0 {
			queryHistoryPagesFetched = (len(queryHistoryItems) + e.config.Sources.QueryHistoryLimit - 1) / e.config.Sources.QueryHistoryLimit
		}
	}

	thanosExpressions := thanosExpressions(tenant, day, thanosRules)
	grafanaExpressions := grafanaExpressions(tenant, day, grafanaDashboards)
	queryHistoryExpressions := queryHistoryExpressions(tenant, day, e.config.Sources.QueryHistoryDatasource, queryHistoryItems)
	expressions := append(append(thanosExpressions, grafanaExpressions...), queryHistoryExpressions...)
	validExpressions, evidence, parseErrors := usage.ExtractUsage(expressions, now)
	queryHistoryRows := queryHistoryRows(validExpressions)
	selectorOnlyQueryCount := selectorOnlyQueryHistoryCount(validExpressions, evidence)

	for _, parseError := range parseErrors {
		log.Warn().
			Err(parseError.Err).
			Str("source_type", string(parseError.Expression.SourceType)).
			Str("source_ref", parseError.Expression.SourceRef).
			Str("expression", parseError.Expression.Expression).
			Msg("skipping invalid PromQL expression")
	}
	for _, expression := range validExpressions {
		if expression.SourceType != usage.UsageSourceQueryHistory {
			continue
		}
		if !hasEvidenceForExpression(expression, evidence) {
			log.Info().
				Bool("selector_only_query", true).
				Str("source_ref", expression.SourceRef).
				Str("expression", expression.Expression).
				Msg("query history expression did not contain a concrete metric name")
		}
	}

	if err := e.store.InsertExpressions(e.ctx, validExpressions); err != nil {
		return err
	}
	if err := e.store.InsertQueryHistory(e.ctx, queryHistoryRows); err != nil {
		return err
	}
	if err := e.store.InsertEvidence(e.ctx, evidence); err != nil {
		return err
	}

	log.Info().
		Str("tenant", tenant).
		Time("day", day).
		Int("thanos_expressions_count", len(thanosExpressions)).
		Int("grafana_dashboard_expressions_count", len(grafanaExpressions)).
		Int("query_history_items_count", len(queryHistoryItems)).
		Int("query_history_expressions_count", len(queryHistoryExpressions)).
		Int("query_history_pages_fetched", queryHistoryPagesFetched).
		Int("parse_failures_count", len(parseErrors)).
		Int("selector_only_query_count", selectorOnlyQueryCount).
		Int("evidence_rows_count", len(evidence)).
		Dur("duration", time.Since(start)).
		Int("valid_expressions", len(validExpressions)).
		Msg("usage enrichment completed")

	return nil
}

func (e *Enricher) Shutdown() error {
	e.cancel()
	return e.store.Shutdown()
}

func thanosExpressions(tenant string, day time.Time, groups []thanos.RuleGroup) []usage.PromQLExpression {
	out := make([]usage.PromQLExpression, 0)
	for groupIndex, group := range groups {
		groupName := firstNonEmpty(group.Name, fmt.Sprintf("group-%d", groupIndex))
		for ruleIndex, rule := range group.Rules {
			expression := strings.TrimSpace(rule.Query)
			if expression == "" {
				continue
			}

			ruleName := firstNonEmpty(rule.Name, rule.Alert, rule.Record, fmt.Sprintf("rule-%d", ruleIndex))
			sourceType := usage.UsageSourceThanosRule
			if strings.EqualFold(rule.Type, "alerting") || strings.TrimSpace(rule.Alert) != "" {
				sourceType = usage.UsageSourceThanosAlert
			}

			out = append(out, usage.PromQLExpression{
				Tenant:     tenant,
				Day:        day,
				SourceType: sourceType,
				SourceRef:  fmt.Sprintf("%s/%s", groupName, ruleName),
				SourceName: ruleName,
				Expression: expression,
			})
		}
	}
	return out
}

func grafanaExpressions(tenant string, day time.Time, dashboards []grafana.DashboardDocument) []usage.PromQLExpression {
	out := make([]usage.PromQLExpression, 0)
	for dashboardIndex, dashboard := range dashboards {
		dashboardUID := firstNonEmpty(dashboard.UID, fmt.Sprintf("dashboard-%d", dashboardIndex))
		dashboardTitle := firstNonEmpty(dashboard.Title, dashboard.Dashboard.Title, dashboardUID)
		walkPanels(dashboard.Dashboard.Panels, nil, func(panel grafana.Panel, path []int) {
			panelID := panelID(panel.ID, path)
			panelTitle := strings.TrimSpace(panel.Title)
			for targetIndex, target := range panel.Targets {
				expression := strings.TrimSpace(target.Expr)
				if expression == "" {
					continue
				}
				if !isPrometheusDatasource(target.Datasource, panel.Datasource) || containsUnquotedPipe(expression) {
					continue
				}

				targetRefID := firstNonEmpty(target.RefID, fmt.Sprintf("target-%d", targetIndex))
				sourceName := dashboardTitle
				if panelTitle != "" {
					sourceName = sourceName + "/" + panelTitle
				}

				out = append(out, usage.PromQLExpression{
					Tenant:     tenant,
					Day:        day,
					SourceType: usage.UsageSourceGrafanaDashboard,
					SourceRef:  fmt.Sprintf("%s/%s/%s", dashboardUID, panelID, targetRefID),
					SourceName: sourceName,
					Expression: expression,
				})
			}
		})
	}
	return out
}

func queryHistoryExpressions(tenant string, day time.Time, datasourceUID string, items []grafana.QueryHistoryItem) []usage.PromQLExpression {
	out := make([]usage.PromQLExpression, 0)
	configuredDatasourceUID := strings.TrimSpace(datasourceUID)
	for itemIndex, item := range items {
		historyUID := firstNonEmpty(item.UID, fmt.Sprintf("query-history-%d", itemIndex))
		itemDatasourceUID := firstNonEmpty(item.DatasourceUID, configuredDatasourceUID)
		for queryIndex, query := range item.Queries {
			expression := strings.TrimSpace(query.Expr)
			if expression == "" {
				continue
			}
			datasourceType := strings.TrimSpace(query.Datasource.Type)
			if datasourceType != "" && !strings.EqualFold(datasourceType, "prometheus") {
				continue
			}
			queryDatasourceUID := strings.TrimSpace(query.Datasource.UID)
			if configuredDatasourceUID != "" && queryDatasourceUID != "" && queryDatasourceUID != configuredDatasourceUID {
				continue
			}

			queryRefID := firstNonEmpty(query.RefID, query.Key, fmt.Sprintf("query-%d", queryIndex))
			queriedAt := item.CreatedAt.UTC()
			out = append(out, usage.PromQLExpression{
				Tenant:          tenant,
				Day:             day,
				SourceType:      usage.UsageSourceQueryHistory,
				SourceRef:       fmt.Sprintf("%s/%s", historyUID, queryRefID),
				SourceName:      fmt.Sprintf("grafana-query-history/%s", firstNonEmpty(queryDatasourceUID, itemDatasourceUID, configuredDatasourceUID)),
				Expression:      expression,
				QueriedAt:       &queriedAt,
				QueryHistoryUID: historyUID,
				QueryRefID:      queryRefID,
				DatasourceUID:   firstNonEmpty(queryDatasourceUID, itemDatasourceUID, configuredDatasourceUID),
				CreatedBy:       item.CreatedBy,
				Comment:         item.Comment,
				Starred:         item.Starred,
				QueryKey:        strings.TrimSpace(query.Key),
				EditorMode:      strings.TrimSpace(query.EditorMode),
			})
		}
	}
	return out
}

func queryHistoryRows(expressions []usage.PromQLExpression) []usage.QueryHistoryRow {
	out := make([]usage.QueryHistoryRow, 0)
	for _, expression := range expressions {
		if expression.SourceType != usage.UsageSourceQueryHistory || expression.QueriedAt == nil {
			continue
		}
		out = append(out, usage.QueryHistoryRow{
			Tenant:          expression.Tenant,
			Day:             expression.Day,
			DatasourceUID:   expression.DatasourceUID,
			QueryHistoryUID: expression.QueryHistoryUID,
			QueryRefID:      expression.QueryRefID,
			ExpressionHash:  expression.ExpressionHash,
			Expression:      expression.Expression,
			CreatedBy:       expression.CreatedBy,
			CreatedAt:       expression.QueriedAt.UTC(),
			Comment:         expression.Comment,
			Starred:         expression.Starred,
			QueryKey:        expression.QueryKey,
			EditorMode:      expression.EditorMode,
			DiscoveredAt:    expression.DiscoveredAt,
		})
	}
	return out
}

func selectorOnlyQueryHistoryCount(expressions []usage.PromQLExpression, evidence []usage.MetricUsageEvidence) int {
	count := 0
	for _, expression := range expressions {
		if expression.SourceType != usage.UsageSourceQueryHistory {
			continue
		}
		if !hasEvidenceForExpression(expression, evidence) {
			count++
		}
	}
	return count
}

func hasEvidenceForExpression(expression usage.PromQLExpression, evidence []usage.MetricUsageEvidence) bool {
	for _, row := range evidence {
		if row.SourceType == expression.SourceType &&
			row.SourceRef == expression.SourceRef &&
			row.ExpressionHash == expression.ExpressionHash {
			return true
		}
	}
	return false
}

func walkPanels(panels []grafana.Panel, parentPath []int, visit func(grafana.Panel, []int)) {
	for index, panel := range panels {
		path := append(append([]int{}, parentPath...), index)
		visit(panel, path)
		if len(panel.Panels) > 0 {
			walkPanels(panel.Panels, path, visit)
		}
	}
}

func panelID(value any, path []int) string {
	switch typed := value.(type) {
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case int:
		return strconv.Itoa(typed)
	case string:
		if trimmed := strings.TrimSpace(typed); trimmed != "" {
			return trimmed
		}
	}

	parts := make([]string, 0, len(path))
	for _, item := range path {
		parts = append(parts, strconv.Itoa(item))
	}
	return "panel-" + strings.Join(parts, "-")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isPrometheusDatasource(datasources ...grafana.QueryDatasource) bool {
	for _, datasource := range datasources {
		datasourceType := strings.TrimSpace(datasource.Type)
		if datasourceType == "" {
			continue
		}
		return strings.EqualFold(datasourceType, "prometheus")
	}
	return true
}

func containsUnquotedPipe(expression string) bool {
	inQuote := rune(0)
	escaped := false
	for _, r := range expression {
		if escaped {
			escaped = false
			continue
		}
		if inQuote != 0 {
			if inQuote != '`' && r == '\\' {
				escaped = true
				continue
			}
			if r == inQuote {
				inQuote = 0
			}
			continue
		}
		switch r {
		case '\'', '"', '`':
			inQuote = r
		case '|':
			return true
		}
	}
	return false
}
