package origin

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"

	"code.google.com/p/go-uuid/uuid"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kuser "github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/RangelReale/osin"
	"github.com/RangelReale/osincli"
	"github.com/emicklei/go-restful"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/challenger/passwordchallenger"
	"github.com/openshift/origin/pkg/auth/authenticator/password/allowanypassword"
	"github.com/openshift/origin/pkg/auth/authenticator/password/basicauthpassword"
	"github.com/openshift/origin/pkg/auth/authenticator/password/denypassword"
	"github.com/openshift/origin/pkg/auth/authenticator/password/htpasswd"
	"github.com/openshift/origin/pkg/auth/authenticator/request/basicauthrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/headerrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/unionrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/x509request"
	"github.com/openshift/origin/pkg/auth/authenticator/token/filetoken"
	"github.com/openshift/origin/pkg/auth/oauth/external"
	"github.com/openshift/origin/pkg/auth/oauth/external/github"
	"github.com/openshift/origin/pkg/auth/oauth/external/google"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/oauth/registry"
	authnregistry "github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/server/csrf"
	"github.com/openshift/origin/pkg/auth/server/grant"
	"github.com/openshift/origin/pkg/auth/server/login"
	"github.com/openshift/origin/pkg/auth/server/session"
	"github.com/openshift/origin/pkg/auth/server/tokenrequest"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/client"
	oauthclient "github.com/openshift/origin/pkg/oauth/registry/client"
	"github.com/openshift/origin/pkg/oauth/registry/clientauthorization"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	"github.com/openshift/origin/pkg/oauth/server/osinserver/registrystorage"
)

const (
	OpenShiftOAuthAPIPrefix      = "/oauth"
	OpenShiftLoginPrefix         = "/login"
	OpenShiftApprovePrefix       = "/oauth/approve"
	OpenShiftOAuthCallbackPrefix = "/oauth2callback"
	OpenShiftWebConsoleClientID  = "openshift-web-console"
)

var (
	OSWebConsoleClientBase = oauthapi.OAuthClient{
		ObjectMeta: kapi.ObjectMeta{
			Name: OpenShiftWebConsoleClientID,
		},
		Secret: uuid.NewUUID().String(), // random secret so no one knows what it is ahead of time.
	}
	// OSBrowserClientBase is used as a skeleton for building a Client.  We can't set the allowed redirecturis because we don't yet know the host:port of the auth server
	OSBrowserClientBase = oauthapi.OAuthClient{
		ObjectMeta: kapi.ObjectMeta{
			Name: "openshift-browser-client",
		},
		Secret: uuid.NewUUID().String(), // random secret so no one knows what it is ahead of time.  This still allows us to loop back for /requestToken
	}
	OSCliClientBase = oauthapi.OAuthClient{
		ObjectMeta: kapi.ObjectMeta{
			Name: "openshift-challenging-client",
		},
		Secret:                uuid.NewUUID().String(), // random secret so no one knows what it is ahead of time.  This still allows us to loop back for /requestToken
		RespondWithChallenges: true,
	}
)

