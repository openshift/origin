package internaloauth

import (
	"fmt"

	userapi "github.com/openshift/api/user/v1"
	"github.com/openshift/origin/pkg/oauth/apis/oauth"
)

const errInvalidUIDStr = "user.UID (%s) does not match token.userUID (%s)"

func NewUIDValidator() OAuthTokenValidator {
	return OAuthTokenValidatorFunc(
		func(token *oauth.OAuthAccessToken, user *userapi.User) error {
			if string(user.UID) != token.UserUID {
				return fmt.Errorf(errInvalidUIDStr, user.UID, token.UserUID)
			}
			return nil
		},
	)
}
