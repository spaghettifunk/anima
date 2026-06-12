package indexer

import (
	"context"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog/log"
	"github.com/spaghettifunk/anima/internal/storage"
)

const (
	defaultBatchSize     = 1000
	defaultFlushInterval = 10 * time.Second
)

type HandlerConfig struct {
	BatchSize             int
	FlushInterval         time.Duration
	Ready                 chan bool
	MetricsTopic          string
	QueryAuditTopic       string
	RecordingRuleResolver RecordingRuleResolver
}

type Handler struct {
	store     storage.Storage
	config    HandlerConfig
	resolver  RecordingRuleResolver
	readyOnce sync.Once
}

func NewHandler(store storage.Storage, cfg HandlerConfig) sarama.ConsumerGroupHandler {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = defaultFlushInterval
	}
	resolver := cfg.RecordingRuleResolver
	if resolver == nil {
		if storeResolver, ok := store.(RecordingRuleResolver); ok {
			resolver = storeResolver
		} else {
			resolver = noopRecordingRuleResolver{}
		}
	}
	return &Handler{
		store:    store,
		config:   cfg,
		resolver: resolver,
	}
}

func (h *Handler) Setup(sarama.ConsumerGroupSession) error {
	h.readyOnce.Do(func() {
		if h.config.Ready != nil {
			close(h.config.Ready)
		}
	})
	return nil
}

func (h *Handler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *Handler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ticker := time.NewTicker(h.config.FlushInterval)
	defer ticker.Stop()

	batch := make([]storage.ObservationRow, 0, h.config.BatchSize)
	messages := make([]*sarama.ConsumerMessage, 0, h.config.BatchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}

		log.Debug().
			Str("topic", claim.Topic()).
			Int32("partition", claim.Partition()).
			Int("size", len(batch)).
			Msg("flushing metric observations")

		if err := h.store.Insert(batch); err != nil {
			log.Error().Err(err).
				Str("topic", claim.Topic()).
				Int32("partition", claim.Partition()).
				Int("size", len(batch)).
				Msg("failed to insert metric observation batch")
			return err
		}

		for _, message := range messages {
			session.MarkMessage(message, "")
		}
		session.Commit()

		batch = batch[:0]
		messages = messages[:0]
		return nil
	}

	for {
		select {
		case <-session.Context().Done():
			return flush()
		case <-ticker.C:
			if err := flush(); err != nil {
				return err
			}
		case message, ok := <-claim.Messages():
			if !ok {
				return flush()
			}

			if h.isQueryAuditTopic(message.Topic) {
				if err := h.handleQueryAuditMessage(session.Context(), message); err != nil {
					return err
				}
				session.MarkMessage(message, "")
				session.Commit()
				continue
			}

			observations, err := DecodeObservations(message.Value, message.Timestamp)
			if err != nil {
				log.Error().Err(err).
					Str("topic", message.Topic).
					Int32("partition", message.Partition).
					Int64("offset", message.Offset).
					Msg("failed to decode metric observation")
				session.MarkMessage(message, "")
				session.Commit()
				continue
			}

			for _, observation := range observations {
				batch = append(batch, observation.StorageRow())
			}
			messages = append(messages, message)

			if len(batch) >= h.config.BatchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}
}

func (h *Handler) isQueryAuditTopic(topic string) bool {
	return h.config.QueryAuditTopic != "" && topic == h.config.QueryAuditTopic
}

func (h *Handler) handleQueryAuditMessage(ctx context.Context, message *sarama.ConsumerMessage) error {
	event, err := DecodeQueryAuditEvent(message.Value)
	if err != nil {
		log.Error().Err(err).
			Str("topic", message.Topic).
			Int32("partition", message.Partition).
			Int64("offset", message.Offset).
			Msg("failed to decode query audit event")
		return nil
	}

	record, warnings, err := NormalizeQueryEvent(event, message.Timestamp, time.Now())
	if err != nil {
		log.Error().Err(err).
			Str("topic", message.Topic).
			Int32("partition", message.Partition).
			Int64("offset", message.Offset).
			Msg("failed to normalize query audit event")
		return err
	}
	for _, warning := range warnings {
		log.Warn().
			Err(warning.Err).
			Str("field", warning.Field).
			Str("value", warning.Value).
			Msg("invalid query audit prometheus time; storing null field")
	}

	usageRows := BuildMetricQueryUsageRecords(ctx, event, record, h.resolver)
	if err := h.store.InsertQueryEventWithUsage(ctx, record, usageRows); err != nil {
		log.Error().Err(err).
			Str("topic", message.Topic).
			Int32("partition", message.Partition).
			Int64("offset", message.Offset).
			Str("usage_kind", record.UsageKind).
			Int("metric_usage_rows", len(usageRows)).
			Msg("failed to insert query audit event")
		return err
	}

	log.Debug().
		Str("topic", message.Topic).
		Int32("partition", message.Partition).
		Int64("offset", message.Offset).
		Str("usage_kind", record.UsageKind).
		Int("metric_usage_rows", len(usageRows)).
		Msg("stored query audit event")
	return nil
}
