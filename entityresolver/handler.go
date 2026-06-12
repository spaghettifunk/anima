package entityresolver

import (
	"context"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog/log"
)

const (
	DefaultBatchSize     = 100
	DefaultFlushInterval = 10 * time.Second
)

type HandlerConfig struct {
	BatchSize     int
	FlushInterval time.Duration
	Ready         chan bool
	Topics        TopicConfig
}

type Handler struct {
	graph     EntityGraphStore
	analytics EntityAnalyticsStore
	config    HandlerConfig
	readyOnce sync.Once
}

func NewHandler(graph EntityGraphStore, analytics EntityAnalyticsStore, cfg HandlerConfig) sarama.ConsumerGroupHandler {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = DefaultFlushInterval
	}
	return &Handler{graph: graph, analytics: analytics, config: cfg}
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

	batch := ResolverResult{}
	messages := make([]*sarama.ConsumerMessage, 0, h.config.BatchSize)

	flush := func() error {
		if len(messages) == 0 {
			return nil
		}
		batch = dedupeResult(batch)
		if err := h.persist(session.Context(), batch); err != nil {
			log.Error().Err(err).Int("messages", len(messages)).Msg("failed to persist entity resolver batch")
			return err
		}
		for _, message := range messages {
			session.MarkMessage(message, "")
		}
		session.Commit()
		batch = ResolverResult{}
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
			result, err := ResolvePayload(message.Topic, message.Value, h.config.Topics)
			if err != nil {
				log.Error().Err(err).Str("topic", message.Topic).Int64("offset", message.Offset).Msg("failed to resolve OTLP payload")
				session.MarkMessage(message, "")
				session.Commit()
				continue
			}
			batch.Entities = append(batch.Entities, result.Entities...)
			batch.Relationships = append(batch.Relationships, result.Relationships...)
			batch.HealthWindows = append(batch.HealthWindows, result.HealthWindows...)
			batch.ChangeEvents = append(batch.ChangeEvents, result.ChangeEvents...)
			messages = append(messages, message)
			if len(messages) >= h.config.BatchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}
}

func (h *Handler) persist(ctx context.Context, result ResolverResult) error {
	if err := h.graph.UpsertEntities(ctx, result.Entities); err != nil {
		return err
	}
	if err := h.graph.UpsertRelationships(ctx, result.Relationships); err != nil {
		return err
	}
	if err := h.graph.LinkChangeEvents(ctx, result.ChangeEvents); err != nil {
		return err
	}
	if err := h.analytics.InsertServiceHealthWindows(ctx, result.HealthWindows); err != nil {
		return err
	}
	return h.analytics.InsertChangeEvents(ctx, result.ChangeEvents)
}

func dedupeResult(in ResolverResult) ResolverResult {
	entities := map[string]Entity{}
	for _, entity := range in.Entities {
		if existing, ok := entities[entity.ID]; ok {
			if entity.FirstSeenAt.Before(existing.FirstSeenAt) {
				existing.FirstSeenAt = entity.FirstSeenAt
			}
			if entity.LastSeenAt.After(existing.LastSeenAt) {
				existing.LastSeenAt = entity.LastSeenAt
			}
			entities[entity.ID] = existing
			continue
		}
		entities[entity.ID] = entity
	}
	relationships := map[string]Relationship{}
	for _, relationship := range in.Relationships {
		if existing, ok := relationships[relationship.ID]; ok {
			existing.EvidenceCount += maxInt64(relationship.EvidenceCount, 1)
			if relationship.FirstSeenAt.Before(existing.FirstSeenAt) {
				existing.FirstSeenAt = relationship.FirstSeenAt
			}
			if relationship.LastSeenAt.After(existing.LastSeenAt) {
				existing.LastSeenAt = relationship.LastSeenAt
			}
			relationships[relationship.ID] = existing
			continue
		}
		relationships[relationship.ID] = relationship
	}
	events := map[string]ChangeEvent{}
	for _, event := range in.ChangeEvents {
		events[event.ID] = event
	}
	out := ResolverResult{HealthWindows: in.HealthWindows}
	for _, entity := range entities {
		out.Entities = append(out.Entities, entity)
	}
	for _, relationship := range relationships {
		out.Relationships = append(out.Relationships, relationship)
	}
	for _, event := range events {
		out.ChangeEvents = append(out.ChangeEvents, event)
	}
	return out
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
