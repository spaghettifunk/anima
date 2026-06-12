package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestSourcesConfigLoadsGrafanaTokenFromEnvOnly(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	setRequiredConfig()
	viper.Set("sources.grafana_url", "https://grafana-from-config.example")
	viper.Set("sources.grafana_token", "token-from-config")

	t.Setenv("GRAFANA_URL", "https://grafana-from-env.example")
	t.Setenv("GRAFANA_TOKEN", "token-from-env")

	cfg := New()
	if got := cfg.Sources.GrafanaURL; got != "https://grafana-from-config.example" {
		t.Fatalf("GrafanaURL = %q, want config value", got)
	}
	if got := cfg.Sources.GrafanaToken; got != "token-from-env" {
		t.Fatalf("GrafanaToken = %q, want env value", got)
	}
}

func TestValidateRequiresSources(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	setRequiredConfig()
	viper.Set("sources.tenant", "")

	cfg := New()
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want sources validation error")
	}
}

func TestReporterConfigLoadsDefaultsAndEnvOverrides(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	setRequiredConfig()
	viper.Set("reporter.address", "127.0.0.1")
	viper.Set("reporter.port", 8601)
	viper.Set("reporter.default_tenant", "config-tenant")
	viper.Set("reporter.default_lookback_days", 14)
	viper.Set("reporter.default_limit", 50)
	viper.Set("reporter.max_limit", 500)

	t.Setenv("REPORTER_ADDRESS", "0.0.0.0")
	t.Setenv("REPORTER_PORT", "9700")
	t.Setenv("REPORT_DEFAULT_TENANT", "env-tenant")
	t.Setenv("REPORT_DEFAULT_LOOKBACK_DAYS", "45")
	t.Setenv("REPORT_DEFAULT_LIMIT", "75")
	t.Setenv("REPORT_MAX_LIMIT", "750")

	cfg := New()
	if got := cfg.Reporter.Address; got != "0.0.0.0" {
		t.Fatalf("Reporter.Address = %q, want env override", got)
	}
	if got := cfg.Reporter.Port; got != 9700 {
		t.Fatalf("Reporter.Port = %d, want env override", got)
	}
	if got := cfg.Reporter.DefaultTenant; got != "env-tenant" {
		t.Fatalf("Reporter.DefaultTenant = %q, want env override", got)
	}
	if got := cfg.Reporter.DefaultLookbackDays; got != 45 {
		t.Fatalf("Reporter.DefaultLookbackDays = %d, want env override", got)
	}
	if got := cfg.Reporter.DefaultLimit; got != 75 {
		t.Fatalf("Reporter.DefaultLimit = %d, want env override", got)
	}
	if got := cfg.Reporter.MaxLimit; got != 750 {
		t.Fatalf("Reporter.MaxLimit = %d, want env override", got)
	}
}

func TestQueryHistoryConfigLoadsFromTOMLOnly(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	setRequiredConfig()
	viper.Set("sources.grafana_timeout_seconds", 12)
	viper.Set("sources.query_history_enabled", true)
	viper.Set("sources.query_history_datasource_uid", "mimir")
	viper.Set("sources.query_history_lookback_hours", 48)
	viper.Set("sources.query_history_limit", 250)
	viper.Set("sources.query_history_sort", "time-asc")
	viper.Set("sources.query_history_max_pages", 7)
	viper.Set("reporter.max_queried_at_entries", 9)

	t.Setenv("QUERY_HISTORY_DATASOURCE_UID", "ignored")
	t.Setenv("REPORT_MAX_QUERIED_AT_ENTRIES", "999")

	cfg := New()
	if cfg.Sources.GrafanaTimeoutSeconds != 12 ||
		cfg.Sources.QueryHistoryDatasource != "mimir" ||
		cfg.Sources.QueryHistoryLookbackHrs != 48 ||
		cfg.Sources.QueryHistoryLimit != 250 ||
		cfg.Sources.QueryHistorySort != "time-asc" ||
		cfg.Sources.QueryHistoryMaxPages != 7 {
		t.Fatalf("query history config = %#v", cfg.Sources)
	}
	if cfg.Reporter.MaxQueriedAtEntries != 9 {
		t.Fatalf("MaxQueriedAtEntries = %d, want TOML value", cfg.Reporter.MaxQueriedAtEntries)
	}
}

func TestValidateRequiresQueryHistoryInputsWhenEnabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	setRequiredConfig()
	viper.Set("sources.query_history_enabled", true)
	viper.Set("sources.query_history_datasource_uid", "")

	cfg := New()
	cfg.Sources.QueryHistoryDatasource = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want query history datasource validation error")
	}
}

func TestValidateRequiresMaxQueriedAtEntries(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	setRequiredConfig()

	cfg := New()
	cfg.Reporter.MaxQueriedAtEntries = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want max_queried_at_entries validation error")
	}
}

