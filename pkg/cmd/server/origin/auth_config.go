package origin

import (
	"crypto/x509"
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/auth/server/session"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
)

type AuthConfig struct {
	Options configapi.OAuthConfig

	// Valid redirectURI prefixes to direct browsers to the web console
	AssetPublicAddresses []string
	MasterRoots          *x509.CertPool
	EtcdHelper           tools.EtcdHelper

	UserRegistry     userregistry.Registry
	IdentityRegistry identityregistry.Registry

	// sessionAuth holds the Authenticator built from the exported Session* options. It should only be accessed via getSessionAuth(), since it is lazily built.
	sessionAuth *session.Authenticator
}

func BuildAuthConfig(options configapi.MasterConfig) (*AuthConfig, error) {
	etcdHelper, err := etcd.NewOpenShiftEtcdHelper(options.EtcdClientInfo)
	if err != nil {
		return nil, fmt.Errorf("Error setting up server storage: %v", err)
	}

	apiServerCAs, err := configapi.GetAPIServerCertCAPool(options)
	if err != nil {
		return nil, err
	}

	// Build the list of valid redirect_uri prefixes for a login using the openshift-web-console client to redirect to
	// TODO: allow configuring this
	// TODO: remove hard-coding of development UI server
	assetPublicURLs := []string{options.OAuthConfig.AssetPublicURL, "http://localhost:9000", "https://localhost:9000"}

	userStorage := useretcd.NewREST(etcdHelper)
	userRegistry := userregistry.NewRegistry(userStorage)
	identityStorage := identityetcd.NewREST(etcdHelper)
	identityRegistry := identityregistry.NewRegistry(identityStorage)

	ret := &AuthConfig{
		Options: *options.OAuthConfig,

		AssetPublicAddresses: assetPublicURLs,
		MasterRoots:          apiServerCAs,
		EtcdHelper:           etcdHelper,

		IdentityRegistry: identityRegistry,
		UserRegistry:     userRegistry,
	}

	return ret, nil

}
