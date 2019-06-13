package openshiftkubeapiserver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog"

	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"
	"github.com/openshift/oauth-server/pkg/oauthserver"

	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	"github.com/openshift/origin/pkg/oauth/apis/oauth/validation"
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

func NewOAuthServerHandler(genericConfig *genericapiserver.Config, oauthConfig *osinv1.OAuthConfig) (http.Handler, map[string]genericapiserver.PostStartHookFunc, error) {
	if oauthConfig == nil {
		return http.NotFoundHandler(), nil, nil
	}

	config, err := NewOAuthServerConfigFromMasterConfig(genericConfig, oauthConfig)
	if err != nil {
		return nil, nil, err
	}
	oauthServer, err := config.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, nil, err
	}
	return oauthServer.GenericAPIServer.PrepareRun().GenericAPIServer.Handler.FullHandlerChain,
		map[string]genericapiserver.PostStartHookFunc{
			"oauth.openshift.io-startoauthclientsbootstrapping": config.StartOAuthClientsBootstrapping,
		},
		nil
}

func validateURL(urlString string) error {
	urlObj, err := url.Parse(urlString)
	if err != nil {
		return fmt.Errorf("%q is an invalid URL: %v", urlString, err)
	}
	if len(urlObj.Scheme) == 0 {
		return fmt.Errorf("must contain a valid scheme")
	}
	if len(urlObj.Host) == 0 {
		return fmt.Errorf("must contain a valid host")
	}
	return nil
}

func loadOAuthMetadataFile(metadataFile string) ([]byte, *oauthdiscovery.OauthAuthorizationServerMetadata, error) {
	data, err := ioutil.ReadFile(metadataFile)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to read External OAuth Metadata file: %v", err)
	}

	oauthMetadata := &oauthdiscovery.OauthAuthorizationServerMetadata{}
	if err := json.Unmarshal(data, oauthMetadata); err != nil {
		return nil, nil, fmt.Errorf("unable to decode External OAuth Metadata file: %v", err)
	}

	if err := validateURL(oauthMetadata.Issuer); err != nil {
		return nil, nil, fmt.Errorf("error validating External OAuth Metadata Issuer field: %v", err)
	}

	if err := validateURL(oauthMetadata.AuthorizationEndpoint); err != nil {
		return nil, nil, fmt.Errorf("error validating External OAuth Metadata AuthorizationEndpoint field: %v", err)
	}

	if err := validateURL(oauthMetadata.TokenEndpoint); err != nil {
		return nil, nil, fmt.Errorf("error validating External OAuth Metadata TokenEndpoint field: %v", err)
	}

	return data, oauthMetadata, nil
}

func getOauthMetadata(masterPublicURL string) oauthdiscovery.OauthAuthorizationServerMetadata {
	return oauthdiscovery.OauthAuthorizationServerMetadata{
		Issuer:                masterPublicURL,
		AuthorizationEndpoint: oauthdiscovery.OpenShiftOAuthAuthorizeURL(masterPublicURL),
		TokenEndpoint:         oauthdiscovery.OpenShiftOAuthTokenURL(masterPublicURL),
		// Note: this list is incomplete, which is allowed per the draft spec
		ScopesSupported:               scope.DefaultSupportedScopes(),
		ResponseTypesSupported:        []string{"code", "token"},
		GrantTypesSupported:           []string{"authorization_code", "implicit"},
		CodeChallengeMethodsSupported: validation.CodeChallengeMethodsSupported,
	}
}

func prepOauthMetadata(oauthConfig *osinv1.OAuthConfig, oauthMetadataFile string) ([]byte, *oauthdiscovery.OauthAuthorizationServerMetadata, error) {
	if len(oauthMetadataFile) > 0 {
		return loadOAuthMetadataFile(oauthMetadataFile)
	}
	if oauthConfig != nil && len(oauthConfig.MasterPublicURL) != 0 {
		metadataStruct := getOauthMetadata(oauthConfig.MasterPublicURL)
		metadata, err := json.MarshalIndent(metadataStruct, "", "  ")
		if err != nil {
			klog.Errorf("Unable to initialize OAuth authorization server metadata route: %v", err)
			return nil, nil, err
		}
		return metadata, &metadataStruct, nil
	}
	return nil, nil, nil
}
