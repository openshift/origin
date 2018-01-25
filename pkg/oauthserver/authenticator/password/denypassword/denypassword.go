package denypassword

import (
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"
)

// denyPasswordAuthenticator denies all password requests
type denyPasswordAuthenticator struct {
}

// New creates a new password authenticator that denies any login attempt
func New() authenticator.Password {
	return &denyPasswordAuthenticator{}
}

// AuthenticatePassword denies any login attempt
func (a denyPasswordAuthenticator) AuthenticatePassword(username, password string) (user.Info, bool, error) {
	return nil, false, nil
}
