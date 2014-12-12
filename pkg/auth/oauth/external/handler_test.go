package external

import (
	"testing"

	"github.com/openshift/origin/pkg/auth/oauth/handlers"
)

func TestHandler(t *testing.T) {
	_ = handlers.NewUnionAuthenticationHandler(nil, map[string]handlers.AuthenticationRedirector{"handler": &Handler{}}, nil)
}
