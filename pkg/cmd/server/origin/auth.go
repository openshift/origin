package origin

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/basicauthpassword"
	"github.com/openshift/origin/pkg/auth/authenticator/bearertoken"
	authfile "github.com/openshift/origin/pkg/auth/authenticator/file"
	"github.com/openshift/origin/pkg/auth/authenticator/requestheader"
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

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/client"
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
func (c *AuthConfig) InstallAPI(mux cmdutil.Mux) []string {
	oauthEtcd := oauthetcd.New(c.EtcdHelper)

	authRequestHandler := c.getAuthenticationRequestHandler()

	// Check if the authentication handler wants to be told when we authenticated
	success, ok := authRequestHandler.(handlers.AuthenticationSuccessHandler)
	if !ok {
		success = emptySuccess{}
	}
	authHandler := c.getAuthenticationHandler(mux, success, emptyError{})

	storage := registrystorage.New(oauthEtcd, oauthEtcd, oauthEtcd, registry.NewUserConversion())
	config := osinserver.NewDefaultServerConfig()

	grantChecker := registry.NewClientAuthorizationGrantChecker(oauthEtcd)
	grantHandler := c.getGrantHandler(mux, authRequestHandler, oauthEtcd, oauthEtcd)

	server := osinserver.New(
		config,
		storage,
		osinserver.AuthorizeHandlers{
			handlers.NewAuthorizeAuthenticator(
				authHandler,
				authRequestHandler,
			),
			handlers.NewGrantCheck(
				grantChecker,
				grantHandler,
			),
		},
		osinserver.AccessHandlers{
			handlers.NewDenyAccessAuthenticator(),
		},
	)
	server.Install(mux, OpenShiftOAuthAPIPrefix)
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

// getCSRF returns the object responsible for generating and checking CSRF tokens
func getCSRF() csrf.CSRF {
	return csrf.NewCookieCSRF("csrf", "/", "", false, false)
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
		grantServer := grant.NewGrant(getCSRF(), auth, grant.DefaultGrantFormRenderer, clientregistry, authregistry)
		grantServer.Install(mux, OpenShiftApprovePrefix)
		grantHandler = handlers.NewRedirectGrant(OpenShiftApprovePrefix)
	default:
		glog.Fatalf("No grant handler found that matches %v.  The oauth server cannot start!", grantHandlerType)
	}
	return grantHandler
}

func (c *AuthConfig) getAuthenticationHandler(mux cmdutil.Mux, success handlers.AuthenticationSuccessHandler, error handlers.AuthenticationErrorHandler) handlers.AuthenticationHandler {
	// TODO presumeably we'll want either a list of what we've got or a way to describe a registry of these
	// hard-coded strings as a stand-in until it gets sorted out
	var authHandler handlers.AuthenticationHandler
	authHandlerType := env("ORIGIN_AUTH_HANDLER", "login")
	switch authHandlerType {
	case "google", "github":
		callbackPath := OpenShiftOAuthCallbackPrefix + "/" + authHandlerType

		var oauthProvider external.Provider
		if authHandlerType == "google" {
			oauthProvider = google.NewProvider(env("ORIGIN_GOOGLE_CLIENT_ID", ""), env("ORIGIN_GOOGLE_CLIENT_SECRET", ""))
		} else if authHandlerType == "github" {
			oauthProvider = github.NewProvider(env("ORIGIN_GITHUB_CLIENT_ID", ""), env("ORIGIN_GITHUB_CLIENT_SECRET", ""))
		}

		state := external.DefaultState()
		success := handlers.AuthenticationSuccessHandlers{success, state.(handlers.AuthenticationSuccessHandler)}
		oauthHandler, err := external.NewHandler(oauthProvider, state, c.MasterAddr+callbackPath, success, error)
		if err != nil {
			glog.Fatalf("unexpected error: %v", err)
		}

		mux.Handle(callbackPath, oauthHandler)
		authHandler = oauthHandler
	case "login":
		var passwordAuth authenticator.Password

		passwordAuthType := env("ORIGIN_PASSWORD_AUTH_TYPE", "empty")
		switch passwordAuthType {
		case "basic":
			basicAuthURL := env("ORIGIN_BASIC_AUTH_URL", "")
			if len(basicAuthURL) == 0 {
				glog.Fatalf("ORIGIN_BASIC_AUTH_URL is required to support basic password auth")
			}
			passwordAuth = basicauthpassword.New(basicAuthURL)
		case "empty":
			// Accepts any username and password
			passwordAuth = emptyPasswordAuth{}
		default:
			glog.Fatalf("No password auth found that matches %v.  The oauth server cannot start!", passwordAuthType)
		}

		authHandler = &redirectAuthHandler{RedirectURL: OpenShiftLoginPrefix, ThenParam: "then"}
		success := handlers.AuthenticationSuccessHandlers{success, redirectSuccessHandler{}}
		login := login.NewLogin(getCSRF(), &callbackPasswordAuthenticator{passwordAuth, success}, login.DefaultLoginFormRenderer)
		login.Install(mux, OpenShiftLoginPrefix)
	case "empty":
		authHandler = emptyAuth{}
	default:
		glog.Fatalf("No AuthenticationHandler found that matches %v.  The oauth server cannot start!", authHandlerType)
	}

	return authHandler
}

