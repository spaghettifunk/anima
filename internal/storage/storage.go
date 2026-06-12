package storage

import "context"

type Storage interface {
	Initialize() error
	Insert(rows []ObservationRow) error
	InsertQueryEventWithUsage(ctx context.Context, event QueryEventRecord, usage []MetricQueryUsageRecord) error
	Shutdown() error
}
