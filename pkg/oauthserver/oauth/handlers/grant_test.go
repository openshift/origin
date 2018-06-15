package handlers

import (
	"testing"

	"github.com/openshift/origin/pkg/oauthserver/osinserver"
)

func TestGrant(t *testing.T) {
	_ = osinserver.AuthorizeHandler(&GrantCheck{})
}

func TestEmptyGrant(t *testing.T) {
	_ = NewEmptyGrant()
}

func TestAutoGrant(t *testing.T) {
	_ = NewAutoGrant()
}

func TestRedirectGrant(t *testing.T) {
	_ = NewRedirectGrant("/")
}