func TestQueryAuditProxyConfigLoadsDefaultsAndTOML(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	cfg := New()
	if cfg.QueryAuditProxy.Address != "0.0.0.0" ||
		cfg.QueryAuditProxy.Port != 8080 ||
		cfg.QueryAuditProxy.Upstream != "http://thanos-query-frontend.monitoring.svc.cluster.local:9090" ||
		cfg.QueryAuditProxy.EventBufferSize != 10000 ||
		cfg.QueryAuditProxy.AuditTopic != "query-audit-events-v1" ||
		cfg.QueryAuditProxy.LogRawQuery {
		t.Fatalf("default query audit proxy config = %#v", cfg.QueryAuditProxy)
	}

	viper.Set("query_audit_proxy.address", "0.0.0.0")
	viper.Set("query_audit_proxy.port", 9090)
	viper.Set("query_audit_proxy.upstream", "http://thanos.example:9090")
	viper.Set("query_audit_proxy.log_raw_query", true)
	viper.Set("query_audit_proxy.event_buffer_size", 123)
	viper.Set("query_audit_proxy.audit_topic", "audit-topic")

	cfg = New()
	if cfg.QueryAuditProxy.Address != "0.0.0.0" ||
		cfg.QueryAuditProxy.Port != 9090 ||
		cfg.QueryAuditProxy.Upstream != "http://thanos.example:9090" ||
		cfg.QueryAuditProxy.EventBufferSize != 123 ||
		cfg.QueryAuditProxy.AuditTopic != "audit-topic" ||
		!cfg.QueryAuditProxy.LogRawQuery {
		t.Fatalf("query audit proxy config = %#v", cfg.QueryAuditProxy)
	}
}

func TestValidateRequiresQueryAuditProxyInputs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "section",
			mutate: func(cfg *Config) {
				cfg.QueryAuditProxy = nil
			},
		},
		{
			name: "address",
			mutate: func(cfg *Config) {
				cfg.QueryAuditProxy.Address = ""
			},
		},
		{
			name: "port",
			mutate: func(cfg *Config) {
				cfg.QueryAuditProxy.Port = 0
			},
		},
		{
			name: "upstream",
			mutate: func(cfg *Config) {
				cfg.QueryAuditProxy.Upstream = ""
			},
		},
		{
			name: "upstream url",
			mutate: func(cfg *Config) {
				cfg.QueryAuditProxy.Upstream = "thanos-query-frontend:9090"
			},
		},
		{
			name: "event buffer size",
			mutate: func(cfg *Config) {
				cfg.QueryAuditProxy.EventBufferSize = 0
			},
		},
		{
			name: "audit topic",
			mutate: func(cfg *Config) {
				cfg.QueryAuditProxy.AuditTopic = ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			setRequiredConfig()

			cfg := New()
			tt.mutate(cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want query audit proxy validation error")
			}
		})
	}
}

func setRequiredConfig() {
	viper.Set("collector.address", "0.0.0.0")
	viper.Set("collector.port", 8500)
	viper.Set("collector.metrics_topic", "metric-observations-v1")
	viper.Set("collector.traces_topic", "traces-observations-v1")
	viper.Set("collector.logs_topic", "logs-observations-v1")
	viper.Set("producer.brokers", "kafka1:29092")
	viper.Set("producer.partitions_topic", 1)
	viper.Set("consumer.brokers", "kafka1:29092")
	viper.Set("consumer.group", "indexer")
	viper.Set("consumer.batch_size", 1000)
	viper.Set("consumer.flush_interval_seconds", 10)
	viper.Set("consumer.partitions_topic", 1)
	viper.Set("store.address", "postgres")
	viper.Set("store.port", 5432)
	viper.Set("store.dbname", "adaptive_telemetry")
	viper.Set("store.username", "anima")
	viper.Set("store.password", "anima")
	viper.Set("logger.level", "debug")
	viper.Set("sources.rules_url", "localhost:10902")
	viper.Set("sources.grafana_url", "https://grafana.example")
	viper.Set("sources.grafana_token", "token")
	viper.Set("sources.grafana_timeout_seconds", 30)
	viper.Set("sources.query_history_enabled", true)
	viper.Set("sources.query_history_datasource_uid", "thanos")
	viper.Set("sources.query_history_lookback_hours", 24)
	viper.Set("sources.query_history_limit", 100)
	viper.Set("sources.query_history_sort", "time-desc")
	viper.Set("sources.query_history_max_pages", 100)
	viper.Set("sources.tenant", "default-tenant")
	viper.Set("reporter.address", "0.0.0.0")
	viper.Set("reporter.port", 8600)
	viper.Set("reporter.default_tenant", "default-tenant")
	viper.Set("reporter.default_lookback_days", 30)
	viper.Set("reporter.default_limit", 100)
	viper.Set("reporter.max_limit", 1000)
	viper.Set("reporter.max_queried_at_entries", 50)
}
