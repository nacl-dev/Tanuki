package dedup

import "math/bits"

// HammingDistance returns the number of differing bits between two 64-bit hashes.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// IsDuplicate returns true when the Hamming distance between a and b is within
// the given threshold (inclusive).
func IsDuplicate(a, b uint64, threshold int) bool {
	return HammingDistance(a, b) <= threshold
}

// Similarity converts a Hamming distance to a percentage similarity score (0–100).
func Similarity(distance int) float64 {
	// 64 bits total; 0 distance = 100%, 64 distance = 0%
	return 100.0 * float64(64-distance) / 64.0
}
