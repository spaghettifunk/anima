package entityresolver

import (
	"math"
	"sort"
	"time"
)

type healthAccumulator struct {
	durations []time.Duration
	items     map[string]*healthBucket
}

type healthBucket struct {
	ServiceID      string
	Window         string
	WindowStart    time.Time
	WindowEnd      time.Time
	RequestCount   int64
	ErrorCount     int64
	LogErrorCount  int64
	SpanErrorCount int64
	DurationsMs    []float64
}

func newHealthAccumulator(windows []time.Duration) *healthAccumulator {
	return &healthAccumulator{durations: windows, items: map[string]*healthBucket{}}
}

func (a *healthAccumulator) addSpan(serviceID string, observedAt time.Time, duration time.Duration, isError bool) {
	for _, window := range a.durations {
		bucket := a.bucket(serviceID, observedAt, window)
		bucket.RequestCount++
		if isError {
			bucket.ErrorCount++
			bucket.SpanErrorCount++
		}
		if duration > 0 {
			bucket.DurationsMs = append(bucket.DurationsMs, float64(duration)/float64(time.Millisecond))
		}
	}
}

func (a *healthAccumulator) addLogError(serviceID string, observedAt time.Time) {
	for _, window := range a.durations {
		bucket := a.bucket(serviceID, observedAt, window)
		bucket.ErrorCount++
		bucket.LogErrorCount++
	}
}

func (a *healthAccumulator) bucket(serviceID string, observedAt time.Time, window time.Duration) *healthBucket {
	start := observedAt.UTC().Truncate(window)
	key := serviceID + "|" + window.String() + "|" + start.Format(time.RFC3339Nano)
	if bucket, ok := a.items[key]; ok {
		return bucket
	}
	bucket := &healthBucket{ServiceID: serviceID, Window: window.String(), WindowStart: start, WindowEnd: start.Add(window)}
	a.items[key] = bucket
	return bucket
}

func (a *healthAccumulator) windows() []ServiceHealthWindow {
	out := make([]ServiceHealthWindow, 0, len(a.items))
	for _, bucket := range a.items {
		errorRate := 0.0
		if bucket.RequestCount > 0 {
			errorRate = float64(bucket.ErrorCount) / float64(bucket.RequestCount)
		}
		out = append(out, ServiceHealthWindow{
			ServiceID:      bucket.ServiceID,
			Window:         bucket.Window,
			WindowStart:    bucket.WindowStart,
			WindowEnd:      bucket.WindowEnd,
			RequestCount:   bucket.RequestCount,
			ErrorCount:     bucket.ErrorCount,
			ErrorRate:      errorRate,
			DurationP50Ms:  percentile(bucket.DurationsMs, 0.50),
			DurationP95Ms:  percentile(bucket.DurationsMs, 0.95),
			DurationP99Ms:  percentile(bucket.DurationsMs, 0.99),
			LogErrorCount:  bucket.LogErrorCount,
			SpanErrorCount: bucket.SpanErrorCount,
			Source:         sourceOTLP,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ServiceID == out[j].ServiceID {
			return out[i].WindowStart.Before(out[j].WindowStart)
		}
		return out[i].ServiceID < out[j].ServiceID
	})
	return out
}

func percentile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	if len(cp) == 1 {
		return cp[0]
	}
	pos := q * float64(len(cp)-1)
	lower := int(math.Floor(pos))
	upper := int(math.Ceil(pos))
	if lower == upper {
		return cp[lower]
	}
	return cp[lower] + (cp[upper]-cp[lower])*(pos-float64(lower))
}
