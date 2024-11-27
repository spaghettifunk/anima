package metadata

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"hash/fnv"
)

func GetAlignedRange(offset, size, granularity uint64) *MemoryRange {
	m := &MemoryRange{
		Offset: GetAligned(offset, granularity),
		Size:   GetAligned(size, granularity),
	}
	return m
}

func GetAligned(operand, granularity uint64) uint64 {
	val := (operand + (granularity - 1)) &^ (granularity - 1)
	return val
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
			result[i+j] = letters[r&61] // Use the lower 6 bits (64 possible values)
			r >>= 6                     // Shift right by 6 bits for the next character
		}
	}
	return string(result)
}

func BytesToCodepoint(bytes string, offset uint32) (int32, uint8, error) {
	var out_codepoint int32
	var out_advance uint8

	codepoint := int32(bytes[offset])
	if codepoint >= 0 && codepoint < 0x7F {
		// Normal single-byte ascii character.
		out_advance = 1
		out_codepoint = codepoint
		return out_codepoint, out_advance, nil
	} else if (codepoint & 0xE0) == 0xC0 {
		// Double-byte character
		codepoint = int32(((bytes[offset+0] & 0b00011111) << 6) +
			(bytes[offset+1] & 0b00111111))
		out_advance = 2
		out_codepoint = codepoint
		return out_codepoint, out_advance, nil
	} else if (codepoint & 0xF0) == 0xE0 {
		// Triple-byte character
		codepoint = int32(((bytes[offset+0] & 0b00001111) << 12) +
			((bytes[offset+1] & 0b00111111) << 6) +
			(bytes[offset+2] & 0b00111111))
		out_advance = 3
		out_codepoint = codepoint
		return out_codepoint, out_advance, nil
	} else if (codepoint & 0xF8) == 0xF0 {
		// 4-byte character
		codepoint = int32(((bytes[offset+0] & 0b00000111) << 18) +
			((bytes[offset+1] & 0b00111111) << 12) +
			((bytes[offset+2] & 0b00111111) << 6) +
			(bytes[offset+3] & 0b00111111))
		out_advance = 4
		out_codepoint = codepoint
		return out_codepoint, out_advance, nil
	} else {
		// NOTE: Not supporting 5 and 6-byte characters; return as invalid UTF-8.
		out_advance = 0
		out_codepoint = 0
		err := fmt.Errorf("string bytes_to_codepoint() - Not supporting 5 and 6-byte characters; Invalid UTF-8")
		return out_codepoint, out_advance, err
	}
}
