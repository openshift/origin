package openid

import (
	"testing"

	"github.com/openshift/origin/pkg/auth/oauth/external"
)

func TestOpenID(t *testing.T) {
	p, err := NewProvider("openid", nil, Config{
		ClientID:     "foo",
		ClientSecret: "secret",
		AuthorizeURL: "https://foo",
		TokenURL:     "https://foo",
		Scopes:       []string{"openid"},
		IDClaims:     []string{"sub"},
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	_ = external.Provider(p)

}
