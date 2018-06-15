package google

import (
	"testing"

	"github.com/openshift/origin/pkg/oauthserver/oauth/external"
)

func TestGoogle(t *testing.T) {
	p, err := NewProvider("google", "clientid", "clientsecret", "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	_ = external.Provider(p)
}
