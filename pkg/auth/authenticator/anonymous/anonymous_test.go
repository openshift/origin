package anonymous

import (
	"testing"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"

	"k8s.io/kubernetes/pkg/auth/authenticator"
	"k8s.io/kubernetes/pkg/util/sets"
)

func TestAnonymous(t *testing.T) {
	var a authenticator.Request = NewAuthenticator()
	u, ok, err := a.AuthenticateRequest(nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if !ok {
		t.Fatalf("Unexpectedly unauthenticated")
	}
	if u.GetName() != bootstrappolicy.UnauthenticatedUsername {
		t.Fatalf("Expected username %s, got %s", bootstrappolicy.UnauthenticatedUsername, u.GetName())
	}
	if !sets.NewString(u.GetGroups()...).Equal(sets.NewString(bootstrappolicy.UnauthenticatedGroup)) {
		t.Fatalf("Expected group %s, got %v", bootstrappolicy.UnauthenticatedGroup, u.GetGroups())
	}
}
