package utils

func Pointer[T any](d T) *T {
	return &d
}

func Contains[T comparable](arr []T, x T) bool {
	for _, v := range arr {
		if v == x {
			return true
		}
	}
	return false
}

func PositiveOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
