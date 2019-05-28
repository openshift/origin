package authenticator

import (
	"k8s.io/apiserver/pkg/authentication/authenticator"

	"github.com/openshift/oauth-server/pkg/api"
)

type Assertion interface {
	AuthenticateAssertion(assertionType, data string) (*authenticator.Response, bool, error)
}

type Client interface {
	AuthenticateClient(client api.Client) (*authenticator.Response, bool, error)
}