// InstallSupport registers endpoints for an OAuth2 server into the provided mux,
// then returns an array of strings indicating what endpoints were started
// (these are format strings that will expect to be sent a single string value).
func (c *AuthConfig) InstallAPI(container *restful.Container) []string {
	// TODO: register into container
	mux := container.ServeMux

	oauthEtcd := oauthetcd.New(c.EtcdHelper)

	authRequestHandler, authHandler, authFinalizer, err := c.getAuthorizeAuthenticationHandlers(mux)
	if err != nil {
		glog.Fatal(err)
	}

	storage := registrystorage.New(oauthEtcd, oauthEtcd, oauthEtcd, registry.NewUserConversion())
	config := osinserver.NewDefaultServerConfig()
	if c.Options.TokenConfig.AuthorizeTokenMaxAgeSeconds > 0 {
		config.AuthorizationExpiration = c.Options.TokenConfig.AuthorizeTokenMaxAgeSeconds
	}
	if c.Options.TokenConfig.AccessTokenMaxAgeSeconds > 0 {
		config.AccessExpiration = c.Options.TokenConfig.AccessTokenMaxAgeSeconds
	}

	grantChecker := registry.NewClientAuthorizationGrantChecker(oauthEtcd)
	grantHandler := c.getGrantHandler(mux, authRequestHandler, oauthEtcd, oauthEtcd)

	server := osinserver.New(
		config,
		storage,
		osinserver.AuthorizeHandlers{
			handlers.NewAuthorizeAuthenticator(
				authRequestHandler,
				authHandler,
				handlers.EmptyError{},
			),
			handlers.NewGrantCheck(
				grantChecker,
				grantHandler,
				handlers.EmptyError{},
			),
			authFinalizer,
		},
		osinserver.AccessHandlers{
			handlers.NewDenyAccessAuthenticator(),
		},
		osinserver.NewDefaultErrorHandler(),
	)
	server.Install(mux, OpenShiftOAuthAPIPrefix)

	CreateOrUpdateDefaultOAuthClients(c.Options.MasterPublicURL, c.AssetPublicAddresses, oauthEtcd)
	osOAuthClientConfig := c.NewOpenShiftOAuthClientConfig(&OSBrowserClientBase)
	osOAuthClientConfig.RedirectUrl = c.Options.MasterPublicURL + path.Join(OpenShiftOAuthAPIPrefix, tokenrequest.DisplayTokenEndpoint)

	osOAuthClient, _ := osincli.NewClient(osOAuthClientConfig)
	if c.MasterRoots != nil {
		// Copy the default transport
		var transport http.Transport = *http.DefaultTransport.(*http.Transport)
		// Set TLS CA roots
		transport.TLSClientConfig = &tls.Config{RootCAs: c.MasterRoots}
		osOAuthClient.Transport = &transport
	}

	tokenRequestEndpoints := tokenrequest.NewEndpoints(osOAuthClient)
	tokenRequestEndpoints.Install(mux, OpenShiftOAuthAPIPrefix)

	// glog.Infof("oauth server configured as: %#v", server)
	// glog.Infof("auth handler: %#v", authHandler)
	// glog.Infof("auth request handler: %#v", authRequestHandler)
	// glog.Infof("grant checker: %#v", grantChecker)
	// glog.Infof("grant handler: %#v", grantHandler)

	return []string{
		fmt.Sprintf("Started OAuth2 API at %%s%s", OpenShiftOAuthAPIPrefix),
		fmt.Sprintf("Started login server at %%s%s", OpenShiftLoginPrefix),
	}
}

// NewOpenShiftOAuthClientConfig provides config for OpenShift OAuth client
func (c *AuthConfig) NewOpenShiftOAuthClientConfig(client *oauthapi.OAuthClient) *osincli.ClientConfig {
	config := &osincli.ClientConfig{
		ClientId:                 client.Name,
		ClientSecret:             client.Secret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             OpenShiftOAuthAuthorizeURL(c.Options.MasterPublicURL),
		TokenUrl:                 OpenShiftOAuthTokenURL(c.Options.MasterPublicURL),
		Scope:                    "",
	}
	return config
}

func OpenShiftOAuthAuthorizeURL(masterAddr string) string {
	return masterAddr + path.Join(OpenShiftOAuthAPIPrefix, osinserver.AuthorizePath)
}
func OpenShiftOAuthTokenURL(masterAddr string) string {
	return masterAddr + path.Join(OpenShiftOAuthAPIPrefix, osinserver.TokenPath)
}

func CreateOrUpdateDefaultOAuthClients(masterPublicAddr string, assetPublicAddresses []string, clientRegistry oauthclient.Registry) {
	clientsToEnsure := []*oauthapi.OAuthClient{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OSWebConsoleClientBase.Name,
			},
			Secret:                OSWebConsoleClientBase.Secret,
			RespondWithChallenges: OSWebConsoleClientBase.RespondWithChallenges,
			RedirectURIs:          assetPublicAddresses,
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OSBrowserClientBase.Name,
			},
			Secret:                OSBrowserClientBase.Secret,
			RespondWithChallenges: OSBrowserClientBase.RespondWithChallenges,
			RedirectURIs:          []string{masterPublicAddr + path.Join(OpenShiftOAuthAPIPrefix, tokenrequest.DisplayTokenEndpoint)},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OSCliClientBase.Name,
			},
			Secret:                OSCliClientBase.Secret,
			RespondWithChallenges: OSCliClientBase.RespondWithChallenges,
			RedirectURIs:          []string{masterPublicAddr + path.Join(OpenShiftOAuthAPIPrefix, tokenrequest.DisplayTokenEndpoint)},
		},
	}

	for _, currClient := range clientsToEnsure {
		if existing, err := clientRegistry.GetClient(currClient.Name); err == nil || strings.Contains(err.Error(), " not found") {
			if existing != nil {
				clientRegistry.DeleteClient(currClient.Name)
			}
			if err = clientRegistry.CreateClient(currClient); err != nil {
				glog.Errorf("Error creating client: %v due to %v\n", currClient, err)
			}
		} else {
			glog.Errorf("Error getting client: %v due to %v\n", currClient, err)

		}
	}
}

