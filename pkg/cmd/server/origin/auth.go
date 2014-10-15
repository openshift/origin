package origin

import (
	"fmt"
	"net/http"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/server/session"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	"github.com/openshift/origin/pkg/oauth/server/osinserver/registrystorage"
)

const (
	OpenShiftOAuthAPIPrefix = "/oauth"
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
				emptyAuth{},
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

	return []string{
		fmt.Sprintf("Started OAuth2 API at %%s%s", OpenShiftOAuthAPIPrefix),
	}
}

type emptyAuth struct{}

func (emptyAuth) AuthenticationNeeded(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<body>AuthenticationNeeded - not implemented</body>")
}
func (emptyAuth) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<body>AuthenticationError - %s</body>", err)
}

type emptyGrant struct{}

func (emptyGrant) GrantNeeded(grant *api.Grant, w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<body>GrantNeeded - not implemented<pre>%#v</pre></body>", grant)
}

func (emptyGrant) GrantError(err error, w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<body>GrantError - %s</body>", err)
}
