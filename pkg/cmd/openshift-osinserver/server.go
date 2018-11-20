package openshift_osinserver

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	routeclient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
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

	routeClient, err := routeclient.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	oauthServerConfig.ExtraOAuthConfig.RouteClient = routeClient
	oauthServerConfig.ExtraOAuthConfig.KubeClient = kubeClient

	// Build the list of valid redirect_uri prefixes for a login using the openshift-web-console client to redirect to
	oauthServerConfig.ExtraOAuthConfig.AssetPublicAddresses = []string{oauthConfig.AssetPublicURL}

	oauthServer, err := oauthServerConfig.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return err
	}

	return oauthServer.GenericAPIServer.PrepareRun().Run(stopCh)

}
