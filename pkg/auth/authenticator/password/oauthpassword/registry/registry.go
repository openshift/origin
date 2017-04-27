package registry

import (
	"net/http"

	"github.com/openshift/origin/pkg/client"
	oclient "github.com/openshift/origin/pkg/oauth/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	restclient "k8s.io/client-go/rest"
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

func (a *Authenticator) AuthenticatePassword(username, password string) (user.Info, bool, error) {
	token, ok, err := a.token.AuthenticatePassword(username, password)
	if !ok || err != nil {
		return nil, false, err
	}

	auth := oclient.OAuthWrapper{a.rt, token}

	client, err := client.New(&restclient.Config{Transport: auth, Host: a.host})
	if err != nil {
		return nil, false, err
	}
	u, err := client.Users().Get("~", metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	info := &user.DefaultInfo{
		Name: u.Name,
		UID:  string(u.UID),
	}

	return info, true, nil
}
