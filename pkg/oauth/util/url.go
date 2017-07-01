package util

import (
	"path"

	"github.com/openshift/origin/pkg/auth/server/tokenrequest"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
)

const (
	OpenShiftOAuthAPIPrefix = "/oauth"
)

func OpenShiftOAuthAuthorizeURL(masterAddr string) string {
	return masterAddr + path.Join(OpenShiftOAuthAPIPrefix, osinserver.AuthorizePath)
}
func OpenShiftOAuthTokenURL(masterAddr string) string {
	return masterAddr + path.Join(OpenShiftOAuthAPIPrefix, osinserver.TokenPath)
}
func OpenShiftOAuthTokenRequestURL(masterAddr string) string {
	return masterAddr + path.Join(OpenShiftOAuthAPIPrefix, tokenrequest.RequestTokenEndpoint)
}
