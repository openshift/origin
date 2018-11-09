package manifest

import (
	"github.com/docker/distribution/registry/api/errcode"
	registryapiv2 "github.com/docker/distribution/registry/api/v2"
)

type imageNotFound struct {
	msg string
	err error
}

func NewImageNotFound(msg string, err error) error {
	return &imageNotFound{msg: msg, err: err}
}

func IsImageNotFound(err error) bool {
	switch t := err.(type) {
	case errcode.Errors:
		return len(t) == 1 && IsImageNotFound(t[0])
	case errcode.Error:
		return t.Code == registryapiv2.ErrorCodeManifestUnknown
	case *imageNotFound:
		return true
	default:
		return false
	}
}

func (e *imageNotFound) Error() string {
	return e.msg
}
