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

	"code.google.com/p/go-uuid/uuid"
	"github.com/RangelReale/osin"
	"github.com/RangelReale/osincli"
	"github.com/emicklei/go-restful"
	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	kuser "k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/challenger/passwordchallenger"
	"github.com/openshift/origin/pkg/auth/authenticator/challenger/placeholderchallenger"
	"github.com/openshift/origin/pkg/auth/authenticator/password/allowanypassword"
	"github.com/openshift/origin/pkg/auth/authenticator/password/basicauthpassword"
	"github.com/openshift/origin/pkg/auth/authenticator/password/denypassword"
	"github.com/openshift/origin/pkg/auth/authenticator/password/htpasswd"
	"github.com/openshift/origin/pkg/auth/authenticator/password/ldappassword"
	"github.com/openshift/origin/pkg/auth/authenticator/request/basicauthrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/headerrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/unionrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/x509request"
	"github.com/openshift/origin/pkg/auth/oauth/external"
	"github.com/openshift/origin/pkg/auth/oauth/external/github"
	"github.com/openshift/origin/pkg/auth/oauth/external/google"
	"github.com/openshift/origin/pkg/auth/oauth/external/openid"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/server/csrf"
	"github.com/openshift/origin/pkg/auth/server/grant"
	"github.com/openshift/origin/pkg/auth/server/login"
	"github.com/openshift/origin/pkg/auth/server/tokenrequest"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	accesstokenregistry "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	authorizetokenregistry "github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken"
	authorizetokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken/etcd"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	clientetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclient/etcd"
	clientauthregistry "github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization"
	clientauthetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization/etcd"
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
		Secret: uuid.New(), // random secret so no one knows what it is ahead of time.
	}
	// OSBrowserClientBase is used as a skeleton for building a Client.  We can't set the allowed redirecturis because we don't yet know the host:port of the auth server
	OSBrowserClientBase = oauthapi.OAuthClient{
		ObjectMeta: kapi.ObjectMeta{
			Name: "openshift-browser-client",
		},
		Secret: uuid.New(), // random secret so no one knows what it is ahead of time.  This still allows us to loop back for /requestToken
	}
	OSCliClientBase = oauthapi.OAuthClient{
		ObjectMeta: kapi.ObjectMeta{
			Name: "openshift-challenging-client",
		},
		Secret:                uuid.New(), // random secret so no one knows what it is ahead of time.  This still allows us to loop back for /requestToken
		RespondWithChallenges: true,
	}
)

