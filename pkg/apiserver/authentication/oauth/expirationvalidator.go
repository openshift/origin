package oauth

import (
	"errors"
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	oauthv1 "github.com/openshift/api/oauth/v1"
	userv1 "github.com/openshift/api/user/v1"
)

var config = spew.ConfigState{Indent: "\t", MaxDepth: 5, DisableMethods: true}
var errExpired = errors.New("token is expired")

func NewExpirationValidator() OAuthTokenValidator {
	return OAuthTokenValidatorFunc(
		func(token *oauthv1.OAuthAccessToken, uu *userv1.User) error {
			if token.ExpiresIn > 0 {
				now := time.Now()
				if expire(token).Before(now) {
					return fmt.Errorf("expired %s", config.Sdump(token, uu, now.String()))
				}
			}
			if token.DeletionTimestamp != nil {
				return fmt.Errorf("del ts %s", config.Sdump(token, uu))
			}
			return nil
		},
	)
}

func expire(token *oauthv1.OAuthAccessToken) time.Time {
	return token.CreationTimestamp.Add(time.Duration(token.ExpiresIn) * time.Second)
}
