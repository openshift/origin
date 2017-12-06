package registry

import (
	"errors"
	"time"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/oauth/apis/oauth"
	"github.com/openshift/origin/pkg/user/apis/user"
)

var errExpired = errors.New("token is expired")

func NewExpirationValidator() authenticator.OAuthTokenValidator {
	return authenticator.OAuthTokenValidatorFunc(
		func(token *oauth.OAuthAccessToken, _ *user.User) error {
			if token.ExpiresIn > 0 {
				if expire(token).Before(time.Now()) {
					return errExpired
				}
			}
			if token.DeletionTimestamp != nil {
				return errExpired
			}
			return nil
		},
	)
}

func expire(token *oauth.OAuthAccessToken) time.Time {
	return token.CreationTimestamp.Add(time.Duration(token.ExpiresIn) * time.Second)
}
