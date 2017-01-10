package hash

import (
	"encoding/base64"
	"fmt"
	"io"
)

func NewRander(rand io.Reader) Rander {
	return &rander{rand}
}

type rander struct {
	rand io.Reader
}

func (r *rander) Rand(length int) (string, error) {
	randomBytes := make([]byte, length)
	read, err := r.rand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	if read != length {
		return "", fmt.Errorf("could not read %d random bytes (got %d)", length, read)
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}
