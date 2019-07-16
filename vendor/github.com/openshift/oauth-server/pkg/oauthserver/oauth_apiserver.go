package oauthserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kclientset "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	osinv1 "github.com/openshift/api/osin/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	bootstrap "github.com/openshift/library-go/pkg/authentication/bootstrapauthenticator"
	"github.com/openshift/oauth-server/pkg/config"
	"github.com/openshift/oauth-server/pkg/server/crypto"
	"github.com/openshift/oauth-server/pkg/server/headers"
	"github.com/openshift/oauth-server/pkg/server/session"
	"github.com/openshift/oauth-server/pkg/userregistry/identitymapper"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func init() {
	utilruntime.Must(osinv1.Install(scheme))
}

// TODO we need to switch the oauth server to an external type, but that can be done after we get our externally facing flag values fixed
// TODO remaining bits involve the session file, LDAP util code, validation, ...
func NewOAuthServerConfig(oauthConfig osinv1.OAuthConfig, userClientConfig *rest.Config, genericConfig *genericapiserver.RecommendedConfig) (*OAuthServerConfig, error) {
	// TODO: there is probably some better way to do this
	decoder := codecs.UniversalDecoder(osinv1.GroupVersion)
	for i, idp := range oauthConfig.IdentityProviders {
		if idp.Provider.Object != nil {
			// depending on how you get here, the IDP objects may or may not be filled out
			break
		}
		idpObject, err := runtime.Decode(decoder, idp.Provider.Raw)
		if err != nil {
			return nil, err
		}
		oauthConfig.IdentityProviders[i].Provider.Object = idpObject
	}

	// this leaves the embedded OAuth server code path alone
	if genericConfig == nil {
		genericConfig = genericapiserver.NewRecommendedConfig(codecs)
	}

	genericConfig.LoopbackClientConfig = userClientConfig

	userClient, err := userclient.NewForConfig(userClientConfig)
	if err != nil {
		return nil, err
	}
	oauthClient, err := oauthclient.NewForConfig(userClientConfig)
	if err != nil {
		return nil, err
	}
	eventsClient, err := corev1.NewForConfig(userClientConfig)
	if err != nil {
		return nil, err
	}
	routeClient, err := routeclient.NewForConfig(userClientConfig)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kclientset.NewForConfig(userClientConfig)
	if err != nil {
		return nil, err
	}

	bootstrapUserDataGetter := bootstrap.NewBootstrapUserDataGetter(kubeClient.CoreV1(), kubeClient.CoreV1())

	var sessionAuth session.SessionAuthenticator
	if oauthConfig.SessionConfig != nil {
		// TODO we really need to enforce HTTPS always
		secure := isHTTPS(oauthConfig.MasterPublicURL)
		auth, err := buildSessionAuth(secure, oauthConfig.SessionConfig, bootstrapUserDataGetter)
		if err != nil {
			return nil, err
		}
		sessionAuth = auth

		// session capability is the only thing required to enable the bootstrap IDP
		// we dynamically enable or disable its UI based on the backing secret
		// this must be the first IDP to make sure that it can handle basic auth challenges first
		// this mostly avoids weird cases with the allow all IDP
		oauthConfig.IdentityProviders = append(
			[]osinv1.IdentityProvider{
				{
					Name:            bootstrap.BootstrapUser, // will never conflict with other IDPs due to the :
					UseAsChallenger: true,
					UseAsLogin:      true,
					MappingMethod:   string(identitymapper.MappingMethodClaim), // irrelevant, but needs to be valid
					Provider: runtime.RawExtension{
						Object: &config.BootstrapIdentityProvider{},
					},
				},
			},
			oauthConfig.IdentityProviders...,
		)
	}

	if len(oauthConfig.IdentityProviders) == 0 {
		oauthConfig.IdentityProviders = []osinv1.IdentityProvider{
			{
				Name:            "defaultDenyAll",
				UseAsChallenger: true,
				UseAsLogin:      true,
				MappingMethod:   string(identitymapper.MappingMethodClaim),
				Provider: runtime.RawExtension{
					Object: &osinv1.DenyAllPasswordIdentityProvider{},
				},
			},
		}
	}

	ret := &OAuthServerConfig{
		GenericConfig: genericConfig,
		ExtraOAuthConfig: ExtraOAuthConfig{
			Options:                        oauthConfig,
			KubeClient:                     kubeClient,
			EventsClient:                   eventsClient.Events(""),
			RouteClient:                    routeClient,
			UserClient:                     userClient.Users(),
			IdentityClient:                 userClient.Identities(),
			UserIdentityMappingClient:      userClient.UserIdentityMappings(),
			OAuthAccessTokenClient:         oauthClient.OAuthAccessTokens(),
			OAuthAuthorizeTokenClient:      oauthClient.OAuthAuthorizeTokens(),
			OAuthClientClient:              oauthClient.OAuthClients(),
			OAuthClientAuthorizationClient: oauthClient.OAuthClientAuthorizations(),
			SessionAuth:                    sessionAuth,
			BootstrapUserDataGetter:        bootstrapUserDataGetter,
		},
	}
	genericConfig.BuildHandlerChainFunc = ret.buildHandlerChainForOAuth

	return ret, nil
}

