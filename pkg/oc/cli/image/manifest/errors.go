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

func (e *imageNotFound) Error() string {
	return e.msg
}

type imageForbidden struct {
	msg string
	err error
}

func NewImageForbidden(msg string, err error) error {
	return &imageForbidden{msg: msg, err: err}
}

func (e *imageForbidden) Error() string {
	return e.msg
}

func IsImageForbidden(err error) bool {
	switch t := err.(type) {
	case errcode.Errors:
		for _, err := range t {
			if IsImageForbidden(err) {
				return true
			}
		}
		return false
	case errcode.Error:
		return t.Code == errcode.ErrorCodeDenied
	case *imageForbidden:
		return true
	default:
		return false
	}
}
func IsImageNotFound(err error) bool {
	switch t := err.(type) {
	case errcode.Errors:
		for _, err := range t {
			if IsImageNotFound(err) {
				return true
			}
		}
		return false
	case errcode.Error:
		return t.Code == registryapiv2.ErrorCodeManifestUnknown
	case *imageNotFound:
		return true
	case *imageForbidden:
		return true
	default:
		return false
	}
}