// InstallAPI registers endpoints for an OAuth2 server into the provided mux,
// then returns an array of strings indicating what endpoints were started
// (these are format strings that will expect to be sent a single string value).
func (c *AuthConfig) InstallAPI(container *restful.Container) []string {
	// TODO: register into container
	mux := container.ServeMux

	accessTokenStorage := accesstokenetcd.NewREST(c.EtcdHelper)
	accessTokenRegistry := accesstokenregistry.NewRegistry(accessTokenStorage)
	authorizeTokenStorage := authorizetokenetcd.NewREST(c.EtcdHelper)
	authorizeTokenRegistry := authorizetokenregistry.NewRegistry(authorizeTokenStorage)
	clientStorage := clientetcd.NewREST(c.EtcdHelper)
	clientRegistry := clientregistry.NewRegistry(clientStorage)
	clientAuthStorage := clientauthetcd.NewREST(c.EtcdHelper)
	clientAuthRegistry := clientauthregistry.NewRegistry(clientAuthStorage)

	authRequestHandler, authHandler, authFinalizer, err := c.getAuthorizeAuthenticationHandlers(mux)
	if err != nil {
		glog.Fatal(err)
	}

	storage := registrystorage.New(accessTokenRegistry, authorizeTokenRegistry, clientRegistry, registry.NewUserConversion())
	config := osinserver.NewDefaultServerConfig()
	if c.Options.TokenConfig.AuthorizeTokenMaxAgeSeconds > 0 {
		config.AuthorizationExpiration = c.Options.TokenConfig.AuthorizeTokenMaxAgeSeconds
	}
	if c.Options.TokenConfig.AccessTokenMaxAgeSeconds > 0 {
		config.AccessExpiration = c.Options.TokenConfig.AccessTokenMaxAgeSeconds
	}

	grantChecker := registry.NewClientAuthorizationGrantChecker(clientAuthRegistry)
	grantHandler := c.getGrantHandler(mux, authRequestHandler, clientRegistry, clientAuthRegistry)

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

	CreateOrUpdateDefaultOAuthClients(c.Options.MasterPublicURL, c.AssetPublicAddresses, clientRegistry)
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

	tokenRequestEndpoints := tokenrequest.NewEndpoints(c.Options.MasterPublicURL, osOAuthClient)
	tokenRequestEndpoints.Install(mux, OpenShiftOAuthAPIPrefix)

	// glog.Infof("oauth server configured as: %#v", server)
	// glog.Infof("auth handler: %#v", authHandler)
	// glog.Infof("auth request handler: %#v", authRequestHandler)
	// glog.Infof("grant checker: %#v", grantChecker)
	// glog.Infof("grant handler: %#v", grantHandler)

	return []string{
		fmt.Sprintf("Started OAuth2 API at %%s%s", OpenShiftOAuthAPIPrefix),
		fmt.Sprintf("Started Login endpoint at %%s%s", OpenShiftLoginPrefix),
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
func OpenShiftOAuthTokenRequestURL(masterAddr string) string {
	return masterAddr + path.Join(OpenShiftOAuthAPIPrefix, tokenrequest.RequestTokenEndpoint)
}

func CreateOrUpdateDefaultOAuthClients(masterPublicAddr string, assetPublicAddresses []string, clientRegistry clientregistry.Registry) {
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
			RedirectURIs:          []string{masterPublicAddr + path.Join(OpenShiftOAuthAPIPrefix, tokenrequest.ImplicitTokenEndpoint)},
		},
	}

	ctx := kapi.NewContext()
	for _, currClient := range clientsToEnsure {
		existing, err := clientRegistry.GetClient(ctx, currClient.Name)
		if err == nil {
			// Update the existing resource version
			currClient.ResourceVersion = existing.ResourceVersion

			// Preserve redirects for clients other than the CLI client
			// The CLI client doesn't care about the redirect URL, just the token or error fragment
			if currClient.Name != OSCliClientBase.Name {
				// Add in any redirects from the existing one
				// This preserves any additional customized redirects in the default clients
				redirects := util.NewStringSet(currClient.RedirectURIs...)
				for _, redirect := range existing.RedirectURIs {
					if !redirects.Has(redirect) {
						currClient.RedirectURIs = append(currClient.RedirectURIs, redirect)
						redirects.Insert(redirect)
					}
				}
			}

			if _, err := clientRegistry.UpdateClient(ctx, currClient); err != nil {
				glog.Errorf("Error updating OAuthClient %v: %v", currClient.Name, err)
			}
		} else if kerrs.IsNotFound(err) {
			if _, err = clientRegistry.CreateClient(ctx, currClient); err != nil {
				glog.Errorf("Error creating OAuthClient %v: %v", currClient.Name, err)
			}
		} else {
			glog.Errorf("Error getting OAuthClient %v: %v", currClient.Name, err)
		}
	}
}

