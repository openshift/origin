package registry

import (
	"errors"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kuser "github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	"github.com/openshift/origin/pkg/user/registry/user"
)

type TokenAuthenticator struct {
	tokens oauthaccesstoken.Registry
	users  user.Registry
}

var ErrExpired = errors.New("Token is expired")

func NewTokenAuthenticator(tokens oauthaccesstoken.Registry, users user.Registry) *TokenAuthenticator {
	return &TokenAuthenticator{
		tokens: tokens,
		users:  users,
	}
}

func (a *TokenAuthenticator) AuthenticateToken(value string) (kuser.Info, bool, error) {
	ctx := api.NewContext()

	token, err := a.tokens.GetAccessToken(ctx, value)
	if err != nil {
		return nil, false, err
	}
	if token.CreationTimestamp.Time.Add(time.Duration(token.ExpiresIn) * time.Second).Before(time.Now()) {
		return nil, false, ErrExpired
	}

	u, err := a.users.GetUser(ctx, token.UserName)
	if err != nil {
		return nil, false, err
	}
	if string(u.UID) != token.UserUID {
		return nil, false, fmt.Errorf("user.UID (%s) does not match token.userUID (%s)", u.UID, token.UserUID)
	}

	return &kuser.DefaultInfo{
		Name:   u.Name,
		UID:    string(u.UID),
		Groups: u.Groups,
	}, true, nil
}
