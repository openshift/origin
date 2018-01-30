package github

import (
	"testing"

	"github.com/openshift/origin/pkg/oauthserver/oauth/external"
)

func TestGitHub(t *testing.T) {
	_ = external.Provider(NewProvider("github", "clientid", "clientsecret", nil, nil))
}
