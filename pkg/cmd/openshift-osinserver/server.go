package openshift_osinserver

import (
	"k8s.io/client-go/rest"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/oauthserver/oauthserver"
	genericapiserver "k8s.io/apiserver/pkg/server"

	// for metrics
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus"
)

func RunOpenShiftOsinServer(oauthConfig configapi.OAuthConfig, kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	oauthServerConfig, err := oauthserver.NewOAuthServerConfigFromInternal(oauthConfig, kubeClientConfig)
	if err != nil {
		return err
	}

	// TODO you probably want to set this
	//oauthServerConfig.GenericConfig.CorsAllowedOriginList = genericConfig.CorsAllowedOriginList
	//oauthServerConfig.GenericConfig.SecureServing = genericConfig.SecureServing
	//oauthServerConfig.GenericConfig.AuditBackend = genericConfig.AuditBackend
	//oauthServerConfig.GenericConfig.AuditPolicyChecker = genericConfig.AuditPolicyChecker

	// Build the list of valid redirect_uri prefixes for a login using the openshift-web-console client to redirect to
	oauthServerConfig.ExtraOAuthConfig.AssetPublicAddresses = []string{oauthConfig.AssetPublicURL}

	oauthServer, err := oauthServerConfig.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return err
	}

	return oauthServer.GenericAPIServer.PrepareRun().Run(stopCh)

}
