package entityresolver

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spaghettifunk/anima/internal/config"
	"github.com/spaghettifunk/anima/internal/consumer"
	"github.com/spaghettifunk/anima/internal/utils"
)

type EntityResolver struct {
	ctx       context.Context
	cancel    context.CancelFunc
	config    config.Config
	graph     EntityGraphStore
	analytics EntityAnalyticsStore
	consumer  consumer.KafkaConsumer
	topics    []string
}

func New(cfg config.Config, graph EntityGraphStore, analytics EntityAnalyticsStore) (*EntityResolver, error) {
	ctx, cancel := context.WithCancel(context.Background())
	topics, err := resolverTopics(cfg)
	if err != nil {
		cancel()
		return nil, err
	}
	if graph == nil {
		cancel()
		return nil, fmt.Errorf("entity graph store is required")
	}
	if analytics == nil {
		cancel()
		return nil, fmt.Errorf("entity analytics store is required")
	}
	return &EntityResolver{
		ctx:       ctx,
		cancel:    cancel,
		config:    cfg,
		graph:     graph,
		analytics: analytics,
		topics:    topics,
	}, nil
}

func (r *EntityResolver) Initialize() error {
	if err := r.graph.Initialize(r.ctx); err != nil {
		return err
	}
	if store, ok := r.analytics.(interface{ Initialize() error }); ok {
		if err := store.Initialize(); err != nil {
			_ = r.graph.Shutdown(context.Background())
			return err
		}
	}
	group := strings.TrimSpace(r.config.EntityResolver.ConsumerGroup)
	if group == "" {
		group = "entity-resolver"
	}
	kc, err := consumer.NewKafkaConsumer(r.config.Consumer.Brokers, group, r.ctx, r.cancel)
	if err != nil {
		_ = r.graph.Shutdown(context.Background())
		return err
	}
	r.consumer = kc

	ready := make(chan bool)
	handler := NewHandler(r.graph, r.analytics, HandlerConfig{
		BatchSize:     utils.PositiveOrDefault(r.config.EntityResolver.BatchSize, DefaultBatchSize),
		FlushInterval: time.Duration(utils.PositiveOrDefault(r.config.EntityResolver.FlushIntervalSeconds, int(DefaultFlushInterval/time.Second))) * time.Second,
		Ready:         ready,
		Topics: TopicConfig{
			Metrics: strings.TrimSpace(r.config.Collector.RawMetricsTopic),
			Logs:    strings.TrimSpace(r.config.Collector.RawLogsTopic),
			Traces:  strings.TrimSpace(r.config.Collector.RawTracesTopic),
		},
	})

	log.Info().Strs("topics", r.topics).Str("consumer_group", group).Msg("starting entity resolver consumer")
	return r.consumer.Initialize(r.topics, ready, handler)
}

func (r *EntityResolver) Start() error {
	return r.consumer.GetMessages()
}

func (r *EntityResolver) Shutdown() error {
	r.cancel()
	var err error
	if r.consumer.Consumer != nil {
		err = r.consumer.Shutdown()
	}
	if r.graph != nil {
		if shutdownErr := r.graph.Shutdown(context.Background()); shutdownErr != nil && err == nil {
			err = shutdownErr
		}
	}
	if store, ok := r.analytics.(interface{ Shutdown() error }); ok {
		if shutdownErr := store.Shutdown(); shutdownErr != nil && err == nil {
			err = shutdownErr
		}
	}
	return err
}

func resolverTopics(cfg config.Config) ([]string, error) {
	if cfg.Collector == nil {
		return nil, fmt.Errorf("collector config is required")
	}
	topics := []string{
		strings.TrimSpace(cfg.Collector.RawMetricsTopic),
		strings.TrimSpace(cfg.Collector.RawLogsTopic),
		strings.TrimSpace(cfg.Collector.RawTracesTopic),
	}
	if slices.Contains(topics, "") {
		return nil, fmt.Errorf("raw OTLP topics are required")
	}
	return topics, nil
}
