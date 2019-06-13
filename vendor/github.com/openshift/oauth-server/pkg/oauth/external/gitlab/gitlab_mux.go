package gitlab

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/openshift/oauth-server/pkg/oauth/external"

	"k8s.io/klog"
)

// The hosted version of GitLab is guaranteed to be using the latest stable version
// meaning that we can count on it having OIDC support (and no sub claim bug)
const gitlabHostedDomain = "gitlab.com"

func NewProvider(providerName, URL, clientID, clientSecret string, transport http.RoundTripper, legacy *bool) (external.Provider, error) {
	if isLegacy(legacy, URL) {
		klog.Infof("Using legacy OAuth2 for GitLab identity provider %s url=%s clientID=%s", providerName, URL, clientID)
		return NewOAuthProvider(providerName, URL, clientID, clientSecret, transport)
	}
	klog.Infof("Using OIDC for GitLab identity provider %s url=%s clientID=%s", providerName, URL, clientID)
	return NewOIDCProvider(providerName, URL, clientID, clientSecret, transport)
}

func isLegacy(legacy *bool, URL string) bool {
	// if a value is specified, honor it
	if legacy != nil {
		return *legacy
	}

	// use OIDC if we know it will work since the hosted version is being used
	// validation handles URL parsing errors so we can ignore them here
	if u, err := url.Parse(URL); err == nil && strings.EqualFold(u.Hostname(), gitlabHostedDomain) {
		return false
	}

	// otherwise use OAuth2 (to be safe for now)
	return true
}
