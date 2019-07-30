package openshiftkubeapiserver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"k8s.io/klog"

	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/apiserver-library-go/pkg/authorization/scope"
	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"
)

// PKCE [RFC7636] code challenge methods supported
// https://tools.ietf.org/html/rfc7636#section-4.3
const (
	codeChallengeMethodPlain  = "plain"
	codeChallengeMethodSHA256 = "S256"
)

var codeChallengeMethodsSupported = []string{codeChallengeMethodPlain, codeChallengeMethodSHA256}

// TODO: promote this struct as it is not effectively part of our API, since we
// validate configuration using LoadOAuthMetadataFile

func getOauthMetadata(masterPublicURL string) oauthdiscovery.OauthAuthorizationServerMetadata {
	return oauthdiscovery.OauthAuthorizationServerMetadata{
		Issuer:                masterPublicURL,
		AuthorizationEndpoint: oauthdiscovery.OpenShiftOAuthAuthorizeURL(masterPublicURL),
		TokenEndpoint:         oauthdiscovery.OpenShiftOAuthTokenURL(masterPublicURL),
		// Note: this list is incomplete, which is allowed per the draft spec
		ScopesSupported:               scope.DefaultSupportedScopes(),
		ResponseTypesSupported:        []string{"code", "token"},
		GrantTypesSupported:           []string{"authorization_code", "implicit"},
		CodeChallengeMethodsSupported: codeChallengeMethodsSupported,
	}
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
