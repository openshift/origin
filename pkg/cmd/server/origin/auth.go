package origin

import (
	"crypto/tls"
	"crypto/x509"
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
	"github.com/openshift/origin/pkg/auth/authenticator/request/bearertoken"
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
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/client"
	oauthclient "github.com/openshift/origin/pkg/oauth/registry/client"
	"github.com/openshift/origin/pkg/oauth/registry/clientauthorization"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	"github.com/openshift/origin/pkg/oauth/server/osinserver/registrystorage"
	"github.com/openshift/origin/pkg/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/etcd"
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

type AuthRequestHandlerType string

const (
	// AuthRequestHandlerBearer validates a passed "Authorization: Bearer" token, using the specified TokenStore
	AuthRequestHandlerBearer AuthRequestHandlerType = "bearer"
	// AuthRequestHandlerRequestHeader treats any request with a value in one of the RequestHeaders headers as authenticated
	AuthRequestHandlerRequestHeader AuthRequestHandlerType = "requestheader"
	// AuthRequestHandlerBasicAuth validates a passed "Authorization: Basic" header using the specified PasswordAuth
	AuthRequestHandlerBasicAuth AuthRequestHandlerType = "basicauth"
	// AuthRequestHandlerSession authenticates requests containing user information in the request session
	AuthRequestHandlerSession AuthRequestHandlerType = "session"
)

type AuthHandlerType string

const (
	// AuthHandlerLogin redirects unauthenticated requests to a login page, or sends a www-authenticate challenge. Logins are validated using the specified PasswordAuth
	AuthHandlerLogin AuthHandlerType = "login"
	// AuthHandlerGithub redirects unauthenticated requests to GitHub to request an OAuth token.
	AuthHandlerGithub AuthHandlerType = "github"
	// AuthHandlerGoogle redirects unauthenticated requests to Google to request an OAuth token.
	AuthHandlerGoogle AuthHandlerType = "google"
	// AuthHandlerDeny treats unauthenticated requests as failures
	AuthHandlerDeny AuthHandlerType = "deny"
)

type GrantHandlerType string

const (
	// GrantHandlerAuto auto-approves client authorization grant requests
	GrantHandlerAuto GrantHandlerType = "auto"
	// GrantHandlerPrompt prompts the user to approve new client authorization grant requests
	GrantHandlerPrompt GrantHandlerType = "prompt"
	// GrantHandlerDeny auto-denies client authorization grant requests
	GrantHandlerDeny GrantHandlerType = "deny"
)

type PasswordAuthType string

const (
	// PasswordAuthAnyPassword treats any non-empty username and password combination as a successful authentication
	PasswordAuthAnyPassword PasswordAuthType = "anypassword"
	// PasswordAuthBasicAuthURL validates password credentials by making a request to a remote url using basic auth. See basicauthpassword.Authenticator
	PasswordAuthBasicAuthURL PasswordAuthType = "basicauthurl"
	// PasswordAuthHTPasswd validates usernames and passwords against an htpasswd file
	PasswordAuthHTPasswd PasswordAuthType = "htpasswd"
	// PasswordAuthDeny treats any username and password combination as an unsuccessful authentication
	PasswordAuthDeny PasswordAuthType = "deny"
)

type TokenStoreType string

const (
	// Validate bearer tokens by looking in the OAuth access token registry
	TokenStoreOAuth TokenStoreType = "oauth"
	// Validate bearer tokens by looking in a CSV file located at the specified TokenFilePath
	TokenStoreFile TokenStoreType = "file"
)

func ParseAuthRequestHandlerTypes(types string) []AuthRequestHandlerType {
	handlerTypes := []AuthRequestHandlerType{}
	for _, handlerType := range strings.Split(types, ",") {
		trimmedType := AuthRequestHandlerType(strings.TrimSpace(handlerType))
		switch trimmedType {
		case AuthRequestHandlerBearer, AuthRequestHandlerRequestHeader, AuthRequestHandlerBasicAuth, AuthRequestHandlerSession:
			handlerTypes = append(handlerTypes, trimmedType)
		default:
			glog.Fatalf("Unrecognized request handler type: %s", trimmedType)
		}
	}
	return handlerTypes
}

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
	if c.AuthorizeTokenMaxAgeSeconds > 0 {
		config.AuthorizationExpiration = c.AuthorizeTokenMaxAgeSeconds
	}
	if c.AccessTokenMaxAgeSeconds > 0 {
		config.AccessExpiration = c.AccessTokenMaxAgeSeconds
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

	CreateOrUpdateDefaultOAuthClients(c.MasterPublicAddr, c.AssetPublicAddresses, oauthEtcd)
	osOAuthClientConfig := c.NewOpenShiftOAuthClientConfig(&OSBrowserClientBase)
	osOAuthClientConfig.RedirectUrl = c.MasterPublicAddr + path.Join(OpenShiftOAuthAPIPrefix, tokenrequest.DisplayTokenEndpoint)

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
		AuthorizeUrl:             OpenShiftOAuthAuthorizeURL(c.MasterPublicAddr),
		TokenUrl:                 OpenShiftOAuthTokenURL(c.MasterPublicAddr),
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
	if c.sessionAuth == nil {
		sessionStore := session.NewStore(int(c.SessionMaxAgeSeconds), c.SessionSecrets...)
		c.sessionAuth = session.NewAuthenticator(sessionStore, c.SessionName)
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
	authFinalizer, err := c.getAuthenticationFinalizer()
	if err != nil {
		return nil, nil, nil, err
	}

	return authRequestHandler, authHandler, authFinalizer, nil
}

// getGrantHandler returns the object that handles approving or rejecting grant requests
func (c *AuthConfig) getGrantHandler(mux cmdutil.Mux, auth authenticator.Request, clientregistry clientregistry.Registry, authregistry clientauthorization.Registry) handlers.GrantHandler {
	var grantHandler handlers.GrantHandler
	grantHandlerType := c.GrantHandler
	switch grantHandlerType {
	case GrantHandlerDeny:
		grantHandler = handlers.NewEmptyGrant()
	case GrantHandlerAuto:
		grantHandler = handlers.NewAutoGrant()
	case GrantHandlerPrompt:
		grantServer := grant.NewGrant(getCSRF(), auth, grant.DefaultFormRenderer, clientregistry, authregistry)
		grantServer.Install(mux, OpenShiftApprovePrefix)
		grantHandler = handlers.NewRedirectGrant(OpenShiftApprovePrefix)
	default:
		glog.Fatalf("No grant handler found that matches %v.  The oauth server cannot start!", grantHandlerType)
	}
	return grantHandler
}

// getAuthenticationFinalizer returns an authentication finalizer which is called just prior to writing a response to an authorization request
func (c *AuthConfig) getAuthenticationFinalizer() (osinserver.AuthorizeHandler, error) {
	for _, requestHandler := range c.AuthRequestHandlers {
		switch requestHandler {
		case AuthRequestHandlerSession:
			// The session needs to know the authorize flow is done so it can invalidate the session
			return osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, w http.ResponseWriter) (bool, error) {
				_ = c.getSessionAuth().InvalidateAuthentication(w, ar.HttpRequest)
				return false, nil
			}), nil
		}
	}

	// Otherwise return a no-op finalizer
	return osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, w http.ResponseWriter) (bool, error) {
		return false, nil
	}), nil
}

