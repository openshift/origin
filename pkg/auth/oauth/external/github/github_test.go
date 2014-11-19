package github

import (
	"testing"

	"github.com/openshift/origin/pkg/auth/oauth/external"
)

func TestGithub(t *testing.T) {
	_ = external.Provider(NewProvider("", ""))
}
