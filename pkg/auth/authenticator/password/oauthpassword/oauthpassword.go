package oauthpassword

import (
	"github.com/RangelReale/osincli"
	"github.com/openshift/origin/pkg/auth/authenticator"
)

type Authenticator struct {
	client *osincli.Client
}

func New() authenticator.Password {
	return &Authenticator{}
}

func (a *Authenticator) AuthenticatePassword(user, password string) (string, bool, error) {
	areq := a.client.NewAccessRequest(osincli.PASSWORD, nil)
	areq.CustomParameters["username"] = user
	areq.CustomParameters["password"] = password
	token, err := areq.GetToken()
	if err != nil {
		if oerr, ok := err.(*osincli.Error); ok {
			if oerr.Id == osincli.E_ACCESS_DENIED {
				return nil, false, nil
			}
		}
		return nil, false, err
	}
	return token.AccessToken, true, nil
}
