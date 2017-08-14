package origin

import (
	"crypto/md5"
	"fmt"
	"net/url"

	"github.com/pborman/uuid"

	"k8s.io/apiserver/pkg/storage"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/auth/server/session"
	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type AuthConfig struct {
	Options configapi.OAuthConfig

	// AssetPublicAddresses contains valid redirectURI prefixes to direct browsers to the web console
	AssetPublicAddresses []string

	// KubeClient is kubeclient with enough permission for the auth API
	KubeClient kclientset.Interface

	// OpenShiftClient is osclient with enough permission for the auth API
	OpenShiftClient osclient.Interface

	// RESTOptionsGetter provides storage and RESTOption lookup
	RESTOptionsGetter restoptions.Getter

	// EtcdBackends is a list of storage interfaces, each of which talks to a single etcd backend.
	// These are only used to ensure newly created tokens are distributed to all backends before returning them for use.
	// EtcdHelper should normally be used for storage functions.
	EtcdBackends []storage.Interface

	UserClient                userclient.UserResourceInterface
	IdentityClient            userclient.IdentityInterface
	UserIdentityMappingClient userclient.UserIdentityMappingInterface

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

	userClient, err := userclient.NewForConfig(&masterConfig.PrivilegedLoopbackClientConfig)
	if err != nil {
		return nil, err
	}

	ret := &AuthConfig{
		Options: *options.OAuthConfig,

		KubeClient: kubeClient,

		OpenShiftClient: osClient,

		AssetPublicAddresses: assetPublicURLs,
		RESTOptionsGetter:    masterConfig.RESTOptionsGetter,

		IdentityClient:            userClient.Identities(),
		UserClient:                userClient.Users(),
		UserIdentityMappingClient: userClient.UserIdentityMappings(),

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
	sessionStore := session.NewStore(secure, secrets...)
	return session.NewAuthenticator(sessionStore, config.SessionName, int(config.SessionMaxAgeSeconds)), sessionStore, nil
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
