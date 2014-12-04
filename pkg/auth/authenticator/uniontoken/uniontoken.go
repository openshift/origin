package uniontoken

import (
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authapi "github.com/openshift/origin/pkg/auth/api"
	authauthenticator "github.com/openshift/origin/pkg/auth/authenticator"
)

type unionTokenAuthenticator []authauthenticator.Token

// NewUnionAuthentication returns a request authenticator that validates credentials using a chain of authenticator.Request objects
func NewUnionAuthentication(authTokenAuthenticators []authauthenticator.Token) authauthenticator.Token {
	return unionTokenAuthenticator(authTokenAuthenticators)
}

// AuthenticateRequest authenticates the request using a chain of authenticator.Request objects.  The first
// success returns that identity.  Errors are only returned if no matches are found.
func (authHandler unionTokenAuthenticator) AuthenticateToken(token string) (authapi.UserInfo, bool, error) {
	var errors kutil.ErrorList
	for _, currHandler := range authHandler {
		info, ok, err := currHandler.AuthenticateToken(token)
		if err == nil && ok {
			return info, ok, err
		}
		if err != nil {
			errors = append(errors, err)
		}
	}

	return nil, false, errors.ToError()
}
