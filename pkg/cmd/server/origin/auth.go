package origin

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"code.google.com/p/go-uuid/uuid"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/RangelReale/osincli"
	"github.com/emicklei/go-restful"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/challenger/passwordchallenger"
	"github.com/openshift/origin/pkg/auth/authenticator/password/allowanypassword"
	"github.com/openshift/origin/pkg/auth/authenticator/password/basicauthpassword"
	"github.com/openshift/origin/pkg/auth/authenticator/request/basicauthrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/headerrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/unionrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/token/bearertoken"
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
)

var (
	// OSBrowserClientBase is used as a skeleton for building a Client.  We can't set the allowed redirecturis because we don't yet know the host:port of the auth server
	OSBrowserClientBase = oauthapi.Client{
		ObjectMeta: kapi.ObjectMeta{
			Name: "openshift-browser-client",
		},
		Secret: uuid.NewUUID().String(), // random secret so no one knows what it is ahead of time.  This still allows us to loop back for /requestToken
	}
	OSCliClientBase = oauthapi.Client{
		ObjectMeta: kapi.ObjectMeta{
			Name: "openshift-challenging-client",
		},
		Secret:                uuid.NewUUID().String(), // random secret so no one knows what it is ahead of time.  This still allows us to loop back for /requestToken
		RespondWithChallenges: true,
	}
)

type AuthConfig struct {
	MasterAddr     string
	SessionSecrets []string
	EtcdHelper     tools.EtcdHelper
}

// InstallAPI starts an OAuth2 server and registers the supported REST APIs
// into the provided mux, then returns an array of strings indicating what
// endpoints were started (these are format strings that will expect to be sent
// a single string value).
func (c *AuthConfig) InstallAPI(container *restful.Container) []string {
	oauthEtcd := oauthetcd.New(c.EtcdHelper)

	// TODO: register these items with go-restful
	mux := container.ServeMux

	authRequestHandler, authHandler := c.getAuthorizeAuthenticationHandlers(mux)

	storage := registrystorage.New(oauthEtcd, oauthEtcd, oauthEtcd, registry.NewUserConversion())
	config := osinserver.NewDefaultServerConfig()

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
		},
		osinserver.AccessHandlers{
			handlers.NewDenyAccessAuthenticator(),
		},
		osinserver.NewDefaultErrorHandler(),
	)
	server.Install(mux, OpenShiftOAuthAPIPrefix)

	CreateOrUpdateDefaultOAuthClients(c.MasterAddr, oauthEtcd)
	osOAuthClientConfig := c.NewOpenShiftOAuthClientConfig(&OSBrowserClientBase)
	osOAuthClientConfig.RedirectUrl = c.MasterAddr + OpenShiftOAuthAPIPrefix + tokenrequest.DisplayTokenEndpoint
	osOAuthClient, _ := osincli.NewClient(osOAuthClientConfig)
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
func (c *AuthConfig) NewOpenShiftOAuthClientConfig(client *oauthapi.Client) *osincli.ClientConfig {
	config := &osincli.ClientConfig{
		ClientId:                 client.Name,
		ClientSecret:             client.Secret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             c.MasterAddr + OpenShiftOAuthAPIPrefix + "/authorize",
		TokenUrl:                 c.MasterAddr + OpenShiftOAuthAPIPrefix + "/token",
		Scope:                    "",
	}
	return config
}

func CreateOrUpdateDefaultOAuthClients(masterAddr string, clientRegistry oauthclient.Registry) {
	clientsToEnsure := []*oauthapi.Client{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OSBrowserClientBase.Name,
			},
			Secret:                OSBrowserClientBase.Secret,
			RespondWithChallenges: OSBrowserClientBase.RespondWithChallenges,
			RedirectURIs:          []string{masterAddr + OpenShiftOAuthAPIPrefix + tokenrequest.DisplayTokenEndpoint},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OSCliClientBase.Name,
			},
			Secret:                OSCliClientBase.Secret,
			RespondWithChallenges: OSCliClientBase.RespondWithChallenges,
			RedirectURIs:          []string{masterAddr + OpenShiftOAuthAPIPrefix + tokenrequest.DisplayTokenEndpoint},
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

func (c *AuthConfig) getAuthorizeAuthenticationHandlers(mux cmdutil.Mux) (authenticator.Request, handlers.AuthenticationHandler) {
	// things based on sessionStore only work if they can share exactly the same instance of sessionStore
	// this results in really ugly initialization code as we have to pass this concept through to many disparate pieces
	// of the config that MIGHT need the information.  The first step to fixing it is to see how far it goes, so this
	// does not attempt to hide the ugly
	sessionStore := session.NewStore(c.SessionSecrets...)
	authRequestHandler := c.getAuthenticationRequestHandler(sessionStore)
	authHandler := c.getAuthenticationHandler(mux, sessionStore, handlers.EmptyError{})

	return authRequestHandler, authHandler
}

