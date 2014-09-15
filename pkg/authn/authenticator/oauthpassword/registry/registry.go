package registry

import (
	"net/http"

	"github.com/openshift/origin/pkg/authn/api"
	"github.com/openshift/origin/pkg/client"
	oclient "github.com/openshift/origin/pkg/oauth/client"
)

type OAuthAccessTokenSource interface {
	AuthenticatePassword(username, password string) (string, bool, error)
}

type Authenticator struct {
	token OAuthAccessTokenSource
	host  string
	rt    http.RoundTripper
}

func New(token OAuthAccessTokenSource, host string, rt http.RoundTripper) *Authenticator {
	if rt == nil {
		rt = http.DefaultTransport
	}
	return &Authenticator{token, host, rt}
}

func (a *Authenticator) AuthenticatePassword(username, password string) (api.UserInfo, bool, error) {
	token, ok, err := a.token.AuthenticatePassword(username, password)
	if !ok || err != nil {
		return nil, false, err
	}

	auth := oclient.OAuthWrapper{a.rt, token}
	client, err := client.NewDirect(a.host, auth)
	if err != nil {
		return nil, false, err
	}
	user, err := client.GetUser("~")
	if err != nil {
		return nil, false, err
	}

	info := &api.DefaultUserInfo{
		Name: user.Name,
		UID:  user.UID,
	}

	return info, true, nil
}
