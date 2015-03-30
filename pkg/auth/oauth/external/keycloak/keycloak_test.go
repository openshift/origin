package keycloak

import (
	"testing"

	"github.com/openshift/origin/pkg/auth/oauth/external"
)

func TestKeycloak(t *testing.T) {
	provider, _ := NewProviderFromBytes([]byte(""))
	_ = external.Provider(provider)
}
