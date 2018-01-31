package authenticator

import (
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/openshift/origin/pkg/oauthserver/api"
)

type Assertion interface {
	AuthenticateAssertion(assertionType, data string) (user.Info, bool, error)
}

type Client interface {
	AuthenticateClient(client api.Client) (user.Info, bool, error)
}
