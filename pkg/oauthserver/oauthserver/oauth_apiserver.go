package oauthserver

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	"k8s.io/apiserver/pkg/features"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	kclientset "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/configconversion"
	"github.com/openshift/origin/pkg/oauth/urls"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/password/bootstrap"
	"github.com/openshift/origin/pkg/oauthserver/server/crypto"
	"github.com/openshift/origin/pkg/oauthserver/server/session"
	"github.com/openshift/origin/pkg/oauthserver/userregistry/identitymapper"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

// TODO we need to switch the oauth server to an external type, but that can be done after we get our externally facing flag values fixed
func NewOAuthServerConfig(oauthConfig osinv1.OAuthConfig, userClientConfig *rest.Config) (*OAuthServerConfig, error) {
	legacyConfig := &legacyconfigv1.MasterConfig{}
	legacyConfig.OAuthConfig = &legacyconfigv1.OAuthConfig{}
	if err := configconversion.Convert_osinv1_OAuthConfig_to_legacyconfigv1_OAuthConfig(&oauthConfig, legacyConfig.OAuthConfig, nil); err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	if err := latest.Codec.Encode(legacyConfig, buf); err != nil {
		return nil, err
	}
	internalConfig := &configapi.MasterConfig{}
	if _, _, err := latest.Codec.Decode(buf.Bytes(), nil, internalConfig); err != nil {
		return nil, err
	}
	return NewOAuthServerConfigFromInternal(*internalConfig.OAuthConfig, userClientConfig)
}

func NewOAuthServerConfigFromInternal(oauthConfig configapi.OAuthConfig, userClientConfig *rest.Config) (*OAuthServerConfig, error) {
	genericConfig := genericapiserver.NewRecommendedConfig(codecs)
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

	var sessionAuth *session.Authenticator
	if oauthConfig.SessionConfig != nil {
		// TODO we really need to enforce HTTPS always
		secure := isHTTPS(oauthConfig.MasterPublicURL)
		auth, err := buildSessionAuth(secure, oauthConfig.SessionConfig, kubeClient.CoreV1())
		if err != nil {
			return nil, err
		}
		sessionAuth = auth

		// session capability is the only thing required to enable the bootstrap IDP
		// we dynamically enable or disable its UI based on the backing secret
		// this must be the first IDP to make sure that it can handle basic auth challenges first
		// this mostly avoids weird cases with the allow all IDP
		oauthConfig.IdentityProviders = append(
			[]configapi.IdentityProvider{
				{
					Name:            bootstrap.BootstrapUser, // will never conflict with other IDPs due to the :
					UseAsChallenger: true,
					UseAsLogin:      true,
					MappingMethod:   string(identitymapper.MappingMethodClaim), // irrelevant, but needs to be valid
					Provider:        &configapi.BootstrapIdentityProvider{},
				},
			},
			oauthConfig.IdentityProviders...,
		)
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
		},
	}
	genericConfig.BuildHandlerChainFunc = ret.buildHandlerChainForOAuth

	return ret, nil
}

func buildSessionAuth(secure bool, config *configapi.SessionConfig, secretsGetter corev1.SecretsGetter) (*session.Authenticator, error) {
	secrets, err := getSessionSecrets(config.SessionSecretsFile)
	if err != nil {
		return nil, err
	}
	sessionStore := session.NewStore(config.SessionName, secure, secrets...)
	return session.NewAuthenticator(sessionStore, time.Duration(config.SessionMaxAgeSeconds)*time.Second, secretsGetter), nil
}

func getSessionSecrets(filename string) ([][]byte, error) {
	// Build secrets list
	var secrets [][]byte

	if len(filename) != 0 {
		sessionSecrets, err := latest.ReadSessionSecrets(filename)
		if err != nil {
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
	Options configapi.OAuthConfig

	// AssetPublicAddresses contains valid redirectURI prefixes to direct browsers to the web console
	AssetPublicAddresses []string

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

	SessionAuth *session.Authenticator
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
	handler, err := c.WithOAuth(startingHandler)
	if err != nil {
		// the existing errors all cause the server to die anyway
		panic(err)
	}
	if utilfeature.DefaultFeatureGate.Enabled(features.AdvancedAuditing) {
		handler = genericapifilters.WithAudit(handler, genericConfig.AuditBackend, genericConfig.AuditPolicyChecker, genericConfig.LongRunningFunc)
	}

	handler = genericfilters.WithMaxInFlightLimit(handler, genericConfig.MaxRequestsInFlight, genericConfig.MaxMutatingRequestsInFlight, genericConfig.LongRunningFunc)
	handler = genericfilters.WithCORS(handler, genericConfig.CorsAllowedOriginList, nil, nil, nil, "true")
	handler = genericfilters.WithTimeoutForNonLongRunningRequests(handler, genericConfig.LongRunningFunc, genericConfig.RequestTimeout)
	handler = genericapifilters.WithRequestInfo(handler, genericapiserver.NewRequestInfoResolver(genericConfig))
	handler = genericfilters.WithPanicRecovery(handler)
	return handler
}

// TODO, this moves to the `apiserver.go` when we have it for this group
// TODO TODO, this actually looks a lot like a controller or an add-on manager style thing.  Seems like we'd want to do this outside
// EnsureBootstrapOAuthClients creates or updates the bootstrap oauth clients that openshift relies upon.
func (c *OAuthServerConfig) StartOAuthClientsBootstrapping(context genericapiserver.PostStartHookContext) error {
	// the TODO above still applies, but this makes it possible for this poststarthook to do its job with a split kubeapiserver and not run forever
	go func() {
		wait.PollUntil(1*time.Second, func() (done bool, err error) {
			webConsoleClient := oauthv1.OAuthClient{
				ObjectMeta:            metav1.ObjectMeta{Name: openShiftWebConsoleClientID},
				Secret:                "",
				RespondWithChallenges: false,
				RedirectURIs:          c.ExtraOAuthConfig.AssetPublicAddresses,
				GrantMethod:           oauthv1.GrantHandlerAuto,
			}
			if err := ensureOAuthClient(webConsoleClient, c.ExtraOAuthConfig.OAuthClientClient, true, false); err != nil {
				utilruntime.HandleError(err)
				return false, nil
			}

			browserClient := oauthv1.OAuthClient{
				ObjectMeta:            metav1.ObjectMeta{Name: openShiftBrowserClientID},
				Secret:                crypto.Random256BitsString(),
				RespondWithChallenges: false,
				RedirectURIs:          []string{urls.OpenShiftOAuthTokenDisplayURL(c.ExtraOAuthConfig.Options.MasterPublicURL)},
				GrantMethod:           oauthv1.GrantHandlerAuto,
			}
			if err := ensureOAuthClient(browserClient, c.ExtraOAuthConfig.OAuthClientClient, true, true); err != nil {
				utilruntime.HandleError(err)
				return false, nil
			}

			cliClient := oauthv1.OAuthClient{
				ObjectMeta:            metav1.ObjectMeta{Name: openShiftCLIClientID},
				Secret:                "",
				RespondWithChallenges: true,
				RedirectURIs:          []string{urls.OpenShiftOAuthTokenImplicitURL(c.ExtraOAuthConfig.Options.MasterPublicURL)},
				GrantMethod:           oauthv1.GrantHandlerAuto,
			}
			if err := ensureOAuthClient(cliClient, c.ExtraOAuthConfig.OAuthClientClient, false, false); err != nil {
				utilruntime.HandleError(err)
				return false, nil
			}

			return true, nil
		}, context.StopCh)
	}()

	return nil
}
