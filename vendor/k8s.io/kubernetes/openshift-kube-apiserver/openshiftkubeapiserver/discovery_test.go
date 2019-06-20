package openshiftkubeapiserver

import (
	"reflect"
	"testing"

	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"
)

func TestGetOauthMetadata(t *testing.T) {
	actual := getOauthMetadata("https://localhost:8443")
	expected := oauthdiscovery.OauthAuthorizationServerMetadata{
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
		ResponseTypesSupported: []string{
			"code",
			"token",
		},
		GrantTypesSupported: []string{
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
