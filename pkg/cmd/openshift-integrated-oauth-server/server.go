package openshift_integrated_oauth_server

import (
	"errors"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver/configprocessing"
	"github.com/openshift/origin/pkg/oauthserver/http2"
	"github.com/openshift/origin/pkg/oauthserver/oauthserver"

	// for metrics
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus"
)

func RunOsinServer(osinConfig *osinv1.OsinServerConfig, stopCh <-chan struct{}) error {
	if osinConfig == nil {
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

func newOAuthServerConfig(osinConfig *osinv1.OsinServerConfig) (*oauthserver.OAuthServerConfig, error) {
	genericConfig := genericapiserver.NewRecommendedConfig(legacyscheme.Codecs)

	servingOptions, err := configprocessing.ToServingOptions(osinConfig.ServingInfo)
	if err != nil {
		return nil, err
	}

	// explicitly override http2 max streams (HTTPServingInfo cannot configure this today)
	servingOptions.HTTP2MaxStreamsPerConnection = http2.MaxStreamsPerConnection

	if err := servingOptions.ApplyTo(&genericConfig.Config.SecureServing, &genericConfig.Config.LoopbackClientConfig); err != nil {
		return nil, err
	}

	// TODO You need real overrides for rate limiting
	kubeClientConfig, err := helpers.GetKubeConfigOrInClusterConfig(osinConfig.KubeClientConfig.KubeConfig, osinConfig.KubeClientConfig.ConnectionOverrides)
	if err != nil {
		return nil, err
	}

	oauthServerConfig, err := oauthserver.NewOAuthServerConfig(osinConfig.OAuthConfig, kubeClientConfig)
	if err != nil {
		return nil, err
	}

	// TODO you probably want to set this
	oauthServerConfig.GenericConfig.CorsAllowedOriginList = osinConfig.CORSAllowedOrigins
	oauthServerConfig.GenericConfig.SecureServing = genericConfig.SecureServing
	//oauthServerConfig.GenericConfig.AuditBackend = genericConfig.AuditBackend
	//oauthServerConfig.GenericConfig.AuditPolicyChecker = genericConfig.AuditPolicyChecker

	return oauthServerConfig, nil
}
