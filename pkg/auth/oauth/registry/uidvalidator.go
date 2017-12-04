package registry

import (
	"fmt"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/oauth/apis/oauth"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

const errInvalidUIDStr = "user.UID (%s) does not match token.userUID (%s)"

func NewUIDValidator() authenticator.OAuthTokenValidator {
	return authenticator.OAuthTokenValidatorFunc(
		func(token *oauth.OAuthAccessToken, user *userapi.User) error {
			if string(user.UID) != token.UserUID {
				return fmt.Errorf(errInvalidUIDStr, user.UID, token.UserUID)
			}
			return nil
		},
	)
}
