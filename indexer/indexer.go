package indexer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spaghettifunk/anima/internal/config"
	"github.com/spaghettifunk/anima/internal/consumer"
	"github.com/spaghettifunk/anima/internal/storage"
	"github.com/spaghettifunk/anima/internal/storage/postgres"
)

type Indexer struct {
	ctx      context.Context
	cancel   context.CancelFunc
	config   config.Config
	store    storage.Storage
	consumer consumer.KafkaConsumer
	topics   []string
}

func New(cfg config.Config) (*Indexer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	store := postgres.NewFromConfig(cfg)

	topics, err := indexerTopics(cfg)
	if err != nil {
		cancel()
		return nil, err
	}

	return &Indexer{
		ctx:    ctx,
		cancel: cancel,
		config: cfg,
		store:  store,
		topics: topics,
	}, nil
}

func (i *Indexer) Initialize() error {
	if strings.TrimSpace(i.config.Consumer.Brokers) == "" {
		return fmt.Errorf("consumer brokers are required")
	}

	if err := i.store.Initialize(); err != nil {
		return err
	}

	consumerGroup := strings.TrimSpace(i.config.Consumer.Group)
	if consumerGroup == "" {
		consumerGroup = "indexer"
	}

	kc, err := consumer.NewKafkaConsumer(i.config.Consumer.Brokers, consumerGroup, i.ctx, i.cancel)
	if err != nil {
		_ = i.store.Shutdown()
		return err
	}
	i.consumer = kc

	ready := make(chan bool)
	handler := NewHandler(i.store, HandlerConfig{
		BatchSize:       positiveOrDefault(i.config.Consumer.BatchSize, defaultBatchSize),
		FlushInterval:   time.Duration(positiveOrDefault(i.config.Consumer.FlushIntervalSeconds, int(defaultFlushInterval/time.Second))) * time.Second,
		Ready:           ready,
		MetricsTopic:    strings.TrimSpace(i.config.Collector.ObservationsMetricsTopic),
		QueryAuditTopic: strings.TrimSpace(i.config.QueryAuditProxy.AuditTopic),
	})

	log.Info().
		Strs("topics", i.topics).
		Str("consumer_group", consumerGroup).
		Msg("starting indexer consumer")

	return i.consumer.Initialize(i.topics, ready, handler)
}

func (i *Indexer) Start() error {
	return i.consumer.GetMessages()
}

func (i *Indexer) Shutdown() error {
	i.cancel()

	var err error
	if i.consumer.Consumer != nil {
		err = i.consumer.Shutdown()
	}
	if shutdownErr := i.store.Shutdown(); shutdownErr != nil && err == nil {
		err = shutdownErr
	}
	return err
}

func positiveOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func indexerTopics(cfg config.Config) ([]string, error) {
	if cfg.Collector == nil {
		return nil, fmt.Errorf("collector config is required")
	}
	if cfg.QueryAuditProxy == nil {
		return nil, fmt.Errorf("query audit proxy config is required")
	}

	metricsTopic := strings.TrimSpace(cfg.Collector.ObservationsMetricsTopic)
	queryAuditTopic := strings.TrimSpace(cfg.QueryAuditProxy.AuditTopic)
	if metricsTopic == "" {
		return nil, fmt.Errorf("collector metrics topic is required")
	}
	if queryAuditTopic == "" {
		return nil, fmt.Errorf("query audit topic is required")
	}
	if metricsTopic == queryAuditTopic {
		return nil, fmt.Errorf("collector metrics topic and query audit topic must be distinct")
	}

	return []string{metricsTopic, queryAuditTopic}, nil
}
