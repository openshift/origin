package headerrequest

import (
	"net/http"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/auth/api"
	authapi "github.com/openshift/origin/pkg/auth/api"
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
	mapper authapi.UserIdentityMapper
}

func NewAuthenticator(config *Config, mapper authapi.UserIdentityMapper) *Authenticator {
	return &Authenticator{config, mapper}
}

func (a *Authenticator) AuthenticateRequest(req *http.Request) (api.UserInfo, bool, error) {
	username := req.Header.Get(a.config.UserNameHeader)
	if len(username) == 0 {
		return nil, false, nil
	}

	identity := &authapi.DefaultUserIdentityInfo{
		UserName: username,
	}
	user, err := a.mapper.UserFor(identity)
	if err != nil {
		return nil, false, err
	}
	glog.V(4).Infof("Got userIdentityMapping: %#v", user)

	return user, true, nil
}