// getGrantHandler returns the object that handles approving or rejecting grant requests
func (c *AuthConfig) getGrantHandler(mux cmdutil.Mux, auth authenticator.Request, clientregistry clientregistry.Registry, authregistry clientauthorization.Registry) handlers.GrantHandler {
	var grantHandler handlers.GrantHandler
	grantHandlerType := env("ORIGIN_GRANT_HANDLER", "prompt")
	switch grantHandlerType {
	case "empty":
		grantHandler = handlers.NewEmptyGrant()
	case "auto":
		grantHandler = handlers.NewAutoGrant(authregistry)
	case "prompt":
		grantServer := grant.NewGrant(getCSRF(), auth, grant.DefaultFormRenderer, clientregistry, authregistry)
		grantServer.Install(mux, OpenShiftApprovePrefix)
		grantHandler = handlers.NewRedirectGrant(OpenShiftApprovePrefix)
	default:
		glog.Fatalf("No grant handler found that matches %v.  The oauth server cannot start!", grantHandlerType)
	}
	return grantHandler
}

func (c *AuthConfig) getAuthenticationHandler(mux cmdutil.Mux, sessionStore session.Store, errorHandler handlers.AuthenticationErrorHandler) handlers.AuthenticationHandler {
	successHandler := c.getAuthenticationSuccessHandler(sessionStore)

	// TODO presumeably we'll want either a list of what we've got or a way to describe a registry of these
	// hard-coded strings as a stand-in until it gets sorted out
	var authHandler handlers.AuthenticationHandler
	authHandlerType := env("ORIGIN_AUTH_HANDLER", "login")
	switch authHandlerType {
	case "google", "github":
		callbackPath := OpenShiftOAuthCallbackPrefix + "/" + authHandlerType
		userRegistry := useretcd.New(c.EtcdHelper, user.NewDefaultUserInitStrategy())
		identityMapper := identitymapper.NewAlwaysCreateUserIdentityToUserMapper(authHandlerType /*for now*/, userRegistry)

		var oauthProvider external.Provider
		if authHandlerType == "google" {
			oauthProvider = google.NewProvider(env("ORIGIN_GOOGLE_CLIENT_ID", ""), env("ORIGIN_GOOGLE_CLIENT_SECRET", ""))
		} else if authHandlerType == "github" {
			oauthProvider = github.NewProvider(env("ORIGIN_GITHUB_CLIENT_ID", ""), env("ORIGIN_GITHUB_CLIENT_SECRET", ""))
		}

		state := external.DefaultState()
		oauthHandler, err := external.NewExternalOAuthRedirector(oauthProvider, state, c.MasterAddr+callbackPath, successHandler, errorHandler, identityMapper)
		if err != nil {
			glog.Fatalf("unexpected error: %v", err)
		}

		mux.Handle(callbackPath, oauthHandler)
		authHandler = handlers.NewUnionAuthenticationHandler(nil, map[string]handlers.AuthenticationRedirector{authHandlerType: oauthHandler}, errorHandler)
	case "login":
		passwordAuth := c.getPasswordAuthenticator()
		authHandler = handlers.NewUnionAuthenticationHandler(
			map[string]handlers.AuthenticationChallenger{"login": passwordchallenger.NewBasicAuthChallenger("openshift")},
			map[string]handlers.AuthenticationRedirector{"login": &redirector{RedirectURL: OpenShiftLoginPrefix, ThenParam: "then"}},
			errorHandler,
		)
		login := login.NewLogin(getCSRF(), &callbackPasswordAuthenticator{passwordAuth, successHandler}, login.DefaultLoginFormRenderer)
		login.Install(mux, OpenShiftLoginPrefix)
	case "empty":
		authHandler = handlers.EmptyAuth{}
	default:
		glog.Fatalf("No AuthenticationHandler found that matches %v.  The oauth server cannot start!", authHandlerType)
	}

	return authHandler
}

func (c *AuthConfig) getPasswordAuthenticator() authenticator.Password {
	// TODO presumeably we'll want either a list of what we've got or a way to describe a registry of these
	// hard-coded strings as a stand-in until it gets sorted out
	passwordAuthType := env("ORIGIN_PASSWORD_AUTH_TYPE", "empty")
	userRegistry := useretcd.New(c.EtcdHelper, user.NewDefaultUserInitStrategy())
	identityMapper := identitymapper.NewAlwaysCreateUserIdentityToUserMapper(passwordAuthType /*for now*/, userRegistry)

	var passwordAuth authenticator.Password
	switch passwordAuthType {
	case "basic":
		basicAuthURL := env("ORIGIN_BASIC_AUTH_URL", "")
		if len(basicAuthURL) == 0 {
			glog.Fatalf("ORIGIN_BASIC_AUTH_URL is required to support basic password auth")
		}
		passwordAuth = basicauthpassword.New(basicAuthURL, identityMapper)
	case "empty":
		// Accepts any username and password
		passwordAuth = allowanypassword.New(identityMapper)
	default:
		glog.Fatalf("No password auth found that matches %v.  The oauth server cannot start!", passwordAuthType)
	}

	return passwordAuth
}

