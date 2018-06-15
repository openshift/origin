// +build windows

package user

import (
	"errors"
	"io"
)

var notSupported = errors.New("not supported in this build")

func GetPasswdPath() (string, error) {
	return "", notSupported
}

func GetPasswd() (io.ReadCloser, error) {
	return nil, notSupported
}

func GetGroupPath() (string, error) {
	return "", notSupported
}

func GetGroup() (io.ReadCloser, error) {
	return nil, notSupported
}

func CurrentUser() (User, error) {
	return User{}, notSupported
}

func CurrentGroup() (Group, error) {
	return Group{}, notSupported
}

