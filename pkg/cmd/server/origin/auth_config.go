package origin

import (
	"crypto/md5"
	"fmt"
	"net/url"

	"github.com/pborman/uuid"

	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/auth/server/session"
	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type AuthConfig struct {
	Options configapi.OAuthConfig

	// AssetPublicAddresses contains valid redirectURI prefixes to direct browsers to the web console
	AssetPublicAddresses []string

	// KubeClient is kubeclient with enough permission for the auth API
	KubeClient kclient.Interface

	// OpenShiftClient is osclient with enough permission for the auth API
	OpenShiftClient osclient.Interface

	// RESTOptionsGetter provides storage and RESTOption lookup
	RESTOptionsGetter restoptions.Getter

	// EtcdBackends is a list of storage interfaces, each of which talks to a single etcd backend.
	// These are only used to ensure newly created tokens are distributed to all backends before returning them for use.
	// EtcdHelper should normally be used for storage functions.
	EtcdBackends []storage.Interface

	UserRegistry     userregistry.Registry
	IdentityRegistry identityregistry.Registry

	SessionAuth *session.Authenticator

	HandlerWrapper handlerWrapper
}

func BuildAuthConfig(masterConfig *MasterConfig) (*AuthConfig, error) {
	options := masterConfig.Options
	osClient, kubeClient := masterConfig.OAuthServerClients()

	var sessionAuth *session.Authenticator
	var sessionHandlerWrapper handlerWrapper
	if options.OAuthConfig.SessionConfig != nil {
		secure := isHTTPS(options.OAuthConfig.MasterPublicURL)
		auth, wrapper, err := buildSessionAuth(secure, options.OAuthConfig.SessionConfig)
		if err != nil {
			return nil, err
		}
		sessionAuth = auth
		sessionHandlerWrapper = wrapper
	}

	// Build the list of valid redirect_uri prefixes for a login using the openshift-web-console client to redirect to
	assetPublicURLs := []string{}
	if !options.DisabledFeatures.Has(configapi.FeatureWebConsole) {
		assetPublicURLs = []string{options.OAuthConfig.AssetPublicURL}
	}

	userStorage, err := useretcd.NewREST(masterConfig.RESTOptionsGetter)
	if err != nil {
		return nil, err
	}
	userRegistry := userregistry.NewRegistry(userStorage)

	identityStorage, err := identityetcd.NewREST(masterConfig.RESTOptionsGetter)
	if err != nil {
		return nil, err
	}
	identityRegistry := identityregistry.NewRegistry(identityStorage)

	ret := &AuthConfig{
		Options: *options.OAuthConfig,

		KubeClient: kubeClient,

		OpenShiftClient: osClient,

		AssetPublicAddresses: assetPublicURLs,
		RESTOptionsGetter:    masterConfig.RESTOptionsGetter,

		IdentityRegistry: identityRegistry,
		UserRegistry:     userRegistry,

		SessionAuth: sessionAuth,

		HandlerWrapper: sessionHandlerWrapper,
	}

	return ret, nil
}

func buildSessionAuth(secure bool, config *configapi.SessionConfig) (*session.Authenticator, handlerWrapper, error) {
	secrets, err := getSessionSecrets(config.SessionSecretsFile)
	if err != nil {
		return nil, nil, err
	}
	sessionStore := session.NewStore(secure, int(config.SessionMaxAgeSeconds), secrets...)
	return session.NewAuthenticator(sessionStore, config.SessionName), sessionStore, nil
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
