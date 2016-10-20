package discovery

import (
	"reflect"
	"testing"

	"github.com/RangelReale/osin"
)

func TestGet(t *testing.T) {
	actual := Get("https://localhost:8443", "https://localhost:8443/oauth/authorize", "https://localhost:8443/oauth/token")
	expected := OauthAuthorizationServerMetadata{
		Issuer:                "https://localhost:8443",
		AuthorizationEndpoint: "https://localhost:8443/oauth/authorize",
		TokenEndpoint:         "https://localhost:8443/oauth/token",
		ScopesSupported: []string{
			"user:full",
			"user:info",
			"user:check-access",
			"user:list-scoped-projects",
			"user:list-projects",
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
