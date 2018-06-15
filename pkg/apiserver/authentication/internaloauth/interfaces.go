package internaloauth

import (
	userapi "github.com/openshift/api/user/v1"
	"github.com/openshift/origin/pkg/oauth/apis/oauth"
)

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

type UserToGroupMapper interface {
	GroupsFor(username string) ([]*userapi.Group, error)
}

type NoopGroupMapper struct{}

func (n NoopGroupMapper) GroupsFor(username string) ([]*userapi.Group, error) {
	return []*userapi.Group{}, nil
}
