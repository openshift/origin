package openshiftkubeapiserver

import (
	"net/http"

	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/oauth-server/pkg/oauthserver"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

// TODO this is taking a very large config for a small piece of it.  The information must be broken up at some point so that
// we can run this in a pod.  This is an indication of leaky abstraction because it spent too much time in openshift start
func NewOAuthServerConfigFromMasterConfig(genericConfig *genericapiserver.Config, oauthConfig *osinv1.OAuthConfig) (*oauthserver.OAuthServerConfig, error) {
	oauthServerConfig, err := oauthserver.NewOAuthServerConfig(*oauthConfig, genericConfig.LoopbackClientConfig, nil)
	if err != nil {
		return nil, err
	}

	oauthServerConfig.GenericConfig.CorsAllowedOriginList = genericConfig.CorsAllowedOriginList
	oauthServerConfig.GenericConfig.SecureServing = genericConfig.SecureServing
	oauthServerConfig.GenericConfig.AuditBackend = genericConfig.AuditBackend
	oauthServerConfig.GenericConfig.AuditPolicyChecker = genericConfig.AuditPolicyChecker

	return oauthServerConfig, nil
}

func NewOAuthServerHandler(genericConfig *genericapiserver.Config, oauthConfig *osinv1.OAuthConfig) (http.Handler, error) {
	if oauthConfig == nil {
		return http.NotFoundHandler(), nil
	}

	config, err := NewOAuthServerConfigFromMasterConfig(genericConfig, oauthConfig)
	if err != nil {
		return nil, err
	}
	oauthServer, err := config.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}
	return oauthServer.GenericAPIServer.PrepareRun().GenericAPIServer.Handler.FullHandlerChain, nil
}
