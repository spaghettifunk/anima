package core

import "sync"

const AVG_COUNT uint8 = 30

type MetricsState struct {
	FrameAVGCounter    uint8
	MStimes            [AVG_COUNT]float64
	MSavg              float64
	Frames             int32
	AccumulatedFrameMS float64
	FPS                float64
}

var onceMetrics sync.Once
var metricsState *MetricsState = nil

func MetricsInitialize() error {
	onceMetrics.Do(func() {
		metricsState = &MetricsState{
			MStimes: [AVG_COUNT]float64{0},
		}
	})
	return nil
}

func MetricsUpdate(frame_elapsed_time float64) {
	// Calculate frame ms average
	frame_ms := (frame_elapsed_time * 1000.0)
	metricsState.MStimes[metricsState.FrameAVGCounter] = frame_ms
	if metricsState.FrameAVGCounter == AVG_COUNT-1 {
		for i := uint8(0); i < AVG_COUNT; i++ {
			metricsState.MSavg += metricsState.MStimes[i]
		}

		metricsState.MSavg /= float64(AVG_COUNT)
	}
	metricsState.FrameAVGCounter++
	metricsState.FrameAVGCounter %= AVG_COUNT

	// Calculate Frames per second.
	metricsState.AccumulatedFrameMS += frame_ms
	if metricsState.AccumulatedFrameMS > 1000 {
		metricsState.FPS = float64(metricsState.Frames)
		metricsState.AccumulatedFrameMS -= 1000
		metricsState.Frames = 0
	}

	// Count all Frames.
	metricsState.Frames++
}

func MetricsFPS() float64 {
	return metricsState.FPS
}

func MetricsFrameTime() float64 {
	return metricsState.MSavg
}

func MetricsFrame() (float64, float64) {
	return metricsState.FPS, metricsState.MSavg
}
