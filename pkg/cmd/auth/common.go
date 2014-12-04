package auth

import (
	"github.com/openshift/origin/pkg/auth/server/csrf"
)

const (
	OpenShiftOAuthCallbackPrefix = "/oauth2callback"
	OpenShiftLoginPrefix         = "/login"
	OpenShiftApprovePrefix       = "/oauth/approve"
)

// GetCSRF returns the object responsible for generating and checking CSRF tokens
func GetCSRF() csrf.CSRF {
	return csrf.NewCookieCSRF("csrf", "/", "", false, false)
}