func (c *AuthConfig) getAuthenticationHandler(mux cmdutil.Mux, errorHandler handlers.AuthenticationErrorHandler) (handlers.AuthenticationHandler, error) {
	successHandler := c.getAuthenticationSuccessHandler()

	// TODO presumably we'll want either a list of what we've got or a way to describe a registry of these
	// hard-coded strings as a stand-in until it gets sorted out
	var authHandler handlers.AuthenticationHandler
	authHandlerType := c.AuthHandler
	switch authHandlerType {
	case AuthHandlerGithub, AuthHandlerGoogle:
		callbackPath := path.Join(OpenShiftOAuthCallbackPrefix, string(authHandlerType))
		userRegistry := useretcd.New(c.EtcdHelper, user.NewDefaultUserInitStrategy())
		identityMapper := identitymapper.NewAlwaysCreateUserIdentityToUserMapper(string(authHandlerType) /*for now*/, userRegistry)

		var oauthProvider external.Provider
		if authHandlerType == AuthHandlerGoogle {
			oauthProvider = google.NewProvider(c.GoogleClientID, c.GoogleClientSecret)
		} else if authHandlerType == AuthHandlerGithub {
			oauthProvider = github.NewProvider(c.GithubClientID, c.GithubClientSecret)
		}

		state := external.DefaultState()
		oauthHandler, err := external.NewExternalOAuthRedirector(oauthProvider, state, c.MasterPublicAddr+callbackPath, successHandler, errorHandler, identityMapper)
		if err != nil {
			return nil, fmt.Errorf("unexpected error: %v", err)
		}

		mux.Handle(callbackPath, oauthHandler)
		authHandler = handlers.NewUnionAuthenticationHandler(nil, map[string]handlers.AuthenticationRedirector{string(authHandlerType): oauthHandler}, errorHandler)
	case AuthHandlerLogin:
		passwordAuth, err := c.getPasswordAuthenticator()
		if err != nil {
			return nil, err
		}
		authHandler = handlers.NewUnionAuthenticationHandler(
			map[string]handlers.AuthenticationChallenger{"login": passwordchallenger.NewBasicAuthChallenger("openshift")},
			map[string]handlers.AuthenticationRedirector{"login": &redirector{RedirectURL: OpenShiftLoginPrefix, ThenParam: "then"}},
			errorHandler,
		)
		login := login.NewLogin(getCSRF(), &callbackPasswordAuthenticator{passwordAuth, successHandler}, login.DefaultLoginFormRenderer)
		login.Install(mux, OpenShiftLoginPrefix)
	case AuthHandlerDeny:
		authHandler = handlers.EmptyAuth{}
	default:
		return nil, fmt.Errorf("No AuthenticationHandler found that matches %v.  The oauth server cannot start!", authHandlerType)
	}

	return authHandler, nil
}