// getCSRF returns the object responsible for generating and checking CSRF tokens
func getCSRF() csrf.CSRF {
	return csrf.NewCookieCSRF("csrf", "/", "", false, false)
}

func (c *AuthConfig) getSessionAuth() *session.Authenticator {
	if c.Options.SessionConfig != nil {
		if c.sessionAuth == nil {
			sessionStore := session.NewStore(int(c.Options.SessionConfig.SessionMaxAgeSeconds), c.Options.SessionConfig.SessionSecrets...)
			c.sessionAuth = session.NewAuthenticator(sessionStore, c.Options.SessionConfig.SessionName)
		}
	}
	return c.sessionAuth
}

func (c *AuthConfig) getAuthorizeAuthenticationHandlers(mux cmdutil.Mux) (authenticator.Request, handlers.AuthenticationHandler, osinserver.AuthorizeHandler, error) {
	authRequestHandler, err := c.getAuthenticationRequestHandler()
	if err != nil {
		return nil, nil, nil, err
	}
	authHandler, err := c.getAuthenticationHandler(mux, handlers.EmptyError{})
	if err != nil {
		return nil, nil, nil, err
	}
	authFinalizer := c.getAuthenticationFinalizer()

	return authRequestHandler, authHandler, authFinalizer, nil
}

// getGrantHandler returns the object that handles approving or rejecting grant requests
func (c *AuthConfig) getGrantHandler(mux cmdutil.Mux, auth authenticator.Request, clientregistry clientregistry.Registry, authregistry clientauthorization.Registry) handlers.GrantHandler {
	switch c.Options.GrantConfig.Method {
	case configapi.GrantHandlerDeny:
		return handlers.NewEmptyGrant()

	case configapi.GrantHandlerAuto:
		return handlers.NewAutoGrant()

	case configapi.GrantHandlerPrompt:
		grantServer := grant.NewGrant(getCSRF(), auth, grant.DefaultFormRenderer, clientregistry, authregistry)
		grantServer.Install(mux, OpenShiftApprovePrefix)
		return handlers.NewRedirectGrant(OpenShiftApprovePrefix)

	default:
		glog.Fatalf("No grant handler found that matches %v.  The oauth server cannot start!", c.Options.GrantConfig.Method)
	}

	return nil
}

// getAuthenticationFinalizer returns an authentication finalizer which is called just prior to writing a response to an authorization request
func (c *AuthConfig) getAuthenticationFinalizer() osinserver.AuthorizeHandler {
	if c.getSessionAuth() != nil {
		// The session needs to know the authorize flow is done so it can invalidate the session
		return osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, w http.ResponseWriter) (bool, error) {
			_ = c.getSessionAuth().InvalidateAuthentication(w, ar.HttpRequest)
			return false, nil
		})
	}

	// Otherwise return a no-op finalizer
	return osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, w http.ResponseWriter) (bool, error) {
		return false, nil
	})
}

