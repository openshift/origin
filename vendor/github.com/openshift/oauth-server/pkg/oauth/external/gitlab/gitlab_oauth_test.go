package gitlab

import (
	"reflect"
	"testing"

	"github.com/openshift/oauth-server/pkg/oauth/external"
)

func TestGitLab(t *testing.T) {
	p, err := NewOAuthProvider("gitlab", "https://gitlab.com/", "clientid", "clientsecret", nil)
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