func (c *AuthConfig) getPasswordAuthenticator() (authenticator.Password, error) {
	// TODO presumably we'll want either a list of what we've got or a way to describe a registry of these
	// hard-coded strings as a stand-in until it gets sorted out
	passwordAuthType := c.PasswordAuth
	userRegistry := useretcd.New(c.EtcdHelper, user.NewDefaultUserInitStrategy())
	identityMapper := identitymapper.NewAlwaysCreateUserIdentityToUserMapper(string(passwordAuthType) /*for now*/, userRegistry)

	var passwordAuth authenticator.Password
	switch passwordAuthType {
	case PasswordAuthBasicAuthURL:
		basicAuthURL := c.BasicAuthURL
		if len(basicAuthURL) == 0 {
			return nil, fmt.Errorf("BasicAuthURL is required to support basic password auth")
		}
		passwordAuth = basicauthpassword.New(basicAuthURL, identityMapper)
	case PasswordAuthAnyPassword:
		// Accepts any username and password
		passwordAuth = allowanypassword.New(identityMapper)
	case PasswordAuthDeny:
		// Deny any username and password
		passwordAuth = denypassword.New()
	case PasswordAuthHTPasswd:
		htpasswdFile := c.HTPasswdFile
		if len(htpasswdFile) == 0 {
			return nil, fmt.Errorf("HTPasswdFile is required to support htpasswd auth")
		}
		if htpasswordAuth, err := htpasswd.New(htpasswdFile, identityMapper); err != nil {
			return nil, fmt.Errorf("Error loading htpasswd file %s: %v", htpasswdFile, err)
		} else {
			passwordAuth = htpasswordAuth
		}

	default:
		return nil, fmt.Errorf("No password auth found that matches %v.  The oauth server cannot start!", passwordAuthType)
	}

	return passwordAuth, nil
}

