package origin

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/server/login"
	"github.com/openshift/origin/pkg/auth/server/session"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	"github.com/openshift/origin/pkg/oauth/server/osinserver/registrystorage"
)

const (
	OpenShiftOAuthAPIPrefix = "/oauth"
	OpenShiftLoginPrefix    = "/login"
)

type AuthConfig struct {
	SessionSecrets []string
	EtcdHelper     tools.EtcdHelper
}

// InstallAPI starts an OAuth2 server and registers the supported REST APIs
// into the provided mux, then returns an array of strings indicating what
// endpoints were started (these are format strings that will expect to be sent
// a single string value).
func (c *AuthConfig) InstallAPI(mux cmdutil.Mux) []string {
	oauthEtcd := oauthetcd.New(c.EtcdHelper)
	storage := registrystorage.New(oauthEtcd, oauthEtcd, oauthEtcd, registry.NewUserConversion())
	config := osinserver.NewDefaultServerConfig()
	sessionStore := session.NewStore(c.SessionSecrets...)
	sessionAuth := session.NewSessionAuthenticator(sessionStore, "ssn")

	server := osinserver.New(
		config,
		storage,
		osinserver.AuthorizeHandlers{
			handlers.NewAuthorizeAuthenticator(
				&redirectAuthHandler{RedirectURL: OpenShiftLoginPrefix, ThenParam: "then"},
				sessionAuth,
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

	login := login.NewLogin(emptyCsrf{}, &sessionPasswordAuthenticator{emptyPasswordAuth{}, sessionAuth}, login.DefaultLoginFormRenderer)
	login.Install(mux, OpenShiftLoginPrefix)

	return []string{
		fmt.Sprintf("Started OAuth2 API at %%s%s", OpenShiftOAuthAPIPrefix),
		fmt.Sprintf("Started login server at %%s%s", OpenShiftLoginPrefix),
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
// Saves the username of any successful password authentication in the session
//
type sessionPasswordAuthenticator struct {
	passwordAuthenticator authenticator.Password
	sessionAuthenticator  *session.SessionAuthenticator
}

// for login.PasswordAuthenticator
func (auth *sessionPasswordAuthenticator) AuthenticatePassword(user, password string) (api.UserInfo, bool, error) {
	return auth.passwordAuthenticator.AuthenticatePassword(user, password)
}

// for login.PasswordAuthenticator
func (auth *sessionPasswordAuthenticator) AuthenticationSucceeded(user api.UserInfo, then string, w http.ResponseWriter, req *http.Request) {
	err := auth.sessionAuthenticator.AuthenticationSucceeded(user, w, req)
	if err != nil {
		fmt.Fprintf(w, "<body>Could not save session, err=%#v</body>", err)
		return
	}

	if len(then) != 0 {
		http.Redirect(w, req, then, http.StatusFound)
	} else {
		fmt.Fprintf(w, "<body>PasswordAuthenticationSucceeded - user=%#v</body>", user)
	}
}
