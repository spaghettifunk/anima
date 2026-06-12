package indexer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/spaghettifunk/anima/internal/storage"
)

type fakeStorage struct {
	inserted       [][]storage.ObservationRow
	queryEvents    []storage.QueryEventRecord
	queryUsage     [][]storage.MetricQueryUsageRecord
	fail           bool
	failQueryUsage bool
}

func (f *fakeStorage) Initialize() error {
	return nil
}

func (f *fakeStorage) Insert(data []storage.ObservationRow) error {
	if f.fail {
		return errors.New("insert failed")
	}
	copied := append([]storage.ObservationRow(nil), data...)
	f.inserted = append(f.inserted, copied)
	return nil
}

func (f *fakeStorage) InsertQueryEventWithUsage(ctx context.Context, event storage.QueryEventRecord, usage []storage.MetricQueryUsageRecord) error {
	if f.failQueryUsage {
		return errors.New("query usage insert failed")
	}
	f.queryEvents = append(f.queryEvents, event)
	f.queryUsage = append(f.queryUsage, append([]storage.MetricQueryUsageRecord(nil), usage...))
	return nil
}

func (f *fakeStorage) Shutdown() error {
	return nil
}

type fakeSession struct {
	ctx     context.Context
	marked  []*sarama.ConsumerMessage
	commits int
}

func (f *fakeSession) Claims() map[string][]int32 {
	return nil
}

func (f *fakeSession) MemberID() string {
	return "member"
}

func (f *fakeSession) GenerationID() int32 {
	return 1
}

func (f *fakeSession) MarkOffset(string, int32, int64, string) {
}

func (f *fakeSession) Commit() {
	f.commits++
}

func (f *fakeSession) ResetOffset(string, int32, int64, string) {
}

func (f *fakeSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {
	f.marked = append(f.marked, msg)
}

func (f *fakeSession) Context() context.Context {
	return f.ctx
}

type fakeClaim struct {
	topic     string
	partition int32
	messages  chan *sarama.ConsumerMessage
}

func (f *fakeClaim) Topic() string {
	return f.topic
}

func (f *fakeClaim) Partition() int32 {
	return f.partition
}

func (f *fakeClaim) InitialOffset() int64 {
	return 0
}

func (f *fakeClaim) HighWaterMarkOffset() int64 {
	return 0
}

func (f *fakeClaim) Messages() <-chan *sarama.ConsumerMessage {
	return f.messages
}

func TestHandlerInsertsValidMessages(t *testing.T) {
	store := &fakeStorage{}
	session := &fakeSession{ctx: context.Background()}
	claim := claimWithMessages(validMessage(1))

	handler := NewHandler(store, HandlerConfig{BatchSize: 100, FlushInterval: time.Hour})
	if err := handler.ConsumeClaim(session, claim); err != nil {
		t.Fatalf("ConsumeClaim() error = %v", err)
	}

	if len(store.inserted) != 1 {
		t.Fatalf("insert batches = %d, want 1", len(store.inserted))
	}
	if len(store.inserted[0]) != 1 {
		t.Fatalf("inserted rows = %d, want 1", len(store.inserted[0]))
	}
	if len(session.marked) != 1 {
		t.Fatalf("marked messages = %d, want 1", len(session.marked))
	}
}

func TestHandlerSkipsInvalidMessages(t *testing.T) {
	store := &fakeStorage{}
	session := &fakeSession{ctx: context.Background()}
	claim := claimWithMessages(&sarama.ConsumerMessage{
		Topic:     "test-topic",
		Partition: 0,
		Offset:    1,
		Value:     []byte(`{"tenant":"prod","cluster":"a","metric_name":""}`),
		Timestamp: time.Now(),
	})

	handler := NewHandler(store, HandlerConfig{BatchSize: 100, FlushInterval: time.Hour})
	if err := handler.ConsumeClaim(session, claim); err != nil {
		t.Fatalf("ConsumeClaim() error = %v", err)
	}

	if len(store.inserted) != 0 {
		t.Fatalf("insert batches = %d, want 0", len(store.inserted))
	}
	if len(session.marked) != 1 {
		t.Fatalf("marked messages = %d, want 1", len(session.marked))
	}
}

func TestHandlerFlushesAtBatchSize(t *testing.T) {
	store := &fakeStorage{}
	session := &fakeSession{ctx: context.Background()}
	claim := claimWithMessages(validArrayMessage(1))

	handler := NewHandler(store, HandlerConfig{BatchSize: 2, FlushInterval: time.Hour})
	if err := handler.ConsumeClaim(session, claim); err != nil {
		t.Fatalf("ConsumeClaim() error = %v", err)
	}

	if len(store.inserted) != 1 {
		t.Fatalf("insert batches = %d, want 1", len(store.inserted))
	}
	if len(store.inserted[0]) != 2 {
		t.Fatalf("inserted rows = %d, want 2", len(store.inserted[0]))
	}
	if len(session.marked) != 1 {
		t.Fatalf("marked messages = %d, want 1", len(session.marked))
	}
}

func TestHandlerStorageFailureDoesNotMarkMessages(t *testing.T) {
	store := &fakeStorage{fail: true}
	session := &fakeSession{ctx: context.Background()}
	claim := claimWithMessages(validMessage(1))

	handler := NewHandler(store, HandlerConfig{BatchSize: 100, FlushInterval: time.Hour})
	if err := handler.ConsumeClaim(session, claim); err == nil {
		t.Fatal("ConsumeClaim() error = nil, want error")
	}

	if len(session.marked) != 0 {
		t.Fatalf("marked messages = %d, want 0", len(session.marked))
	}
}

func TestHandlerRoutesQueryAuditTopic(t *testing.T) {
	store := &fakeStorage{}
	session := &fakeSession{ctx: context.Background()}
	claim := claimWithMessages(&sarama.ConsumerMessage{
		Topic:     "query-audit-events-v1",
		Partition: 0,
		Offset:    1,
		Value:     []byte(`{"timestamp":"2026-06-10T08:00:00Z","method":"GET","path":"/api/v1/query","query":"up","org_id":"prod","query_hash":"hash-a"}`),
		Timestamp: time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC),
	})

	handler := NewHandler(store, HandlerConfig{BatchSize: 100, FlushInterval: time.Hour, QueryAuditTopic: "query-audit-events-v1"})
	if err := handler.ConsumeClaim(session, claim); err != nil {
		t.Fatalf("ConsumeClaim() error = %v", err)
	}

	if len(store.inserted) != 0 {
		t.Fatalf("metric insert batches = %d, want 0", len(store.inserted))
	}
	if len(store.queryEvents) != 1 {
		t.Fatalf("query events = %d, want 1", len(store.queryEvents))
	}
	if got := store.queryEvents[0].UsageKind; got != UsageKindDirectQuery {
		t.Fatalf("UsageKind = %q, want direct query", got)
	}
	if len(store.queryUsage) != 1 || len(store.queryUsage[0]) != 1 || store.queryUsage[0][0].MetricName != "up" {
		t.Fatalf("query usage rows = %#v, want one up row", store.queryUsage)
	}
	if len(session.marked) != 1 || session.commits != 1 {
		t.Fatalf("marked=%d commits=%d, want one committed audit message", len(session.marked), session.commits)
	}
}

