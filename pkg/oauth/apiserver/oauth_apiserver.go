package apiserver

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pborman/uuid"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/features"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/auth/server/session"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	oauthutil "github.com/openshift/origin/pkg/oauth/util"
	routeclient "github.com/openshift/origin/pkg/route/generated/internalclientset"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
)

type OAuthServerConfig struct {
	GenericConfig *genericapiserver.Config

	Options configapi.OAuthConfig

	// AssetPublicAddresses contains valid redirectURI prefixes to direct browsers to the web console
	AssetPublicAddresses []string

	// KubeClient is kubeclient with enough permission for the auth API
	KubeClient kclientset.Interface

	// EventsClient is for creating user events
	EventsClient corev1.EventInterface

	// RouteClient provides a client for OpenShift routes API.
	RouteClient routeclient.Interface

	UserClient                userclient.UserResourceInterface
	IdentityClient            userclient.IdentityInterface
	UserIdentityMappingClient userclient.UserIdentityMappingInterface

	OAuthAccessTokenClient         oauthclient.OAuthAccessTokenInterface
	OAuthAuthorizeTokenClient      oauthclient.OAuthAuthorizeTokenInterface
	OAuthClientClient              oauthclient.OAuthClientInterface
	OAuthClientAuthorizationClient oauthclient.OAuthClientAuthorizationInterface

	SessionAuth *session.Authenticator

	HandlerWrapper handlerWrapper
}

func NewOAuthServerConfig(oauthConfig configapi.OAuthConfig, userClientConfig *rest.Config) (*OAuthServerConfig, error) {
	genericConfig := genericapiserver.NewConfig(kapi.Codecs)

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
		GenericConfig:                  genericConfig,
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

// OAuthServer serves non-API endpoints for openshift.
type OAuthServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer

	PublicURL url.URL
}

type completedOAuthServerConfig struct {
	*OAuthServerConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *OAuthServerConfig) Complete() completedOAuthServerConfig {
	c.GenericConfig.Complete()

	return completedOAuthServerConfig{c}
}

// SkipComplete provides a way to construct a server instance without config completion.
func (c *OAuthServerConfig) SkipComplete() completedOAuthServerConfig {
	return completedOAuthServerConfig{c}
}

// this server is odd.  It doesn't delegate.  We mostly leave it alone, so I don't plan to make it look "normal".  We'll
// model it as a separate API server to reason about its handling chain, but otherwise, just let it be
func (c completedOAuthServerConfig) New(delegationTarget genericapiserver.DelegationTarget) (*OAuthServer, error) {
	genericServer, err := c.OAuthServerConfig.GenericConfig.SkipComplete().New("openshift-oauth", delegationTarget) // completion is done in Complete, no need for a second time
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
func (c *OAuthServerConfig) EnsureBootstrapOAuthClients(context genericapiserver.PostStartHookContext) error {
	webConsoleClient := oauthapi.OAuthClient{
		ObjectMeta:            metav1.ObjectMeta{Name: OpenShiftWebConsoleClientID},
		Secret:                "",
		RespondWithChallenges: false,
		RedirectURIs:          c.AssetPublicAddresses,
		GrantMethod:           oauthapi.GrantHandlerAuto,
	}
	if err := ensureOAuthClient(webConsoleClient, c.OAuthClientClient, true, false); err != nil {
		return err
	}

	browserClient := oauthapi.OAuthClient{
		ObjectMeta:            metav1.ObjectMeta{Name: OpenShiftBrowserClientID},
		Secret:                uuid.New(),
		RespondWithChallenges: false,
		RedirectURIs:          []string{oauthutil.OpenShiftOAuthTokenDisplayURL(c.Options.MasterPublicURL)},
		GrantMethod:           oauthapi.GrantHandlerAuto,
	}
	if err := ensureOAuthClient(browserClient, c.OAuthClientClient, true, true); err != nil {
		return err
	}

	cliClient := oauthapi.OAuthClient{
		ObjectMeta:            metav1.ObjectMeta{Name: OpenShiftCLIClientID},
		Secret:                "",
		RespondWithChallenges: true,
		RedirectURIs:          []string{oauthutil.OpenShiftOAuthTokenImplicitURL(c.Options.MasterPublicURL)},
		GrantMethod:           oauthapi.GrantHandlerAuto,
	}
	if err := ensureOAuthClient(cliClient, c.OAuthClientClient, false, false); err != nil {
		return err
	}

	return nil
}
