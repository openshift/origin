package denypassword

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/openshift/origin/pkg/auth/authenticator"
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
