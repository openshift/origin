package util

import (
	"reflect"
	"testing"

	"github.com/RangelReale/osin"
)

func TestGetOauthMetadata(t *testing.T) {
	actual := GetOauthMetadata("https://localhost:8443")
	expected := OauthAuthorizationServerMetadata{
		Issuer:                "https://localhost:8443",
		AuthorizationEndpoint: "https://localhost:8443/oauth/authorize",
		TokenEndpoint:         "https://localhost:8443/oauth/token",
		ScopesSupported: []string{
			"user:check-access",
			"user:full",
			"user:info",
			"user:list-projects",
			"user:list-scoped-projects",
		},
		ResponseTypesSupported: osin.AllowedAuthorizeType{
			"code",
			"token",
		},
		GrantTypesSupported: osin.AllowedAccessType{
			"authorization_code",
			"implicit",
		},
		CodeChallengeMethodsSupported: []string{
			"plain",
			"S256",
		},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Expected %#v, got %#v", expected, actual)
	}
}
