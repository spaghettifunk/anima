package metadata

import (
	"crypto/rand"
	"encoding/binary"
	"hash/fnv"
)

func GetAlignedRange(offset, size, granularity uint64) *MemoryRange {
	m := &MemoryRange{
		Offset: getAligned(offset, granularity),
		Size:   getAligned(size, granularity),
	}
	return m
}

func getAligned(operand, granularity uint64) uint64 {
	return (operand + (granularity - 1)) & ^(granularity - 1)
}

// Generate a fast FNV-1a hash as a string
func GenerateNewHash() string {
	input := generateRandomString()
	hasher := fnv.New64a() // Create a FNV-1a 64-bit hash
	hasher.Write([]byte(input))
	return string(hasher.Sum(nil))
}

// Generate a random string of a specified length using bitshifting
func generateRandomString() string {
	const length = 8
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var result = make([]byte, length)
	var buf [8]byte
	for i := 0; i < length; i += 8 {
		// Read random bytes from crypto/rand for secure randomness
		_, _ = rand.Read(buf[:])
		r := binary.LittleEndian.Uint64(buf[:])
		// Convert random bits to characters
		for j := 0; j < 8 && i+j < length; j++ {
			result[i+j] = letters[r&63] // Use the lower 6 bits (64 possible values)
			r >>= 6                     // Shift right by 6 bits for the next character
		}
	}
	return string(result)
}