func (c *AuthConfig) getAuthenticationRequestHandler() authenticator.Request {
	// TODO presumeably we'll want either a list of what we've got or a way to describe a registry of these
	// hard-coded strings as a stand-in until it gets sorted out
	var authRequestHandler authenticator.Request
	authRequestHandlerType := env("ORIGIN_AUTH_REQUEST_HANDLER", "session")
	switch authRequestHandlerType {
	case "bearer":
		tokenAuthenticator, err := GetTokenAuthenticator(c.EtcdHelper)
		if err != nil {
			glog.Fatalf("Error creating TokenAuthenticator: %v.  The oauth server cannot start!", err)
		}
		authRequestHandler = bearertoken.New(tokenAuthenticator)
	case "requestheader":
		authRequestHandler = requestheader.NewAuthenticator(requestheader.NewDefaultConfig())
	case "session":
		sessionStore := session.NewStore(c.SessionSecrets...)
		authRequestHandler = session.NewSessionAuthenticator(sessionStore, "ssn")
	default:
		glog.Fatalf("No AuthenticationRequestHandler found that matches %v.  The oauth server cannot start!", authRequestHandlerType)
	}

	return authRequestHandler
}

func GetTokenAuthenticator(etcdHelper tools.EtcdHelper) (authenticator.Token, error) {
	tokenAuthenticatorType := env("ORIGIN_AUTH_TOKEN_AUTHENTICATOR", "etcd")
	switch tokenAuthenticatorType {
	case "etcd":
		oauthRegistry := oauthetcd.New(etcdHelper)
		return authnregistry.NewTokenAuthenticator(oauthRegistry), nil
	case "file":
		return authfile.NewTokenAuthenticator(env("ORIGIN_AUTH_FILE_TOKEN_AUTHENTICATOR_PATH", "authorizedTokens.csv"))
	default:
		return nil, errors.New(fmt.Sprintf("No TokenAuthenticator found that matches %v.  The oauth server cannot start!", tokenAuthenticatorType))
	}
}

type emptyAuth struct{}

func (emptyAuth) AuthenticationNeeded(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<body>AuthenticationNeeded - not implemented</body>")
}
func (emptyAuth) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<body>AuthenticationError - %s</body>", err)
}

// Captures the original request url as a "then" param in a redirect to a login flow
type redirectAuthHandler struct {
	RedirectURL string
	ThenParam   string
}

func (auth *redirectAuthHandler) AuthenticationNeeded(w http.ResponseWriter, req *http.Request) {
	redirectURL, err := url.Parse(auth.RedirectURL)
	if err != nil {
		auth.AuthenticationError(err, w, req)
		return
	}
	if len(auth.ThenParam) != 0 {
		redirectURL.RawQuery = url.Values{
			auth.ThenParam: {req.URL.String()},
		}.Encode()
	}
	http.Redirect(w, req, redirectURL.String(), http.StatusFound)
}

func (auth *redirectAuthHandler) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, "<body>AuthenticationError - %s</body>", err)
}

//
// Approves any login attempt with non-blank username and password
//
type emptyPasswordAuth struct{}

func (emptyPasswordAuth) AuthenticatePassword(user, password string) (api.UserInfo, bool, error) {
	if user == "" || password == "" {
		return nil, false, nil
	}
	return &api.DefaultUserInfo{
		Name: user,
	}, true, nil
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

func (redirectSuccessHandler) AuthenticationSucceeded(user api.UserInfo, then string, w http.ResponseWriter, req *http.Request) error {
	if len(then) == 0 {
		return fmt.Errorf("Auth succeeded, but no redirect existed - user=%#v", user)
	}

	http.Redirect(w, req, then, http.StatusFound)
	return nil
}

type emptySuccess struct{}

func (emptySuccess) AuthenticationSucceeded(user api.UserInfo, state string, w http.ResponseWriter, req *http.Request) error {
	glog.V(4).Infof("AuthenticationSucceeded: %v (state=%s)", user, state)
	return nil
}

type emptyError struct{}

func (emptyError) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) {
	glog.V(4).Infof("AuthenticationError: %v", err)
}