// getCSRF returns the object responsible for generating and checking CSRF tokens
func (c *AuthConfig) getCSRF() csrf.CSRF {
	secure := isHTTPS(c.Options.MasterPublicURL)
	return csrf.NewCookieCSRF("csrf", "/", "", secure, true)
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
func (c *AuthConfig) getGrantHandler(mux cmdutil.Mux, auth authenticator.Request, clientregistry clientregistry.Registry, authregistry clientauthregistry.Registry) handlers.GrantHandler {
	switch c.Options.GrantConfig.Method {
	case configapi.GrantHandlerDeny:
		return handlers.NewEmptyGrant()

	case configapi.GrantHandlerAuto:
		return handlers.NewAutoGrant()

	case configapi.GrantHandlerPrompt:
		grantServer := grant.NewGrant(c.getCSRF(), auth, grant.DefaultFormRenderer, clientregistry, authregistry)
		grantServer.Install(mux, OpenShiftApprovePrefix)
		return handlers.NewRedirectGrant(OpenShiftApprovePrefix)

	default:
		glog.Fatalf("No grant handler found that matches %v.  The oauth server cannot start!", c.Options.GrantConfig.Method)
	}

	return nil
}

// getAuthenticationFinalizer returns an authentication finalizer which is called just prior to writing a response to an authorization request
func (c *AuthConfig) getAuthenticationFinalizer() osinserver.AuthorizeHandler {
	if c.SessionAuth != nil {
		// The session needs to know the authorize flow is done so it can invalidate the session
		return osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, w http.ResponseWriter) (bool, error) {
			_ = c.SessionAuth.InvalidateAuthentication(w, ar.HttpRequest)
			return false, nil
		})
	}

	// Otherwise return a no-op finalizer
	return osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, w http.ResponseWriter) (bool, error) {
		return false, nil
	})
}

func (c *AuthConfig) getAuthenticationHandler(mux cmdutil.Mux, errorHandler handlers.AuthenticationErrorHandler) (handlers.AuthenticationHandler, error) {
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
				// Password auth requires:
				// 1. a session success handler (to remember you logged in)
				// 2. a redirectSuccessHandler (to go back to the "then" param)
				if c.SessionAuth == nil {
					return nil, errors.New("SessionAuth is required for password-based login")
				}
				passwordSuccessHandler := handlers.AuthenticationSuccessHandlers{c.SessionAuth, redirectSuccessHandler{}}

				redirectors["login"] = &redirector{RedirectURL: OpenShiftLoginPrefix, ThenParam: "then"}
				login := login.NewLogin(c.getCSRF(), &callbackPasswordAuthenticator{passwordAuth, passwordSuccessHandler}, login.DefaultLoginFormRenderer)
				login.Install(mux, OpenShiftLoginPrefix)
			}
			if identityProvider.UseAsChallenger {
				challengers["login"] = passwordchallenger.NewBasicAuthChallenger("openshift")
			}

		} else if configapi.IsOAuthIdentityProvider(identityProvider) {
			oauthProvider, err := c.getOAuthProvider(identityProvider)
			if err != nil {
				return nil, err
			}

			// Default state builder, combining CSRF and return URL handling
			state := external.CSRFRedirectingState(c.getCSRF())

			// OAuth auth requires
			// 1. a session success handler (to remember you logged in)
			// 2. a state success handler (to go back to the URL encoded in the state)
			if c.SessionAuth == nil {
				return nil, errors.New("SessionAuth is required for OAuth-based login")
			}
			oauthSuccessHandler := handlers.AuthenticationSuccessHandlers{c.SessionAuth, state}

			// If the specified errorHandler doesn't handle the login error, let the state error handler attempt to propagate specific errors back to the token requester
			oauthErrorHandler := handlers.AuthenticationErrorHandlers{errorHandler, state}

			callbackPath := path.Join(OpenShiftOAuthCallbackPrefix, identityProvider.Name)
			oauthHandler, err := external.NewExternalOAuthRedirector(oauthProvider, state, c.Options.MasterPublicURL+callbackPath, oauthSuccessHandler, oauthErrorHandler, identityMapper)
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

	if len(redirectors) > 0 && len(challengers) == 0 {
		// Add a default challenger that will warn and give a link to the web browser token-granting location
		challengers["placeholder"] = placeholderchallenger.New(OpenShiftOAuthTokenRequestURL(c.Options.MasterPublicURL))
	}

	authHandler := handlers.NewUnionAuthenticationHandler(challengers, redirectors, errorHandler)
	return authHandler, nil
}

