package origin

import (
	"crypto/md5"
	"crypto/x509"
	"fmt"
	"net/url"

	"code.google.com/p/go-uuid/uuid"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/auth/server/session"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
)

type AuthConfig struct {
	Options configapi.OAuthConfig

	// AssetPublicAddresses contains valid redirectURI prefixes to direct browsers to the web console
	AssetPublicAddresses []string
	MasterRoots          *x509.CertPool
	EtcdHelper           storage.Interface

	UserRegistry     userregistry.Registry
	IdentityRegistry identityregistry.Registry

	SessionAuth *session.Authenticator
}

func BuildAuthConfig(options configapi.MasterConfig) (*AuthConfig, error) {
	client, err := etcd.GetAndTestEtcdClient(options.EtcdClientInfo)
	if err != nil {
		return nil, err
	}
	etcdHelper, err := NewEtcdStorage(client, options.EtcdStorageConfig.OpenShiftStorageVersion, options.EtcdStorageConfig.OpenShiftStoragePrefix)
	if err != nil {
		return nil, fmt.Errorf("Error setting up server storage: %v", err)
	}

	apiServerCAs, err := configapi.GetAPIServerCertCAPool(options)
	if err != nil {
		return nil, err
	}

	var sessionAuth *session.Authenticator
	if options.OAuthConfig.SessionConfig != nil {
		secure := isHTTPS(options.OAuthConfig.MasterPublicURL)
		auth, err := BuildSessionAuth(secure, options.OAuthConfig.SessionConfig)
		if err != nil {
			return nil, err
		}
		sessionAuth = auth
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

		SessionAuth: sessionAuth,
	}

	return ret, nil
}

func BuildSessionAuth(secure bool, config *configapi.SessionConfig) (*session.Authenticator, error) {
	secrets, err := getSessionSecrets(config.SessionSecretsFile)
	if err != nil {
		return nil, err
	}
	sessionStore := session.NewStore(secure, int(config.SessionMaxAgeSeconds), secrets...)
	return session.NewAuthenticator(sessionStore, config.SessionName), nil
}

func getSessionSecrets(filename string) ([]string, error) {
	// Build secrets list
	secrets := []string{}

	if len(filename) != 0 {
		sessionSecrets, err := latest.ReadSessionSecrets(filename)
		if err != nil {
			return nil, fmt.Errorf("error reading sessionSecretsFile %s: %v", filename, err)
		}

		if len(sessionSecrets.Secrets) == 0 {
			return nil, fmt.Errorf("sessionSecretsFile %s contained no secrets", filename)
		}

		for _, s := range sessionSecrets.Secrets {
			secrets = append(secrets, s.Authentication)
			secrets = append(secrets, s.Encryption)
		}
	} else {
		// Generate random signing and encryption secrets if none are specified in config
		secrets = append(secrets, fmt.Sprintf("%x", md5.Sum([]byte(uuid.NewRandom().String()))))
		secrets = append(secrets, fmt.Sprintf("%x", md5.Sum([]byte(uuid.NewRandom().String()))))
	}

	return secrets, nil
}

// isHTTPS returns true if the given URL is a valid https URL
func isHTTPS(u string) bool {
	parsedURL, err := url.Parse(u)
	return err == nil && parsedURL.Scheme == "https"
}
