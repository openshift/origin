package headerrequest

import (
	"net/http"
	"strings"

	"k8s.io/apiserver/pkg/authentication/user"

	authapi "github.com/openshift/origin/pkg/oauthserver/api"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/identitymapper"
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
	// GroupsHeaders is the set of headers to check for groups.  All non-empty values from all headers are aggregated.
	GroupsHeaders []string
}

type Authenticator struct {
	providerName string
	config       *Config
	mapper       authapi.UserIdentityMapper
}

func NewAuthenticator(providerName string, config *Config, mapper authapi.UserIdentityMapper) *Authenticator {
	return &Authenticator{providerName: providerName, config: config, mapper: mapper}
}

func (a *Authenticator) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	id, ok := headerValue(req.Header, a.config.IDHeaders)
	if !ok {
		return nil, false, nil
	}

	identity := authapi.NewDefaultUserIdentityInfo(a.providerName, id)

	if email, ok := headerValue(req.Header, a.config.EmailHeaders); ok {
		identity.Extra[authapi.IdentityEmailKey] = email
	}
	if name, ok := headerValue(req.Header, a.config.NameHeaders); ok {
		identity.Extra[authapi.IdentityDisplayNameKey] = name
	}
	if preferredUsername, ok := headerValue(req.Header, a.config.PreferredUsernameHeaders); ok {
		identity.Extra[authapi.IdentityPreferredUsernameKey] = preferredUsername
	}

	identity.ProviderGroups = headerValues(req.Header, a.config.GroupsHeaders)

	return identitymapper.UserFor(a.mapper, identity)
}

func headerValue(h http.Header, headerNames []string) (string, bool) {
	for _, headerName := range headerNames {
		headerName = strings.TrimSpace(headerName)
		if len(headerName) == 0 {
			continue
		}
		headerValue := h.Get(headerName)
		if len(headerValue) > 0 {
			return headerValue, true
		}
	}
	return "", false
}

func headerValues(h http.Header, headerNames []string) []string {
	var values []string
	for _, headerName := range headerNames {
		headerName = strings.TrimSpace(headerName)
		if len(headerName) == 0 {
			continue
		}
		for _, headerValue := range h[http.CanonicalHeaderKey(headerName)] {
			if len(headerValue) > 0 {
				values = append(values, headerValue)
			}
		}
	}
	return values
}
