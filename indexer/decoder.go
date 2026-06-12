package indexer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

func DecodeObservations(payload []byte, kafkaTimestamp time.Time) ([]MetricObservation, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty payload")
	}

	var observations []MetricObservation
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &observations); err != nil {
			return nil, err
		}
	} else {
		var observation MetricObservation
		if err := json.Unmarshal(trimmed, &observation); err != nil {
			return nil, err
		}
		observations = []MetricObservation{observation}
	}

	out := make([]MetricObservation, 0, len(observations))
	for i, observation := range observations {
		normalized, err := observation.Normalize(kafkaTimestamp)
		if err != nil {
			return nil, fmt.Errorf("observation %d: %w", i, err)
		}
		out = append(out, normalized)
	}

	return out, nil
}