func TestHandlerSkipsInvalidQueryAuditJSON(t *testing.T) {
	store := &fakeStorage{}
	session := &fakeSession{ctx: context.Background()}
	claim := claimWithMessages(&sarama.ConsumerMessage{
		Topic:     "query-audit-events-v1",
		Partition: 0,
		Offset:    1,
		Value:     []byte(`{"timestamp":`),
		Timestamp: time.Now(),
	})

	handler := NewHandler(store, HandlerConfig{BatchSize: 100, FlushInterval: time.Hour, QueryAuditTopic: "query-audit-events-v1"})
	if err := handler.ConsumeClaim(session, claim); err != nil {
		t.Fatalf("ConsumeClaim() error = %v", err)
	}

	if len(store.queryEvents) != 0 {
		t.Fatalf("query events = %d, want 0", len(store.queryEvents))
	}
	if len(session.marked) != 1 {
		t.Fatalf("marked messages = %d, want invalid audit message marked", len(session.marked))
	}
}

func TestHandlerQueryAuditStorageFailureDoesNotMark(t *testing.T) {
	store := &fakeStorage{failQueryUsage: true}
	session := &fakeSession{ctx: context.Background()}
	claim := claimWithMessages(&sarama.ConsumerMessage{
		Topic:     "query-audit-events-v1",
		Partition: 0,
		Offset:    1,
		Value:     []byte(`{"timestamp":"2026-06-10T08:00:00Z","method":"GET","path":"/api/v1/query","query":"up","org_id":"prod"}`),
		Timestamp: time.Now(),
	})

	handler := NewHandler(store, HandlerConfig{BatchSize: 100, FlushInterval: time.Hour, QueryAuditTopic: "query-audit-events-v1"})
	if err := handler.ConsumeClaim(session, claim); err == nil {
		t.Fatal("ConsumeClaim() error = nil, want storage failure")
	}

	if len(session.marked) != 0 {
		t.Fatalf("marked messages = %d, want 0", len(session.marked))
	}
}

func claimWithMessages(messages ...*sarama.ConsumerMessage) *fakeClaim {
	ch := make(chan *sarama.ConsumerMessage, len(messages))
	for _, message := range messages {
		ch <- message
	}
	close(ch)
	return &fakeClaim{topic: "test-topic", messages: ch}
}

func validMessage(offset int64) *sarama.ConsumerMessage {
	return &sarama.ConsumerMessage{
		Topic:     "test-topic",
		Partition: 0,
		Offset:    offset,
		Value:     []byte(`{"tenant":"prod","cluster":"a","metric_name":"requests","observed_at":"2026-06-03T06:00:00Z"}`),
		Timestamp: time.Now(),
	}
}

func validArrayMessage(offset int64) *sarama.ConsumerMessage {
	return &sarama.ConsumerMessage{
		Topic:     "test-topic",
		Partition: 0,
		Offset:    offset,
		Value:     []byte(`[{"tenant":"prod","cluster":"a","metric_name":"requests","observed_at":"2026-06-03T06:00:00Z"},{"tenant":"prod","cluster":"a","metric_name":"latency","observed_at":"2026-06-03T06:00:00Z"}]`),
		Timestamp: time.Now(),
	}
}
