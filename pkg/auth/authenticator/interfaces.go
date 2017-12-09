package authenticator

import (
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/oauth/apis/oauth"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

type Assertion interface {
	AuthenticateAssertion(assertionType, data string) (user.Info, bool, error)
}

type Client interface {
	AuthenticateClient(client api.Client) (user.Info, bool, error)
}

type OAuthTokenValidator interface {
	Validate(token *oauth.OAuthAccessToken, user *userapi.User) error
}

var _ OAuthTokenValidator = OAuthTokenValidatorFunc(nil)

type OAuthTokenValidatorFunc func(token *oauth.OAuthAccessToken, user *userapi.User) error

func (f OAuthTokenValidatorFunc) Validate(token *oauth.OAuthAccessToken, user *userapi.User) error {
	return f(token, user)
}

var _ OAuthTokenValidator = OAuthTokenValidators(nil)

type OAuthTokenValidators []OAuthTokenValidator

func (v OAuthTokenValidators) Validate(token *oauth.OAuthAccessToken, user *userapi.User) error {
	for _, validator := range v {
		if err := validator.Validate(token, user); err != nil {
			return err
		}
	}
	return nil
}