func (c *AuthConfig) getAuthenticationHandler(mux cmdutil.Mux, errorHandler handlers.AuthenticationErrorHandler) (handlers.AuthenticationHandler, error) {
	successHandler := c.getAuthenticationSuccessHandler()

	challengers := map[string]handlers.AuthenticationChallenger{}
	redirectors := map[string]handlers.AuthenticationRedirector{}

	for _, identityProvider := range c.Options.IdentityProviders {
		identityMapper := identitymapper.NewAlwaysCreateUserIdentityToUserMapper(c.IdentityRegistry, c.UserRegistry)

		if configapi.IsPasswordAuthenticator(identityProvider) {
			passwordAuth, err := c.getPasswordAuthenticator(identityProvider)
			if err != nil {
				return nil, err
			}

			if identityProvider.UseAsLogin {
				redirectors["login"] = &redirector{RedirectURL: OpenShiftLoginPrefix, ThenParam: "then"}
				login := login.NewLogin(getCSRF(), &callbackPasswordAuthenticator{passwordAuth, successHandler}, login.DefaultLoginFormRenderer)
				login.Install(mux, OpenShiftLoginPrefix)
			}
			if identityProvider.UseAsChallenger {
				challengers["login"] = passwordchallenger.NewBasicAuthChallenger("openshift")
			}

		} else {
			switch provider := identityProvider.Provider.Object.(type) {
			case (*configapi.OAuthRedirectingIdentityProvider):
				callbackPath := ""
				var oauthProvider external.Provider
				switch provider.Provider.Object.(type) {
				case (*configapi.GoogleOAuthProvider):
					callbackPath = path.Join(OpenShiftOAuthCallbackPrefix, "google")
					oauthProvider = google.NewProvider(identityProvider.Name, provider.ClientID, provider.ClientSecret)
				case (*configapi.GitHubOAuthProvider):
					callbackPath = path.Join(OpenShiftOAuthCallbackPrefix, "github")
					oauthProvider = github.NewProvider(identityProvider.Name, provider.ClientID, provider.ClientSecret)
				default:
					return nil, fmt.Errorf("unexpected oauth provider %#v", provider)
				}

				state := external.DefaultState(getCSRF())
				oauthHandler, err := external.NewExternalOAuthRedirector(oauthProvider, state, c.Options.MasterPublicURL+callbackPath, successHandler, errorHandler, identityMapper)
				if err != nil {
					return nil, fmt.Errorf("unexpected error: %v", err)
				}

				mux.Handle(callbackPath, oauthHandler)
				if identityProvider.UseAsLogin {
					redirectors[identityProvider.Name] = oauthHandler
				}
				if identityProvider.UseAsChallenger {
					return nil, errors.New("oauth identity providers cannot issue challenges")
				}
			}
		}
	}

	authHandler := handlers.NewUnionAuthenticationHandler(challengers, redirectors, errorHandler)
	return authHandler, nil
}

func (c *AuthConfig) getPasswordAuthenticator(identityProvider configapi.IdentityProvider) (authenticator.Password, error) {
	identityMapper := identitymapper.NewAlwaysCreateUserIdentityToUserMapper(c.IdentityRegistry, c.UserRegistry)

	switch provider := identityProvider.Provider.Object.(type) {
	case (*configapi.AllowAllPasswordIdentityProvider):
		return allowanypassword.New(identityProvider.Name, identityMapper), nil

	case (*configapi.DenyAllPasswordIdentityProvider):
		return denypassword.New(), nil

	case (*configapi.HTPasswdPasswordIdentityProvider):
		htpasswdFile := provider.File
		if len(htpasswdFile) == 0 {
			return nil, fmt.Errorf("HTPasswdFile is required to support htpasswd auth")
		}
		if htpasswordAuth, err := htpasswd.New(identityProvider.Name, htpasswdFile, identityMapper); err != nil {
			return nil, fmt.Errorf("Error loading htpasswd file %s: %v", htpasswdFile, err)
		} else {
			return htpasswordAuth, nil
		}

	case (*configapi.BasicAuthPasswordIdentityProvider):
		connectionInfo := provider.RemoteConnectionInfo
		if len(connectionInfo.URL) == 0 {
			return nil, fmt.Errorf("URL is required for BasicAuthPasswordIdentityProvider")
		}
		transport, err := cmdutil.TransportFor(connectionInfo.CA, connectionInfo.ClientCert.CertFile, connectionInfo.ClientCert.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("Error building BasicAuthPasswordIdentityProvider client: %v", err)
		}
		return basicauthpassword.New(identityProvider.Name, connectionInfo.URL, transport, identityMapper), nil

	default:
		return nil, fmt.Errorf("No password auth found that matches %v.  The oauth server cannot start!", identityProvider)
	}

}

func (c *AuthConfig) getAuthenticationSuccessHandler() handlers.AuthenticationSuccessHandler {
	successHandlers := handlers.AuthenticationSuccessHandlers{}

	if c.getSessionAuth() != nil {
		successHandlers = append(successHandlers, c.getSessionAuth())
	}

	addedRedirectSuccessHandler := false
	for _, identityProvider := range c.Options.IdentityProviders {
		if !identityProvider.UseAsLogin {
			continue
		}

		switch identityProvider.Provider.Object.(type) {
		case (*configapi.OAuthRedirectingIdentityProvider):
			successHandlers = append(successHandlers, external.DefaultState(getCSRF()).(handlers.AuthenticationSuccessHandler))
		}

		if !addedRedirectSuccessHandler && configapi.IsPasswordAuthenticator(identityProvider) {
			successHandlers = append(successHandlers, redirectSuccessHandler{})
			addedRedirectSuccessHandler = true
		}
	}

	return successHandlers
}

