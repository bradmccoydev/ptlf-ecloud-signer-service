package sigstore

import (
	"crypto/sha256"
)

// hashForECDSA computes a SHA-256 hash of the input, suitable for ECDSA signing.
func hashForECDSA(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}
