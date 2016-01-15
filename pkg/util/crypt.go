// +build !cgo !linux

package util

import (
	"errors"
)

func Crypt(key string, salt string) (string, error) {
	// We use the GNU reentrant form of crypt() ...
	return "", errors.New("crypt() password hashes are not supported on this platform")
}
