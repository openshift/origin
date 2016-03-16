package remotemaster

import (
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/restclient"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type Authenticator struct {
	anonymousConfig restclient.Config
}

// NewAuthenticator authenticates by fetching users/~ using the provided token as a bearer token
func NewAuthenticator(anonymousConfig restclient.Config) (*Authenticator, error) {
	// Ensure credentials are removed from the anonymous config
	anonymousConfig = clientcmd.AnonymousClientConfig(&anonymousConfig)

	return &Authenticator{
		anonymousConfig: anonymousConfig,
	}, nil
}

func (a *Authenticator) AuthenticateToken(value string) (user.Info, bool, error) {
	if len(value) == 0 {
		return nil, false, nil
	}

	tokenConfig := a.anonymousConfig
	tokenConfig.BearerToken = value

	c, err := client.New(&tokenConfig)
	if err != nil {
		return nil, false, err
	}

	u, err := c.Users().Get("~")
	if err != nil {
		return nil, false, err
	}

	return &user.DefaultInfo{
		Name:   u.Name,
		UID:    string(u.UID),
		Groups: u.Groups,
	}, true, nil
}
