package math

import "golang.org/x/exp/constraints"

// Clamp returns the value `f` clamped to the range [low, high].
// It works for any numeric type (integers and floats).
func Clamp[T constraints.Ordered](f, low, high T) T {
	if f < low {
		return low
	}
	if f > high {
		return high
	}
	return f
}