func buildSessionAuth(secure bool, config *osinv1.SessionConfig, getter bootstrap.BootstrapUserDataGetter) (session.SessionAuthenticator, error) {
	secrets, err := getSessionSecrets(config.SessionSecretsFile)
	if err != nil {
		return nil, err
	}
	sessionStore := session.NewStore(config.SessionName, secure, secrets...)
	sessionAuthenticator := session.NewAuthenticator(sessionStore, time.Duration(config.SessionMaxAgeSeconds)*time.Second)
	return session.NewBootstrapAuthenticator(sessionAuthenticator, getter, sessionStore), nil
}

func getSessionSecrets(filename string) ([][]byte, error) {
	// Build secrets list
	var secrets [][]byte

	if len(filename) != 0 {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		jsonData, err := yaml.ToJSON(data)
		if err != nil {
			// probably just json already
			jsonData = data
		}
		sessionSecrets := &osinv1.SessionSecrets{}
		if err := json.NewDecoder(bytes.NewBuffer(jsonData)).Decode(sessionSecrets); err != nil {
			return nil, fmt.Errorf("error reading sessionSecretsFile %s: %v", filename, err)
		}

		if len(sessionSecrets.Secrets) == 0 {
			return nil, fmt.Errorf("sessionSecretsFile %s contained no secrets", filename)
		}

		for _, s := range sessionSecrets.Secrets {
			// TODO make these length independent
			secrets = append(secrets, []byte(s.Authentication))
			secrets = append(secrets, []byte(s.Encryption))
		}
	} else {
		// Generate random signing and encryption secrets if none are specified in config
		const (
			sha256KeyLenBits = sha256.BlockSize * 8 // max key size with HMAC SHA256
			aes256KeyLenBits = 256                  // max key size with AES (AES-256)
		)
		secrets = append(secrets, crypto.RandomBits(sha256KeyLenBits))
		secrets = append(secrets, crypto.RandomBits(aes256KeyLenBits))
	}

	return secrets, nil
}

// isHTTPS returns true if the given URL is a valid https URL
func isHTTPS(u string) bool {
	parsedURL, err := url.Parse(u)
	return err == nil && parsedURL.Scheme == "https"
}

type ExtraOAuthConfig struct {
	Options osinv1.OAuthConfig

	// KubeClient is kubeclient with enough permission for the auth API
	KubeClient kclientset.Interface

	// EventsClient is for creating user events
	EventsClient corev1.EventInterface

	// RouteClient provides a client for OpenShift routes API.
	RouteClient routeclient.RouteV1Interface

	UserClient                userclient.UserInterface
	IdentityClient            userclient.IdentityInterface
	UserIdentityMappingClient userclient.UserIdentityMappingInterface

	OAuthAccessTokenClient         oauthclient.OAuthAccessTokenInterface
	OAuthAuthorizeTokenClient      oauthclient.OAuthAuthorizeTokenInterface
	OAuthClientClient              oauthclient.OAuthClientInterface
	OAuthClientAuthorizationClient oauthclient.OAuthClientAuthorizationInterface

	SessionAuth session.SessionAuthenticator

	BootstrapUserDataGetter bootstrap.BootstrapUserDataGetter
}

type OAuthServerConfig struct {
	GenericConfig    *genericapiserver.RecommendedConfig
	ExtraOAuthConfig ExtraOAuthConfig
}

// OAuthServer serves non-API endpoints for openshift.
type OAuthServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer

	PublicURL url.URL
}

type completedOAuthConfig struct {
	GenericConfig    genericapiserver.CompletedConfig
	ExtraOAuthConfig *ExtraOAuthConfig
}

type CompletedOAuthConfig struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedOAuthConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *OAuthServerConfig) Complete() completedOAuthConfig {
	cfg := completedOAuthConfig{
		c.GenericConfig.Complete(),
		&c.ExtraOAuthConfig,
	}

	return cfg
}

// this server is odd.  It doesn't delegate.  We mostly leave it alone, so I don't plan to make it look "normal".  We'll
// model it as a separate API server to reason about its handling chain, but otherwise, just let it be
func (c completedOAuthConfig) New(delegationTarget genericapiserver.DelegationTarget) (*OAuthServer, error) {
	genericServer, err := c.GenericConfig.New("openshift-oauth", delegationTarget)
	if err != nil {
		return nil, err
	}

	s := &OAuthServer{
		GenericAPIServer: genericServer,
	}

	return s, nil
}

func (c *OAuthServerConfig) buildHandlerChainForOAuth(startingHandler http.Handler, genericConfig *genericapiserver.Config) http.Handler {
	// add OAuth handlers on top of the generic API server handlers
	handler, err := c.WithOAuth(startingHandler)
	if err != nil {
		// the existing errors all cause the OAuth server to die anyway
		panic(err)
	}

	// add back the Authorization header so that WithOAuth can use it even after WithAuthentication deletes it
	// WithOAuth sees users' passwords and can mint tokens so this is not really an issue
	handler = headers.WithRestoreAuthorizationHeader(handler)

	// this is the normal kube handler chain
	handler = genericapiserver.DefaultBuildHandlerChain(handler, genericConfig)

	// store a copy of the Authorization header for later use
	handler = headers.WithPreserveAuthorizationHeader(handler)

	// protected endpoints should not be cached
	handler = headers.WithStandardHeaders(handler)

	return handler
}
