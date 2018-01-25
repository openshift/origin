package util

import (
	"path"
	"strings"

	"github.com/openshift/origin/pkg/oauthserver/osinserver"
	"github.com/openshift/origin/pkg/oauthserver/server/tokenrequest"
)

const OpenShiftOAuthAPIPrefix = "/oauth"

func OpenShiftOAuthAuthorizeURL(masterAddr string) string {
	return openShiftOAuthURL(masterAddr, osinserver.AuthorizePath)
}
func OpenShiftOAuthTokenURL(masterAddr string) string {
	return openShiftOAuthURL(masterAddr, osinserver.TokenPath)
}
func OpenShiftOAuthTokenRequestURL(masterAddr string) string {
	return openShiftOAuthURL(masterAddr, tokenrequest.RequestTokenEndpoint)
}
func OpenShiftOAuthTokenDisplayURL(masterAddr string) string {
	return openShiftOAuthURL(masterAddr, tokenrequest.DisplayTokenEndpoint)
}
func OpenShiftOAuthTokenImplicitURL(masterAddr string) string {
	return openShiftOAuthURL(masterAddr, tokenrequest.ImplicitTokenEndpoint)
}
func openShiftOAuthURL(masterAddr, oauthEndpoint string) string {
	return strings.TrimRight(masterAddr, "/") + path.Join(OpenShiftOAuthAPIPrefix, oauthEndpoint)
}
