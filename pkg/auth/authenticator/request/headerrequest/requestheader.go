package headerrequest

import (
	"net/http"
	"strings"

	"github.com/golang/glog"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"k8s.io/kubernetes/pkg/auth/user"
)

type Config struct {
	// IDHeaders lists the headers to check (in order, case-insensitively) for an identity. The first header with a value wins.
	IDHeaders []string
	// NameHeaders lists the headers to check (in order, case-insensitively) for a display name. The first header with a value wins.
	NameHeaders []string
	// PreferredUsernameHeaders lists the headers to check (in order, case-insensitively) for a preferred username. The first header with a value wins. If empty, the ID is used
	PreferredUsernameHeaders []string
	// EmailHeaders lists the headers to check (in order, case-insensitively) for an email address. The first header with a value wins.
	EmailHeaders []string
}

func NewDefaultConfig() *Config {
	return &Config{
		IDHeaders: []string{"X-Remote-User", "Remote-User"},
	}
}

type Authenticator struct {
	providerName string
	config       *Config
	mapper       authapi.UserIdentityMapper
}

func NewAuthenticator(providerName string, config *Config, mapper authapi.UserIdentityMapper) *Authenticator {
	return &Authenticator{providerName, config, mapper}
}

func (a *Authenticator) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	id := headerValue(req.Header, a.config.IDHeaders)
	if len(id) == 0 {
		return nil, false, nil
	}

	identity := authapi.NewDefaultUserIdentityInfo(a.providerName, id)

	if email := headerValue(req.Header, a.config.EmailHeaders); len(email) > 0 {
		identity.Extra[authapi.IdentityEmailKey] = email
	}
	if name := headerValue(req.Header, a.config.NameHeaders); len(name) > 0 {
		identity.Extra[authapi.IdentityDisplayNameKey] = name
	}
	if preferredUsername := headerValue(req.Header, a.config.PreferredUsernameHeaders); len(preferredUsername) > 0 {
		identity.Extra[authapi.IdentityPreferredUsernameKey] = preferredUsername
	}

	user, err := a.mapper.UserFor(identity)
	if err != nil {
		glog.V(4).Infof("Error creating or updating mapping for: %#v due to %v", identity, err)
		return nil, false, err
	}
	glog.V(4).Infof("Got userIdentityMapping: %#v", user)

	return user, true, nil
}

func headerValue(h http.Header, headerNames []string) string {
	for _, headerName := range headerNames {
		headerName = strings.TrimSpace(headerName)
		if len(headerName) == 0 {
			continue
		}
		headerValue := h.Get(headerName)
		if len(headerValue) > 0 {
			return headerValue
		}
	}
	return ""
}
