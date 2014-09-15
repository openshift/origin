package requestheader

import (
	"net/http"

	"github.com/openshift/origin/pkg/auth/api"
)

type Config struct {
	UserNameHeader string
}

func NewDefaultConfig() *Config {
	return &Config{
		UserNameHeader: "X-Remote-User",
	}
}

type Authenticator struct {
	config *Config
}

func NewAuthenticator(config *Config) *Authenticator {
	return &Authenticator{config}
}

func (a *Authenticator) AuthenticateRequest(req *http.Request) (api.UserInfo, bool, error) {
	name := req.Header.Get(a.config.UserNameHeader)
	if name == "" {
		return nil, false, nil
	}
	user := &api.DefaultUserInfo{
		Name: name,
	}
	return user, true, nil
}
