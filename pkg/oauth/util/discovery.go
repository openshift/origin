package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/golang/glog"

	"github.com/RangelReale/osin"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/oauth/apis/oauth/validation"
	"github.com/openshift/origin/pkg/oauth/urls"
)

// OauthAuthorizationServerMetadata holds OAuth 2.0 Authorization Server Metadata used for discovery
// https://tools.ietf.org/html/draft-ietf-oauth-discovery-04#section-2
type OauthAuthorizationServerMetadata struct {
	// The authorization server's issuer identifier, which is a URL that uses the https scheme and has no query or fragment components.
	// This is the location where .well-known RFC 5785 [RFC5785] resources containing information about the authorization server are published.
	Issuer string `json:"issuer"`

	// URL of the authorization server's authorization endpoint [RFC6749].
	AuthorizationEndpoint string `json:"authorization_endpoint"`

	// URL of the authorization server's token endpoint [RFC6749].
	TokenEndpoint string `json:"token_endpoint"`

	// JSON array containing a list of the OAuth 2.0 [RFC6749] scope values that this authorization server supports.
	// Servers MAY choose not to advertise some supported scope values even when this parameter is used.
	ScopesSupported []string `json:"scopes_supported"`

	// JSON array containing a list of the OAuth 2.0 response_type values that this authorization server supports.
	// The array values used are the same as those used with the response_types parameter defined by "OAuth 2.0 Dynamic Client Registration Protocol" [RFC7591].
	ResponseTypesSupported osin.AllowedAuthorizeType `json:"response_types_supported"`

	// JSON array containing a list of the OAuth 2.0 grant type values that this authorization server supports.
	// The array values used are the same as those used with the grant_types parameter defined by "OAuth 2.0 Dynamic Client Registration Protocol" [RFC7591].
	GrantTypesSupported osin.AllowedAccessType `json:"grant_types_supported"`

	// JSON array containing a list of PKCE [RFC7636] code challenge methods supported by this authorization server.
	// Code challenge method values are used in the "code_challenge_method" parameter defined in Section 4.3 of [RFC7636].
	// The valid code challenge method values are those registered in the IANA "PKCE Code Challenge Methods" registry [IANA.OAuth.Parameters].
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
}

// TODO: promote this struct as it is not effectively part of our API, since we
// validate configuration using LoadOAuthMetadataFile

func getOauthMetadata(masterPublicURL string) OauthAuthorizationServerMetadata {
	return OauthAuthorizationServerMetadata{
		Issuer:                masterPublicURL,
		AuthorizationEndpoint: urls.OpenShiftOAuthAuthorizeURL(masterPublicURL),
		TokenEndpoint:         urls.OpenShiftOAuthTokenURL(masterPublicURL),
		// Note: this list is incomplete, which is allowed per the draft spec
		ScopesSupported:               scope.DefaultSupportedScopes(),
		ResponseTypesSupported:        osin.AllowedAuthorizeType{osin.CODE, osin.TOKEN},
		GrantTypesSupported:           osin.AllowedAccessType{osin.AUTHORIZATION_CODE, osin.AccessRequestType("implicit")},
		CodeChallengeMethodsSupported: validation.CodeChallengeMethodsSupported,
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

func LoadOAuthMetadataFile(metadataFile string) ([]byte, *OauthAuthorizationServerMetadata, error) {
	data, err := ioutil.ReadFile(metadataFile)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to read External OAuth Metadata file: %v", err)
	}

	oauthMetadata := &OauthAuthorizationServerMetadata{}
	if err := json.Unmarshal(data, oauthMetadata); err != nil {
		return nil, nil, fmt.Errorf("Unable to decode External OAuth Metadata file: %v", err)
	}

	if err := validateURL(oauthMetadata.Issuer); err != nil {
		return nil, nil, fmt.Errorf("Error validating External OAuth Metadata Issuer field: %v", err)
	}

	if err := validateURL(oauthMetadata.AuthorizationEndpoint); err != nil {
		return nil, nil, fmt.Errorf("Error validating External OAuth Metadata AuthorizationEndpoint field: %v", err)
	}

	if err := validateURL(oauthMetadata.TokenEndpoint); err != nil {
		return nil, nil, fmt.Errorf("Error validating External OAuth Metadata TokenEndpoint field: %v", err)
	}

	return data, oauthMetadata, nil
}

func PrepOauthMetadata(oauthConfig *configapi.OAuthConfig, oauthMetadataFile string) ([]byte, *OauthAuthorizationServerMetadata, error) {
	if oauthConfig != nil {
		metadataStruct := getOauthMetadata(oauthConfig.MasterPublicURL)
		metadata, err := json.MarshalIndent(metadataStruct, "", "  ")
		if err != nil {
			glog.Errorf("Unable to initialize OAuth authorization server metadata route: %v", err)
			return nil, nil, err
		}
		return metadata, &metadataStruct, nil
	}
	if len(oauthMetadataFile) > 0 {
		return LoadOAuthMetadataFile(oauthMetadataFile)
	}
	return nil, nil, nil
}
