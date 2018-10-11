package openshift_osinserver

import (
	"errors"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver/configprocessing"
	"github.com/openshift/origin/pkg/oauthserver/oauthserver"

	// for metrics
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus"
)

func RunOpenShiftOsinServer(osinConfig *kubecontrolplanev1.KubeAPIServerConfig, stopCh <-chan struct{}) error {
	if osinConfig == nil || osinConfig.OAuthConfig == nil {
		return errors.New("osin server requires non-empty oauthConfig")
	}

	oauthServerConfig, err := newOAuthServerConfig(osinConfig)
	if err != nil {
		return err
	}

	oauthServer, err := oauthServerConfig.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return err
	}

	oauthServer.GenericAPIServer.AddPostStartHookOrDie("oauth.openshift.io-startoauthclientsbootstrapping", oauthServerConfig.StartOAuthClientsBootstrapping)

	return oauthServer.GenericAPIServer.PrepareRun().Run(stopCh)
}

func newOAuthServerConfig(osinConfig *kubecontrolplanev1.KubeAPIServerConfig) (*oauthserver.OAuthServerConfig, error) {
	genericConfig := genericapiserver.NewRecommendedConfig(legacyscheme.Codecs)

	servingOptions, err := configprocessing.ToServingOptions(osinConfig.ServingInfo)
	if err != nil {
		return nil, err
	}
	if err := servingOptions.ApplyTo(&genericConfig.Config); err != nil {
		return nil, err
	}

	// TODO You need real overrides for rate limiting
	kubeClientConfig, err := helpers.GetKubeConfigOrInClusterConfig(osinConfig.KubeClientConfig.KubeConfig, osinConfig.KubeClientConfig.ConnectionOverrides)
	if err != nil {
		return nil, err
	}

	oauthServerConfig, err := oauthserver.NewOAuthServerConfig(*osinConfig.OAuthConfig, kubeClientConfig)
	if err != nil {
		return nil, err
	}

	// TODO you probably want to set this
	//oauthServerConfig.GenericConfig.CorsAllowedOriginList = genericConfig.CorsAllowedOriginList
	oauthServerConfig.GenericConfig.SecureServing = genericConfig.SecureServing
	//oauthServerConfig.GenericConfig.AuditBackend = genericConfig.AuditBackend
	//oauthServerConfig.GenericConfig.AuditPolicyChecker = genericConfig.AuditPolicyChecker

	// Build the list of valid redirect_uri prefixes for a login using the openshift-web-console client to redirect to
	oauthServerConfig.ExtraOAuthConfig.AssetPublicAddresses = []string{osinConfig.OAuthConfig.AssetPublicURL}

	return oauthServerConfig, nil
}
