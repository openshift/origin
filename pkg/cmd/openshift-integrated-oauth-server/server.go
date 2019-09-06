package openshift_integrated_oauth_server

import (
	"errors"
	"net/http"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authentication/request/union"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/path"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericapiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver/configprocessing"
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
	if err := servingOptions.ApplyTo(&genericConfig.Config.SecureServing, &genericConfig.Config.LoopbackClientConfig); err != nil {
		return nil, err
	}
	// the oauth-server must only run in http1 to avoid http2 connection re-use problems when improperly re-using a wildcard certificate
	genericConfig.Config.SecureServing.HTTP1Only = true

	authenticationOptions := genericapiserveroptions.NewDelegatingAuthenticationOptions()
	authenticationOptions.ClientCert.ClientCA = osinConfig.ServingInfo.ClientCA
	authenticationOptions.RemoteKubeConfigFile = osinConfig.KubeClientConfig.KubeConfig
	if err := authenticationOptions.ApplyTo(&genericConfig.Authentication, genericConfig.SecureServing, genericConfig.OpenAPIConfig); err != nil {
		return nil, err
	}

	// These are paths for which we bypass kube authentication/authorization
	// TODO better formalize / generate this list as trailing * matters
	alwaysAllowedPaths := []string{ // The five sections are:
		"/healthz", "/healthz/", // 1. Health checks (root, no wildcard)
		"/oauth/*",           // 2. OAuth (wildcard)
		"/login", "/login/*", // 3. Login (both root and wildcard)
		"/logout", "/logout/", // 4. Logout (root, no wildcard)
		"/oauth2callback/*", // 5. OAuth callbacks (wildcard)
	}

	authorizationOptions := genericapiserveroptions.NewDelegatingAuthorizationOptions().
		WithAlwaysAllowPaths(alwaysAllowedPaths...).
		WithAlwaysAllowGroups(user.SystemPrivilegedGroup)
	authorizationOptions.RemoteKubeConfigFile = osinConfig.KubeClientConfig.KubeConfig
	if err := authorizationOptions.ApplyTo(&genericConfig.Authorization); err != nil {
		return nil, err
	}

	// set up a path authorizer so that we can check allowed paths in the authenticator below
	pathAuthorizer, err := path.NewAuthorizer(alwaysAllowedPaths)
	if err != nil {
		return nil, err
	}

	anonymousAuthenticator := anonymous.NewAuthenticator()
	genericConfig.Authentication.Authenticator = union.New(
		genericConfig.Authentication.Authenticator,
		authenticator.RequestFunc(func(req *http.Request) (*authenticator.Response, bool, error) {
			// standard authenticator errored, but if the request's path is one of the always-allowed ones
			// we will just carry on as anonymous
			authAttributes := authorizer.AttributesRecord{
				Path: req.URL.Path,
			}

			// if the path authorizer would allow anybody, we don't care about previous authentication result
			if decision, _, _ := pathAuthorizer.Authorize(authAttributes); decision == authorizer.DecisionAllow {
				return anonymousAuthenticator.AuthenticateRequest(req)
			}

			return nil, false, nil
		}),
	)

	// TODO You need real overrides for rate limiting
	kubeClientConfig, err := helpers.GetKubeConfigOrInClusterConfig(osinConfig.KubeClientConfig.KubeConfig, osinConfig.KubeClientConfig.ConnectionOverrides)
	if err != nil {
		return nil, err
	}

	oauthServerConfig, err := oauthserver.NewOAuthServerConfig(osinConfig.OAuthConfig, kubeClientConfig, genericConfig)
	if err != nil {
		return nil, err
	}

	// TODO you probably want to set this
	oauthServerConfig.GenericConfig.CorsAllowedOriginList = osinConfig.CORSAllowedOrigins
	//oauthServerConfig.GenericConfig.AuditBackend = genericConfig.AuditBackend
	//oauthServerConfig.GenericConfig.AuditPolicyChecker = genericConfig.AuditPolicyChecker

	return oauthServerConfig, nil
}
