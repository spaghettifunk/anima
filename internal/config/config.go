package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Collector       *collector       `toml:"collector"`
	Producer        *producer        `toml:"producer"`
	Consumer        *consumer        `toml:"consumer"`
	Logger          *logger          `toml:"logger"`
	Sources         *sources         `toml:"sources"`
	Store           *store           `toml:"store"`
	Reporter        *reporter        `toml:"reporter"`
	QueryAuditProxy *queryAuditProxy `toml:"query_audit_proxy"`
}

type collector struct {
	Address      string `toml:"address"`
	Port         int    `toml:"port"`
	MetricsTopic string `toml:"metrics_topic"`
	TracesTopic  string `toml:"traces_topic"`
	LogsTopic    string `toml:"logs_topic"`
}

type store struct {
	Address  string `toml:"address"`
	Port     int    `toml:"port"`
	DBName   string `toml:"dbname"`
	Username string `toml:"username"`
	Password string `toml:"password"`
	TLS      bool   `toml:"tls"`
}

type producer struct {
	Brokers            string `toml:"brokers"`
	PartitionsPerTopic int32  `toml:"partitions_topic"`
}

type consumer struct {
	Brokers              string `toml:"brokers"`
	Group                string `toml:"group"`
	BatchSize            int    `toml:"batch_size"`
	FlushIntervalSeconds int    `toml:"flush_interval_seconds"`
	PartitionsPerTopic   int32  `toml:"partitions_topic"`
}

type logger struct {
	Level string `toml:"level"`
}

type sources struct {
	RulesURL                string `toml:"rules_url"`
	GrafanaURL              string `toml:"grafana_url"`
	GrafanaToken            string `toml:"grafana_token"`
	GrafanaTimeoutSeconds   int    `toml:"grafana_timeout_seconds"`
	QueryHistoryEnabled     bool   `toml:"query_history_enabled"`
	QueryHistoryDatasource  string `toml:"query_history_datasource_uid"`
	QueryHistoryLookbackHrs int    `toml:"query_history_lookback_hours"`
	QueryHistoryLimit       int    `toml:"query_history_limit"`
	QueryHistorySort        string `toml:"query_history_sort"`
	QueryHistoryMaxPages    int    `toml:"query_history_max_pages"`
	Tenant                  string `toml:"tenant"`
}

type reporter struct {
	Address             string `toml:"address"`
	Port                int    `toml:"port"`
	DefaultTenant       string `toml:"default_tenant"`
	DefaultLookbackDays int    `toml:"default_lookback_days"`
	DefaultLimit        int    `toml:"default_limit"`
	MaxLimit            int    `toml:"max_limit"`
	MaxQueriedAtEntries int    `toml:"max_queried_at_entries"`
}

type queryAuditProxy struct {
	Address         string `toml:"address"`
	Port            int    `toml:"port"`
	Upstream        string `toml:"upstream"`
	LogRawQuery     bool   `toml:"log_raw_query"`
	EventBufferSize int    `toml:"event_buffer_size"`
	AuditTopic      string `toml:"audit_topic"`
}

