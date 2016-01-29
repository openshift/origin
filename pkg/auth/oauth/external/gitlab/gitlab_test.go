package gitlab

import (
	"testing"

	"github.com/openshift/origin/pkg/auth/oauth/external"
)

func TestGitLab(t *testing.T) {
	p, err := NewProvider("gitlab", nil, "https://gitlab.com/", "clientid", "clientsecret")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	_ = external.Provider(p)
}
