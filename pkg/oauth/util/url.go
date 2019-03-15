package util

import (
	"path"
	"strings"

	"github.com/openshift/origin/pkg/oauthserver/osinserver"
)

const (
	OpenShiftOAuthAPIPrefix = "/oauth"
	RequestTokenEndpoint    = "/token/request"
	DisplayTokenEndpoint    = "/token/display"
	ImplicitTokenEndpoint   = "/token/implicit"
)

func OpenShiftOAuthAuthorizeURL(masterAddr string) string {
	return openShiftOAuthURL(masterAddr, osinserver.AuthorizePath)
}
func OpenShiftOAuthTokenURL(masterAddr string) string {
	return openShiftOAuthURL(masterAddr, osinserver.TokenPath)
}
func OpenShiftOAuthTokenRequestURL(masterAddr string) string {
	return openShiftOAuthURL(masterAddr, RequestTokenEndpoint)
}
func OpenShiftOAuthTokenDisplayURL(masterAddr string) string {
	return openShiftOAuthURL(masterAddr, DisplayTokenEndpoint)
}
func OpenShiftOAuthTokenImplicitURL(masterAddr string) string {
	return openShiftOAuthURL(masterAddr, ImplicitTokenEndpoint)
}
func openShiftOAuthURL(masterAddr, oauthEndpoint string) string {
	return strings.TrimRight(masterAddr, "/") + path.Join(OpenShiftOAuthAPIPrefix, oauthEndpoint)
}
