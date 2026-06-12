package entityresolver

import (
	"bytes"
	"fmt"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func ResolvePayload(topic string, payload []byte, topics TopicConfig) (ResolverResult, error) {
	switch topic {
	case topics.Metrics:
		md, err := decodeMetrics(payload)
		if err != nil {
			return ResolverResult{}, err
		}
		return ResolveFromMetrics(md), nil
	case topics.Logs:
		ld, err := decodeLogs(payload)
		if err != nil {
			return ResolverResult{}, err
		}
		return ResolveFromLogs(ld), nil
	case topics.Traces:
		td, err := decodeTraces(payload)
		if err != nil {
			return ResolverResult{}, err
		}
		return ResolveFromTraces(td), nil
	default:
		return ResolverResult{}, fmt.Errorf("unsupported entity resolver topic %q", topic)
	}
}

type TopicConfig struct {
	Metrics string
	Logs    string
	Traces  string
}

func decodeMetrics(payload []byte) (pmetric.Metrics, error) {
	if isJSONPayload(payload) {
		decoder := &pmetric.JSONUnmarshaler{}
		return decoder.UnmarshalMetrics(payload)
	}
	decoder := &pmetric.ProtoUnmarshaler{}
	return decoder.UnmarshalMetrics(payload)
}

func decodeLogs(payload []byte) (plog.Logs, error) {
	if isJSONPayload(payload) {
		decoder := &plog.JSONUnmarshaler{}
		return decoder.UnmarshalLogs(payload)
	}
	decoder := &plog.ProtoUnmarshaler{}
	return decoder.UnmarshalLogs(payload)
}

func decodeTraces(payload []byte) (ptrace.Traces, error) {
	if isJSONPayload(payload) {
		decoder := &ptrace.JSONUnmarshaler{}
		return decoder.UnmarshalTraces(payload)
	}
	decoder := &ptrace.ProtoUnmarshaler{}
	return decoder.UnmarshalTraces(payload)
}

func isJSONPayload(payload []byte) bool {
	trimmed := bytes.TrimSpace(payload)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}
