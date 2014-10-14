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
	"github.com/openshift/origin/pkg/auth/authenticator/bearertoken"
	authfile "github.com/openshift/origin/pkg/auth/authenticator/file"
	"github.com/openshift/origin/pkg/auth/authenticator/requestheader"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/oauth/registry"
	authnregistry "github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/server/login"
	"github.com/openshift/origin/pkg/auth/server/session"

	"github.com/openshift/origin/pkg/auth/oauth/callbackhandlers/googlecallback"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	"github.com/openshift/origin/pkg/oauth/server/osinserver/registrystorage"
)

const (
	OpenShiftOAuthAPIPrefix      = "/oauth"
	OpenShiftLoginPrefix         = "/login"
	OpenShiftOAuthCallbackPrefix = "/oauth2callback"
)

type AuthConfig struct {
	SessionSecrets []string
	EtcdHelper     tools.EtcdHelper
}

func getGoogleClientId() string {
	return env("ORIGIN_GOOGLE_CLIENT_ID", "")
}

func getGoogleClientSecret() string {
	return env("ORIGIN_GOOGLE_CLIENT_SECRET", "")
}

// InstallAPI starts an OAuth2 server and registers the supported REST APIs
// into the provided mux, then returns an array of strings indicating what
// endpoints were started (these are format strings that will expect to be sent
// a single string value).
func (c *AuthConfig) InstallAPI(mux cmdutil.Mux) []string {
	oauthEtcd := oauthetcd.New(c.EtcdHelper)

	authRequestHandler := c.getAuthenticationRequestHandler()
	authHandler := c.getAuthenticationHandler()

	storage := registrystorage.New(oauthEtcd, oauthEtcd, oauthEtcd, registry.NewUserConversion())
	config := osinserver.NewDefaultServerConfig()

	server := osinserver.New(
		config,
		storage,
		osinserver.AuthorizeHandlers{
			handlers.NewAuthorizeAuthenticator(
				authHandler,
				authRequestHandler,
			),
			handlers.NewGrantCheck(
				registry.NewClientAuthorizationGrantChecker(oauthEtcd),
				emptyGrant{},
			),
		},
		osinserver.AccessHandlers{
			handlers.NewDenyAccessAuthenticator(),
		},
	)
	server.Install(mux, OpenShiftOAuthAPIPrefix)
	glog.Infof("oauth server configured as: %v", server)

	mux.Handle(OpenShiftOAuthCallbackPrefix+"/google", &googlecallback.OauthCallbackHandler{oauthetcd.New(c.EtcdHelper), getGoogleClientId(), getGoogleClientSecret()})

	// Check if the authentication handler wants to be told when we authenticated
	successHandler, _ := authRequestHandler.(handlers.AuthenticationSucceeded)
	login := login.NewLogin(emptyCsrf{}, &callbackPasswordAuthenticator{emptyPasswordAuth{}, successHandler}, login.DefaultLoginFormRenderer)
	login.Install(mux, OpenShiftLoginPrefix)

	return []string{
		fmt.Sprintf("Started OAuth2 API at %%s%s", OpenShiftOAuthAPIPrefix),
		fmt.Sprintf("Started login server at %%s%s", OpenShiftLoginPrefix),
	}
}

func (c *AuthConfig) getAuthenticationHandler() handlers.AuthenticationHandler {
	// TODO presumeably we'll want either a list of what we've got or a way to describe a registry of these
	// hard-coded strings as a stand-in until it gets sorted out
	var authHandler handlers.AuthenticationHandler
	authHandlerType := env("ORIGIN_AUTH_HANDLER", "empty")
	switch authHandlerType {
	case "google":
		authHandler = &googlecallback.GoogleAuthenticationHandler{"http://localhost:8080" + OpenShiftOAuthCallbackPrefix + "/google", getGoogleClientId()}
	case "password":
		authHandler = &redirectAuthHandler{RedirectURL: OpenShiftLoginPrefix, ThenParam: "then"}
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
			glog.Fatalf("Error creating TokenAutenticator: %v.  The oauth server cannot start!", err)
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
		return nil, errors.New(fmt.Sprintf("No TokenAutenticator found that matches %v.  The oauth server cannot start!", tokenAuthenticatorType))
	}
}

type emptyAuth struct{}

func (emptyAuth) AuthenticationNeeded(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<body>AuthenticationNeeded - not implemented</body>")
}
func (emptyAuth) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<body>AuthenticationError - %s</body>", err)
}
func (emptyAuth) String() string {
	return "emptyAuth"
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

func (auth *redirectAuthHandler) String() string {
	return fmt.Sprintf("redirectAuth{url:%s, then:%s}", auth.RedirectURL, auth.ThenParam)
}

type emptyGrant struct{}

func (emptyGrant) GrantNeeded(grant *api.Grant, w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<body>GrantNeeded - not implemented<pre>%#v</pre></body>", grant)
}

func (emptyGrant) GrantError(err error, w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<body>GrantError - %s</body>", err)
}

type emptyCsrf struct{}

func (emptyCsrf) Generate() (string, error) {
	return "", nil
}

func (emptyCsrf) Check(string) (bool, error) {
	return true, nil
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
	password authenticator.Password
	success  handlers.AuthenticationSucceeded
}

// for login.PasswordAuthenticator
func (auth *callbackPasswordAuthenticator) AuthenticatePassword(user, password string) (api.UserInfo, bool, error) {
	return auth.password.AuthenticatePassword(user, password)
}

// for login.PasswordAuthenticator
func (auth *callbackPasswordAuthenticator) AuthenticationSucceeded(user api.UserInfo, then string, w http.ResponseWriter, req *http.Request) {
	if auth.success != nil {
		err := auth.success.AuthenticationSucceeded(user, w, req)
		if err != nil {
			fmt.Fprintf(w, "<body>Could not save session, err=%#v</body>", err)
			return
		}
	}

	if len(then) != 0 {
		http.Redirect(w, req, then, http.StatusFound)
	} else {
		fmt.Fprintf(w, "<body>PasswordAuthenticationSucceeded - user=%#v</body>", user)
	}
}