func New() *Config {
	grafanaToken := viper.GetString("sources.grafana_token")
	if envToken := strings.TrimSpace(os.Getenv("GRAFANA_TOKEN")); envToken != "" {
		grafanaToken = envToken
	}
	queryHistoryEnabled := true
	if viper.IsSet("sources.query_history_enabled") {
		queryHistoryEnabled = viper.GetBool("sources.query_history_enabled")
	}

	reporterTenant := envString("REPORT_DEFAULT_TENANT", viper.GetString("reporter.default_tenant"))
	if strings.TrimSpace(reporterTenant) == "" {
		reporterTenant = viper.GetString("sources.tenant")
	}

	return &Config{
		Collector: &collector{
			Address:      viper.GetString("collector.address"),
			Port:         viper.GetInt("collector.port"),
			MetricsTopic: viper.GetString("collector.metrics_topic"),
			TracesTopic:  viper.GetString("collector.traces_topic"),
			LogsTopic:    viper.GetString("collector.logs_topic"),
		},
		Store: &store{
			Address:  viper.GetString("store.address"),
			Port:     viper.GetInt("store.port"),
			DBName:   viper.GetString("store.dbname"),
			Username: viper.GetString("store.username"),
			Password: viper.GetString("store.password"),
			TLS:      viper.GetBool("store.tls"),
		},
		Producer: &producer{
			Brokers:            viper.GetString("producer.brokers"),
			PartitionsPerTopic: viper.GetInt32("producer.partitions_topic"),
		},
		Consumer: &consumer{
			Brokers:              viper.GetString("consumer.brokers"),
			Group:                viper.GetString("consumer.group"),
			BatchSize:            viper.GetInt("consumer.batch_size"),
			FlushIntervalSeconds: viper.GetInt("consumer.flush_interval_seconds"),
			PartitionsPerTopic:   viper.GetInt32("consumer.partitions_topic"),
		},
		Logger: &logger{
			Level: viper.GetString("logger.level"),
		},
		Sources: &sources{
			RulesURL:                viper.GetString("sources.rules_url"),
			GrafanaURL:              viper.GetString("sources.grafana_url"),
			GrafanaToken:            grafanaToken,
			GrafanaTimeoutSeconds:   intOrDefault(viper.GetInt("sources.grafana_timeout_seconds"), 30),
			QueryHistoryEnabled:     queryHistoryEnabled,
			QueryHistoryDatasource:  stringOrDefault(viper.GetString("sources.query_history_datasource_uid"), "thanos"),
			QueryHistoryLookbackHrs: intOrDefault(viper.GetInt("sources.query_history_lookback_hours"), 24),
			QueryHistoryLimit:       intOrDefault(viper.GetInt("sources.query_history_limit"), 100),
			QueryHistorySort:        stringOrDefault(viper.GetString("sources.query_history_sort"), "time-desc"),
			QueryHistoryMaxPages:    intOrDefault(viper.GetInt("sources.query_history_max_pages"), 100),
			Tenant:                  viper.GetString("sources.tenant"),
		},
		Reporter: &reporter{
			Address:             envString("REPORTER_ADDRESS", stringOrDefault(viper.GetString("reporter.address"), "0.0.0.0")),
			Port:                envInt("REPORTER_PORT", intOrDefault(viper.GetInt("reporter.port"), 8600)),
			DefaultTenant:       reporterTenant,
			DefaultLookbackDays: envInt("REPORT_DEFAULT_LOOKBACK_DAYS", intOrDefault(viper.GetInt("reporter.default_lookback_days"), 30)),
			DefaultLimit:        envInt("REPORT_DEFAULT_LIMIT", intOrDefault(viper.GetInt("reporter.default_limit"), 100)),
			MaxLimit:            envInt("REPORT_MAX_LIMIT", intOrDefault(viper.GetInt("reporter.max_limit"), 1000)),
			MaxQueriedAtEntries: intOrDefault(viper.GetInt("reporter.max_queried_at_entries"), 50),
		},
		QueryAuditProxy: &queryAuditProxy{
			Address:         stringOrDefault(viper.GetString("query_audit_proxy.address"), "0.0.0.0"),
			Port:            intOrDefault(viper.GetInt("query_audit_proxy.port"), 8080),
			Upstream:        stringOrDefault(viper.GetString("query_audit_proxy.upstream"), "http://thanos-query-frontend.monitoring.svc.cluster.local:9090"),
			LogRawQuery:     viper.GetBool("query_audit_proxy.log_raw_query"),
			EventBufferSize: intOrDefault(viper.GetInt("query_audit_proxy.event_buffer_size"), 10000),
			AuditTopic:      stringOrDefault(viper.GetString("query_audit_proxy.audit_topic"), "query-audit-events-v1"),
		},
	}
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is required")
	}
	if c.Collector == nil {
		return fmt.Errorf("collector config is required")
	}
	if c.Producer == nil {
		return fmt.Errorf("producer config is required")
	}
	if c.Consumer == nil {
		return fmt.Errorf("consumer config is required")
	}
	if c.Logger == nil {
		return fmt.Errorf("logger config is required")
	}
	if c.Sources == nil {
		return fmt.Errorf("sources config is required")
	}
	if c.Store == nil {
		return fmt.Errorf("store config is required")
	}
	if c.Reporter == nil {
		return fmt.Errorf("reporter config is required")
	}
	if c.QueryAuditProxy == nil {
		return fmt.Errorf("query audit proxy config is required")
	}

	if err := requireString("collector.address", c.Collector.Address); err != nil {
		return err
	}
	if err := requirePositiveInt("collector.port", c.Collector.Port); err != nil {
		return err
	}
	if err := requireString("collector.metrics_topic", c.Collector.MetricsTopic); err != nil {
		return err
	}
	if err := requireString("collector.traces_topic", c.Collector.TracesTopic); err != nil {
		return err
	}
	if err := requireString("collector.logs_topic", c.Collector.LogsTopic); err != nil {
		return err
	}
	if err := requireString("store.address", c.Store.Address); err != nil {
		return err
	}
	if err := requirePositiveInt("store.port", c.Store.Port); err != nil {
		return err
	}
	if err := requireString("store.dbname", c.Store.DBName); err != nil {
		return err
	}
	if err := requireString("store.username", c.Store.Username); err != nil {
		return err
	}
	if err := requireString("store.password", c.Store.Password); err != nil {
		return err
	}
	if err := requireString("producer.brokers", c.Producer.Brokers); err != nil {
		return err
	}
	if err := requirePositiveInt32("producer.partitions_topic", c.Producer.PartitionsPerTopic); err != nil {
		return err
	}
	if err := requireString("consumer.brokers", c.Consumer.Brokers); err != nil {
		return err
	}
	if err := requireString("consumer.group", c.Consumer.Group); err != nil {
		return err
	}
	if err := requirePositiveInt("consumer.batch_size", c.Consumer.BatchSize); err != nil {
		return err
	}
	if err := requirePositiveInt("consumer.flush_interval_seconds", c.Consumer.FlushIntervalSeconds); err != nil {
		return err
	}
	if err := requirePositiveInt32("consumer.partitions_topic", c.Consumer.PartitionsPerTopic); err != nil {
		return err
	}
	if err := requireString("logger.level", c.Logger.Level); err != nil {
		return err
	}
	if err := requireString("sources.rules_url", c.Sources.RulesURL); err != nil {
		return err
	}
	if err := requireString("sources.grafana_url", c.Sources.GrafanaURL); err != nil {
		return err
	}
	if err := requireString("sources.grafana_token", c.Sources.GrafanaToken); err != nil {
		return err
	}
	if err := requirePositiveInt("sources.grafana_timeout_seconds", c.Sources.GrafanaTimeoutSeconds); err != nil {
		return err
	}
	if c.Sources.QueryHistoryEnabled {
		if err := requireString("sources.query_history_datasource_uid", c.Sources.QueryHistoryDatasource); err != nil {
			return err
		}
		if err := requireString("sources.query_history_sort", c.Sources.QueryHistorySort); err != nil {
			return err
		}
		if err := requirePositiveInt("sources.query_history_lookback_hours", c.Sources.QueryHistoryLookbackHrs); err != nil {
			return err
		}
		if err := requirePositiveInt("sources.query_history_limit", c.Sources.QueryHistoryLimit); err != nil {
			return err
		}
		if err := requirePositiveInt("sources.query_history_max_pages", c.Sources.QueryHistoryMaxPages); err != nil {
			return err
		}
	}
	if err := requireString("sources.tenant", c.Sources.Tenant); err != nil {
		return err
	}
	if err := requireString("reporter.address", c.Reporter.Address); err != nil {
		return err
	}
	if err := requirePositiveInt("reporter.port", c.Reporter.Port); err != nil {
		return err
	}
	if err := requireString("reporter.default_tenant", c.Reporter.DefaultTenant); err != nil {
		return err
	}
	if err := requirePositiveInt("reporter.default_lookback_days", c.Reporter.DefaultLookbackDays); err != nil {
		return err
	}
	if err := requirePositiveInt("reporter.default_limit", c.Reporter.DefaultLimit); err != nil {
		return err
	}
	if err := requirePositiveInt("reporter.max_limit", c.Reporter.MaxLimit); err != nil {
		return err
	}
	if c.Reporter.DefaultLimit > c.Reporter.MaxLimit {
		return fmt.Errorf("config value %q must be less than or equal to %q", "reporter.default_limit", "reporter.max_limit")
	}
	if err := requirePositiveInt("reporter.max_queried_at_entries", c.Reporter.MaxQueriedAtEntries); err != nil {
		return err
	}
	if err := requireString("query_audit_proxy.address", c.QueryAuditProxy.Address); err != nil {
		return err
	}
	if err := requirePositiveInt("query_audit_proxy.port", c.QueryAuditProxy.Port); err != nil {
		return err
	}
	if err := requireString("query_audit_proxy.upstream", c.QueryAuditProxy.Upstream); err != nil {
		return err
	}
	if err := requireURL("query_audit_proxy.upstream", c.QueryAuditProxy.Upstream); err != nil {
		return err
	}
	if err := requirePositiveInt("query_audit_proxy.event_buffer_size", c.QueryAuditProxy.EventBufferSize); err != nil {
		return err
	}
	if err := requireString("query_audit_proxy.audit_topic", c.QueryAuditProxy.AuditTopic); err != nil {
		return err
	}

	return nil
}

func requireString(name string, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("config value %q must not be empty", name)
	}
	return nil
}

func requirePositiveInt(name string, value int) error {
	if value <= 0 {
		return fmt.Errorf("config value %q must be greater than zero", name)
	}
	return nil
}

func requirePositiveInt32(name string, value int32) error {
	if value <= 0 {
		return fmt.Errorf("config value %q must be greater than zero", name)
	}
	return nil
}

func requireURL(name, value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("config value %q must be a valid URL: %w", name, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("config value %q must include scheme and host", name)
	}
	return nil
}

func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func stringOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func intOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
