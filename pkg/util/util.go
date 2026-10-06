package util

import (
	"math/rand"
	"os"
)

// Write writes content to filename, returning any error to the caller.
func Write(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

// RandomInt returns a pseudo-random int in [min, max).
// The global math/rand source is auto-seeded (Go 1.20+); do NOT reseed per call.
func RandomInt(min, max int) int {
	if max <= min {
		return min
	}
	return rand.Intn(max-min) + min
}
