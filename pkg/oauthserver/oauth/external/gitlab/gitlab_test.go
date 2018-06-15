package gitlab

import (
	"reflect"
	"testing"

	"github.com/openshift/origin/pkg/oauthserver/oauth/external"
)

func TestGitLab(t *testing.T) {
	p, err := NewProvider("gitlab", nil, "https://gitlab.com/", "clientid", "clientsecret")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	_ = external.Provider(p)

	expectedProvider := &provider{
		providerName: "gitlab",
		authorizeURL: "https://gitlab.com/oauth/authorize",
		tokenURL:     "https://gitlab.com/oauth/token",
		userAPIURL:   "https://gitlab.com/api/v3/user",
		clientID:     "clientid",
		clientSecret: "clientsecret",
	}
	if !reflect.DeepEqual(p, expectedProvider) {
		t.Fatalf("Expected\n%#v\ngot\n%#v", expectedProvider, p)
	}
}
