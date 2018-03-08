package oauthserver

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/pborman/uuid"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/features"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	kclientset "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	oauthapi "github.com/openshift/api/oauth/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	oauthutil "github.com/openshift/origin/pkg/oauth/util"
	"github.com/openshift/origin/pkg/oauthserver/server/session"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func NewOAuthServerConfig(oauthConfig configapi.OAuthConfig, userClientConfig *rest.Config) (*OAuthServerConfig, error) {
	genericConfig := genericapiserver.NewRecommendedConfig(codecs)

	var sessionAuth *session.Authenticator
	var sessionHandlerWrapper handlerWrapper
	if oauthConfig.SessionConfig != nil {
		secure := isHTTPS(oauthConfig.MasterPublicURL)
		auth, wrapper, err := buildSessionAuth(secure, oauthConfig.SessionConfig)
		if err != nil {
			return nil, err
		}
		sessionAuth = auth
		sessionHandlerWrapper = wrapper
	}

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

	ret := &OAuthServerConfig{
		GenericConfig: genericConfig,
		ExtraOAuthConfig: ExtraOAuthConfig{
			Options:                        oauthConfig,
			SessionAuth:                    sessionAuth,
			EventsClient:                   eventsClient.Events(""),
			IdentityClient:                 userClient.Identities(),
			UserClient:                     userClient.Users(),
			UserIdentityMappingClient:      userClient.UserIdentityMappings(),
			OAuthAccessTokenClient:         oauthClient.OAuthAccessTokens(),
			OAuthAuthorizeTokenClient:      oauthClient.OAuthAuthorizeTokens(),
			OAuthClientClient:              oauthClient.OAuthClients(),
			OAuthClientAuthorizationClient: oauthClient.OAuthClientAuthorizations(),
			HandlerWrapper:                 sessionHandlerWrapper,
		},
	}
	genericConfig.BuildHandlerChainFunc = ret.buildHandlerChainForOAuth

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

	HandlerWrapper handlerWrapper
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
	handler, err := c.WithOAuth(startingHandler, genericConfig.RequestContextMapper)
	if err != nil {
		// the existing errors all cause the server to die anyway
		panic(err)
	}
	if utilfeature.DefaultFeatureGate.Enabled(features.AdvancedAuditing) {
		handler = genericapifilters.WithAudit(handler, genericConfig.RequestContextMapper, genericConfig.AuditBackend, genericConfig.AuditPolicyChecker, genericConfig.LongRunningFunc)
	}

	handler = genericfilters.WithMaxInFlightLimit(handler, genericConfig.MaxRequestsInFlight, genericConfig.MaxMutatingRequestsInFlight, genericConfig.RequestContextMapper, genericConfig.LongRunningFunc)
	handler = genericfilters.WithCORS(handler, genericConfig.CorsAllowedOriginList, nil, nil, nil, "true")
	handler = genericfilters.WithTimeoutForNonLongRunningRequests(handler, genericConfig.RequestContextMapper, genericConfig.LongRunningFunc, genericConfig.RequestTimeout)
	handler = genericapifilters.WithRequestInfo(handler, genericapiserver.NewRequestInfoResolver(genericConfig), genericConfig.RequestContextMapper)
	handler = apirequest.WithRequestContext(handler, genericConfig.RequestContextMapper)
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
			webConsoleClient := oauthapi.OAuthClient{
				ObjectMeta:            metav1.ObjectMeta{Name: OpenShiftWebConsoleClientID},
				Secret:                "",
				RespondWithChallenges: false,
				RedirectURIs:          c.ExtraOAuthConfig.AssetPublicAddresses,
				GrantMethod:           oauthapi.GrantHandlerAuto,
			}
			if err := ensureOAuthClient(webConsoleClient, c.ExtraOAuthConfig.OAuthClientClient, true, false); err != nil {
				utilruntime.HandleError(err)
				return false, nil
			}

			browserClient := oauthapi.OAuthClient{
				ObjectMeta:            metav1.ObjectMeta{Name: OpenShiftBrowserClientID},
				Secret:                uuid.New(),
				RespondWithChallenges: false,
				RedirectURIs:          []string{oauthutil.OpenShiftOAuthTokenDisplayURL(c.ExtraOAuthConfig.Options.MasterPublicURL)},
				GrantMethod:           oauthapi.GrantHandlerAuto,
			}
			if err := ensureOAuthClient(browserClient, c.ExtraOAuthConfig.OAuthClientClient, true, true); err != nil {
				utilruntime.HandleError(err)
				return false, nil
			}

			cliClient := oauthapi.OAuthClient{
				ObjectMeta:            metav1.ObjectMeta{Name: OpenShiftCLIClientID},
				Secret:                "",
				RespondWithChallenges: true,
				RedirectURIs:          []string{oauthutil.OpenShiftOAuthTokenImplicitURL(c.ExtraOAuthConfig.Options.MasterPublicURL)},
				GrantMethod:           oauthapi.GrantHandlerAuto,
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