func (c *AuthConfig) getAuthenticationRequestHandler() (authenticator.Request, error) {
	var authRequestHandlers []authenticator.Request

	if c.getSessionAuth() != nil {
		authRequestHandlers = append(authRequestHandlers, c.getSessionAuth())
	}

	for _, identityProvider := range c.Options.IdentityProviders {
		identityMapper := identitymapper.NewAlwaysCreateUserIdentityToUserMapper(c.IdentityRegistry, c.UserRegistry)

		if configapi.IsPasswordAuthenticator(identityProvider) {
			passwordAuthenticator, err := c.getPasswordAuthenticator(identityProvider)
			if err != nil {
				return nil, err
			}
			authRequestHandlers = append(authRequestHandlers, basicauthrequest.NewBasicAuthAuthentication(passwordAuthenticator, true))

		} else {
			switch provider := identityProvider.Provider.Object.(type) {
			case (*configapi.RequestHeaderIdentityProvider):
				var authRequestHandler authenticator.Request

				authRequestConfig := &headerrequest.Config{
					UserNameHeaders: provider.Headers,
				}
				authRequestHandler = headerrequest.NewAuthenticator(identityProvider.Name, authRequestConfig, identityMapper)

				// Wrap with an x509 verifier
				if len(provider.ClientCA) > 0 {
					caData, err := ioutil.ReadFile(provider.ClientCA)
					if err != nil {
						return nil, fmt.Errorf("Error reading %s: %v", provider.ClientCA, err)
					}
					opts := x509request.DefaultVerifyOptions()
					opts.Roots = x509.NewCertPool()
					if ok := opts.Roots.AppendCertsFromPEM(caData); !ok {
						return nil, fmt.Errorf("Error loading certs from %s: %v", provider.ClientCA, err)
					}

					authRequestHandler = x509request.NewVerifier(opts, authRequestHandler)
				}
				authRequestHandlers = append(authRequestHandlers, authRequestHandler)

			}
		}
	}

	authRequestHandler := unionrequest.NewUnionAuthentication(authRequestHandlers...)
	return authRequestHandler, nil
}

func GetEtcdTokenAuthenticator(etcdHelper tools.EtcdHelper) (authenticator.Token, error) {
	oauthRegistry := oauthetcd.New(etcdHelper)
	return authnregistry.NewTokenAuthenticator(oauthRegistry), nil
}

func GetCSVTokenAuthenticator(path string) (authenticator.Token, error) {
	return filetoken.NewTokenAuthenticator(path)
}

// Captures the original request url as a "then" param in a redirect to a login flow
type redirector struct {
	RedirectURL string
	ThenParam   string
}

// AuthenticationRedirectNeeded redirects HTTP request to authorization URL
func (auth *redirector) AuthenticationRedirect(w http.ResponseWriter, req *http.Request) error {
	redirectURL, err := url.Parse(auth.RedirectURL)
	if err != nil {
		return err
	}
	if len(auth.ThenParam) != 0 {
		redirectURL.RawQuery = url.Values{
			auth.ThenParam: {req.URL.String()},
		}.Encode()
	}
	http.Redirect(w, req, redirectURL.String(), http.StatusFound)
	return nil
}

//
// Combines password auth, successful login callback, and "then" param redirection
//
type callbackPasswordAuthenticator struct {
	authenticator.Password
	handlers.AuthenticationSuccessHandler
}

// Redirects to the then param on successful authentication
type redirectSuccessHandler struct{}

// AuthenticationSuccess informs client when authentication was successful
func (redirectSuccessHandler) AuthenticationSucceeded(user kuser.Info, then string, w http.ResponseWriter, req *http.Request) (bool, error) {
	if len(then) == 0 {
		return false, fmt.Errorf("Auth succeeded, but no redirect existed - user=%#v", user)
	}

	http.Redirect(w, req, then, http.StatusFound)
	return true, nil
}

// authenticationHandlerFilter creates a filter object that will enforce authentication directly
func authenticationHandlerFilter(handler http.Handler, authenticator authenticator.Request, contextMapper kapi.RequestContextMapper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		user, ok, err := authenticator.AuthenticateRequest(req)
		if err != nil || !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		glog.V(6).Infof("user %v -> %v", user, req.URL)

		ctx, ok := contextMapper.Get(req)
		if !ok {
			http.Error(w, "Unable to find request context", http.StatusInternalServerError)
			return
		}
		if err := contextMapper.Update(req, kapi.WithUser(ctx, user)); err != nil {
			glog.V(4).Infof("Error setting authenticated context: %v", err)
			http.Error(w, "Unable to set authenticated request context", http.StatusInternalServerError)
			return
		}

		handler.ServeHTTP(w, req)
	})
}