func (c *AuthConfig) getAuthenticationSuccessHandler() handlers.AuthenticationSuccessHandler {
	successHandlers := handlers.AuthenticationSuccessHandlers{}

	for _, requestHandler := range c.AuthRequestHandlers {
		switch requestHandler {
		case AuthRequestHandlerSession:
			// The session needs to know so it can write auth info into the session
			successHandlers = append(successHandlers, c.getSessionAuth())
		}
	}

	switch c.AuthHandler {
	case AuthHandlerGithub, AuthHandlerGoogle:
		successHandlers = append(successHandlers, external.DefaultState().(handlers.AuthenticationSuccessHandler))
	case AuthHandlerLogin:
		successHandlers = append(successHandlers, redirectSuccessHandler{})
	}

	return successHandlers
}

func (c *AuthConfig) getAuthenticationRequestHandlerFromType(authRequestHandlerType AuthRequestHandlerType) (authenticator.Request, error) {
	// TODO presumably we'll want either a list of what we've got or a way to describe a registry of these
	// hard-coded strings as a stand-in until it gets sorted out
	var authRequestHandler authenticator.Request
	switch authRequestHandlerType {
	case AuthRequestHandlerBearer:
		switch c.TokenStore {
		case TokenStoreOAuth:
			tokenAuthenticator, err := GetEtcdTokenAuthenticator(c.EtcdHelper)
			if err != nil {
				return nil, fmt.Errorf("Error creating TokenAuthenticator: %v.  The oauth server cannot start!", err)
			}
			authRequestHandler = bearertoken.New(tokenAuthenticator, true)
		case TokenStoreFile:
			tokenAuthenticator, err := GetCSVTokenAuthenticator(c.TokenFilePath)
			if err != nil {
				return nil, fmt.Errorf("Error creating TokenAuthenticator: %v.  The oauth server cannot start!", err)
			}
			authRequestHandler = bearertoken.New(tokenAuthenticator, true)
		default:
			return nil, fmt.Errorf("Unknown TokenStore %s. Must be oauth or file.  The oauth server cannot start!", c.TokenStore)
		}
	case AuthRequestHandlerRequestHeader:
		userRegistry := useretcd.New(c.EtcdHelper, user.NewDefaultUserInitStrategy())
		identityMapper := identitymapper.NewAlwaysCreateUserIdentityToUserMapper(string(authRequestHandlerType) /*for now*/, userRegistry)
		authRequestConfig := &headerrequest.Config{
			UserNameHeaders: c.RequestHeaders,
		}
		authRequestHandler = headerrequest.NewAuthenticator(authRequestConfig, identityMapper)

		// Wrap with an x509 verifier
		if len(c.RequestHeaderCAFile) > 0 {
			caData, err := ioutil.ReadFile(c.RequestHeaderCAFile)
			if err != nil {
				return nil, fmt.Errorf("Error reading %s: %v", c.RequestHeaderCAFile, err)
			}
			opts := x509request.DefaultVerifyOptions()
			opts.Roots = x509.NewCertPool()
			if ok := opts.Roots.AppendCertsFromPEM(caData); !ok {
				return nil, fmt.Errorf("Error loading certs from %s: %v", c.RequestHeaderCAFile, err)
			}

			authRequestHandler = x509request.NewVerifier(opts, authRequestHandler)
		}
	case AuthRequestHandlerBasicAuth:
		passwordAuthenticator, err := c.getPasswordAuthenticator()
		if err != nil {
			return nil, err
		}
		authRequestHandler = basicauthrequest.NewBasicAuthAuthentication(passwordAuthenticator, true)
	case AuthRequestHandlerSession:
		authRequestHandler = c.getSessionAuth()
	default:
		return nil, fmt.Errorf("No AuthenticationRequestHandler found that matches %v.  The oauth server cannot start!", authRequestHandlerType)
	}

	return authRequestHandler, nil
}

func (c *AuthConfig) getAuthenticationRequestHandler() (authenticator.Request, error) {
	// TODO presumably we'll want either a list of what we've got or a way to describe a registry of these
	// hard-coded strings as a stand-in until it gets sorted out
	var authRequestHandlers []authenticator.Request
	for _, requestHandler := range c.AuthRequestHandlers {
		authRequestHandler, err := c.getAuthenticationRequestHandlerFromType(requestHandler)
		if err != nil {
			return nil, err
		}
		authRequestHandlers = append(authRequestHandlers, authRequestHandler)
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
