package selectprovider

import (
	"net/http"

	bootstrap "github.com/openshift/library-go/pkg/authentication/bootstrapauthenticator"
	"github.com/openshift/oauth-server/pkg/api"
	"github.com/openshift/oauth-server/pkg/oauth/handlers"
)

func NewBootstrapSelectProvider(delegate handlers.AuthenticationSelectionHandler, getter bootstrap.BootstrapUserDataGetter) handlers.AuthenticationSelectionHandler {
	return &bootstrapSelectProvider{
		delegate: delegate,
		getter:   getter,
	}
}

type bootstrapSelectProvider struct {
	delegate handlers.AuthenticationSelectionHandler
	getter   bootstrap.BootstrapUserDataGetter
}

func (b *bootstrapSelectProvider) SelectAuthentication(providers []api.ProviderInfo, w http.ResponseWriter, req *http.Request) (*api.ProviderInfo, bool, error) {
	// this should never happen but let us not panic the server in case we screwed up
	// also avoids checking the secret when there is only one provider
	if len(providers) <= 1 || providers[0].Name != bootstrap.BootstrapUser {
		return b.delegate.SelectAuthentication(providers, w, req)
	}

	_, ok, err := b.getter.Get()
	// filter out the bootstrap IDP if the secret is not functional
	if err != nil || !ok {
		return b.delegate.SelectAuthentication(providers[1:], w, req)
	}

	return b.delegate.SelectAuthentication(providers, w, req)
}