func (c *AuthConfig) getOAuthProvider(identityProvider configapi.IdentityProvider) (external.Provider, error) {
	switch provider := identityProvider.Provider.Object.(type) {
	case (*configapi.GitHubIdentityProvider):
		return github.NewProvider(identityProvider.Name, provider.ClientID, provider.ClientSecret), nil

	case (*configapi.GoogleIdentityProvider):
		return google.NewProvider(identityProvider.Name, provider.ClientID, provider.ClientSecret, provider.HostedDomain)

	case (*configapi.OpenIDIdentityProvider):
		transport, err := cmdutil.TransportFor(provider.CA, "", "")
		if err != nil {
			return nil, err
		}

		// OpenID Connect requests MUST contain the openid scope value
		// http://openid.net/specs/openid-connect-core-1_0.html#AuthRequest
		scopes := util.NewStringSet("openid")
		scopes.Insert(provider.ExtraScopes...)

		config := openid.Config{
			ClientID:     provider.ClientID,
			ClientSecret: provider.ClientSecret,

			Scopes: scopes.List(),

			ExtraAuthorizeParameters: provider.ExtraAuthorizeParameters,

			AuthorizeURL: provider.URLs.Authorize,
			TokenURL:     provider.URLs.Token,
			UserInfoURL:  provider.URLs.UserInfo,

			IDClaims:                provider.Claims.ID,
			PreferredUsernameClaims: provider.Claims.PreferredUsername,
			EmailClaims:             provider.Claims.Email,
			NameClaims:              provider.Claims.Name,
		}

		return openid.NewProvider(identityProvider.Name, transport, config)

	default:
		return nil, fmt.Errorf("No OAuth provider found that matches %v.  The OAuth server cannot start!", identityProvider)
	}

}

func (c *AuthConfig) getPasswordAuthenticator(identityProvider configapi.IdentityProvider) (authenticator.Password, error) {
	identityMapper := identitymapper.NewAlwaysCreateUserIdentityToUserMapper(c.IdentityRegistry, c.UserRegistry)

	switch provider := identityProvider.Provider.Object.(type) {
	case (*configapi.AllowAllPasswordIdentityProvider):
		return allowanypassword.New(identityProvider.Name, identityMapper), nil

	case (*configapi.DenyAllPasswordIdentityProvider):
		return denypassword.New(), nil

	case (*configapi.LDAPPasswordIdentityProvider):
		url, err := ldappassword.ParseURL(provider.URL)
		if err != nil {
			return nil, fmt.Errorf("Error parsing LDAPPasswordIdentityProvider URL: %v", err)
		}

		tlsConfig := &tls.Config{}
		if len(provider.CA) > 0 {
			roots, err := util.CertPoolFromFile(provider.CA)
			if err != nil {
				return nil, fmt.Errorf("error loading cert pool from ca file %s: %v", provider.CA, err)
			}
			tlsConfig.RootCAs = roots
		}

		opts := ldappassword.Options{
			URL:          url,
			Insecure:     provider.Insecure,
			TLSConfig:    tlsConfig,
			BindDN:       provider.BindDN,
			BindPassword: provider.BindPassword,

			AttributeEmail:             provider.Attributes.Email,
			AttributeName:              provider.Attributes.Name,
			AttributeID:                provider.Attributes.ID,
			AttributePreferredUsername: provider.Attributes.PreferredUsername,
		}
		return ldappassword.New(identityProvider.Name, opts, identityMapper)

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
		return nil, fmt.Errorf("No password auth found that matches %v.  The OAuth server cannot start!", identityProvider)
	}

}

func (c *AuthConfig) getAuthenticationRequestHandler() (authenticator.Request, error) {
	var authRequestHandlers []authenticator.Request

	if c.SessionAuth != nil {
		authRequestHandlers = append(authRequestHandlers, c.SessionAuth)
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

// redirector captures the original request url as a "then" param in a redirect to a login flow
type redirector struct {
	RedirectURL string
	ThenParam   string
}

// AuthenticationRedirect redirects HTTP request to authorization URL
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

// callbackPasswordAuthenticator combines password auth, successful login callback,
// and "then" param redirection
type callbackPasswordAuthenticator struct {
	authenticator.Password
	handlers.AuthenticationSuccessHandler
}

// redirectSuccessHandler redirects to the then param on successful authentication
type redirectSuccessHandler struct{}

// AuthenticationSucceeded informs client when authentication was successful
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