func (c *AuthConfig) getAuthenticationSuccessHandler(sessionStore session.Store) handlers.AuthenticationSuccessHandler {
	successHandlers := handlers.AuthenticationSuccessHandlers{}

	authRequestHandlerTypes := env("ORIGIN_AUTH_REQUEST_HANDLERS", "session")
	for _, currType := range strings.Split(authRequestHandlerTypes, ",") {
		currType = strings.TrimSpace(currType)
		switch currType {
		case "session":
			successHandlers = append(successHandlers, session.NewAuthenticator(sessionStore, "ssn"))
		}
	}

	authHandlerType := env("ORIGIN_AUTH_HANDLER", "login")
	switch authHandlerType {
	case "google", "github":
		successHandlers = append(successHandlers, external.DefaultState().(handlers.AuthenticationSuccessHandler))
	case "login":
		successHandlers = append(successHandlers, redirectSuccessHandler{})
	}

	return successHandlers
}

func (c *AuthConfig) getAuthenticationRequestHandlerFromType(authRequestHandlerType string, sessionStore session.Store) authenticator.Request {
	// TODO presumeably we'll want either a list of what we've got or a way to describe a registry of these
	// hard-coded strings as a stand-in until it gets sorted out
	var authRequestHandler authenticator.Request
	switch authRequestHandlerType {
	case "bearer":
		tokenAuthenticator, err := GetTokenAuthenticator(c.EtcdHelper)
		if err != nil {
			glog.Fatalf("Error creating TokenAuthenticator: %v.  The oauth server cannot start!", err)
		}
		authRequestHandler = bearertoken.New(tokenAuthenticator)
	case "requestheader":
		userRegistry := useretcd.New(c.EtcdHelper, user.NewDefaultUserInitStrategy())
		identityMapper := identitymapper.NewAlwaysCreateUserIdentityToUserMapper(authRequestHandlerType /*for now*/, userRegistry)
		authRequestHandler = headerrequest.NewAuthenticator(headerrequest.NewDefaultConfig(), identityMapper)
	case "basicauth":
		passwordAuthenticator := c.getPasswordAuthenticator()
		authRequestHandler = basicauthrequest.NewBasicAuthAuthentication(passwordAuthenticator)
	case "session":
		authRequestHandler = session.NewAuthenticator(sessionStore, "ssn")
	default:
		glog.Fatalf("No AuthenticationRequestHandler found that matches %v.  The oauth server cannot start!", authRequestHandlerType)
	}

	return authRequestHandler
}

func (c *AuthConfig) getAuthenticationRequestHandler(sessionStore session.Store) authenticator.Request {
	// TODO presumeably we'll want either a list of what we've got or a way to describe a registry of these
	// hard-coded strings as a stand-in until it gets sorted out
	var authRequestHandlers []authenticator.Request
	authRequestHandlerTypes := env("ORIGIN_AUTH_REQUEST_HANDLERS", "session")
	for _, currType := range strings.Split(authRequestHandlerTypes, ",") {
		currType = strings.TrimSpace(currType)

		authRequestHandlers = append(authRequestHandlers, c.getAuthenticationRequestHandlerFromType(currType, sessionStore))
	}

	authRequestHandler := unionrequest.NewUnionAuthentication(authRequestHandlers)
	return authRequestHandler
}

func GetTokenAuthenticator(etcdHelper tools.EtcdHelper) (authenticator.Token, error) {
	tokenAuthenticatorType := env("ORIGIN_AUTH_TOKEN_AUTHENTICATOR", "etcd")
	switch tokenAuthenticatorType {
	case "etcd":
		oauthRegistry := oauthetcd.New(etcdHelper)
		return authnregistry.NewTokenAuthenticator(oauthRegistry), nil
	case "file":
		return filetoken.NewTokenAuthenticator(env("ORIGIN_AUTH_FILE_TOKEN_AUTHENTICATOR_PATH", "authorizedTokens.csv"))
	default:
		return nil, fmt.Errorf("No TokenAuthenticator found that matches %v.  The oauth server cannot start!", tokenAuthenticatorType)
	}
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
func (redirectSuccessHandler) AuthenticationSucceeded(user api.UserInfo, then string, w http.ResponseWriter, req *http.Request) (bool, error) {
	if len(then) == 0 {
		return false, fmt.Errorf("Auth succeeded, but no redirect existed - user=%#v", user)
	}

	http.Redirect(w, req, then, http.StatusFound)
	return true, nil
}
