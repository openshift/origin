package crypto

import (
	"crypto/rand"
	"encoding/base64"
)

// RandomBits returns a random byte slice with at least the requested bits of entropy.
// Callers should avoid using a value less than 256 unless they have a very good reason.
func RandomBits(bits int) []byte {
	size := bits / 8
	if bits%8 != 0 {
		size++
	}
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		panic(err) // rand should never fail
	}
	return b
}

// RandomBitsString returns a random string with at least the requested bits of entropy.
// It uses RawURLEncoding to ensure we do not get / characters or trailing ='s.
func RandomBitsString(bits int) string {
	return base64.RawURLEncoding.EncodeToString(RandomBits(bits))
}

// Random256BitsString is a convenience function for calling RandomBitsString(256).
// Callers that need a random string should use this function unless they have a
// very good reason to need a different amount of entropy.
func Random256BitsString() string {
	// 32 bytes (256 bits) = 43 base64-encoded characters
	return RandomBitsString(256)
}
