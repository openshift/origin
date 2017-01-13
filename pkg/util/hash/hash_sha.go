package hash

import (
	"crypto/sha256"
	"encoding/base64"
)

func NewSHA256Hasher() Hasher {
	return &sha256Hasher{}
}

// SHA256Size holds the encoded size of a sha256 hash
// ceiling(32 bytes * 4/3 characters per byte)
const SHA256Size = 43

type sha256Hasher struct{}

func (h *sha256Hasher) Hash(plaintext string) string {
	hashed := sha256.Sum256([]byte(plaintext))
	encoded := base64.RawURLEncoding.EncodeToString(hashed[:])
	return encoded
}

func (h *sha256Hasher) VerifyHash(plaintext, hash string) bool {
	if !h.VerifyHashFormat(hash) {
		return false
	}
	return h.Hash(plaintext) == hash
}

func (h *sha256Hasher) VerifyHashFormat(hash string) bool {
	if len(hash) != SHA256Size {
		return false
	}
	// TODO: validate alphabet
	return true
}
