// +build integration,!no-etcd

package integration

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/tools/etcdtest"

	// for osinserver setup.
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/auth/authenticator/challenger/passwordchallenger"
	"github.com/openshift/origin/pkg/auth/authenticator/password/allowanypassword"
	"github.com/openshift/origin/pkg/auth/authenticator/request/basicauthrequest"
	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
	oauthregistry "github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	"github.com/openshift/origin/pkg/cmd/server/origin"
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
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireEtcd()
}

func TestCLIGetToken(t *testing.T) {
	testutil.DeleteAllEtcdKeys()

	// setup
	etcdClient := testutil.NewEtcdClient()
	etcdHelper, _ := master.NewEtcdStorage(etcdClient, latest.InterfacesFor, latest.Version, etcdtest.PathPrefix())

	accessTokenStorage := accesstokenetcd.NewREST(etcdHelper)
	accessTokenRegistry := accesstokenregistry.NewRegistry(accessTokenStorage)
	authorizeTokenStorage := authorizetokenetcd.NewREST(etcdHelper)
	authorizeTokenRegistry := authorizetokenregistry.NewRegistry(authorizeTokenStorage)
	clientStorage := clientetcd.NewREST(etcdHelper)
	clientRegistry := clientregistry.NewRegistry(clientStorage)
	clientAuthStorage := clientauthetcd.NewREST(etcdHelper)
	clientAuthRegistry := clientauthregistry.NewRegistry(clientAuthStorage)

	userStorage := useretcd.NewREST(etcdHelper)
	userRegistry := userregistry.NewRegistry(userStorage)
	identityStorage := identityetcd.NewREST(etcdHelper)
	identityRegistry := identityregistry.NewRegistry(identityStorage)

	identityMapper := identitymapper.NewAlwaysCreateUserIdentityToUserMapper(identityRegistry, userRegistry)

	authRequestHandler := basicauthrequest.NewBasicAuthAuthentication(allowanypassword.New("get-token-test", identityMapper), true)
	authHandler := oauthhandlers.NewUnionAuthenticationHandler(
		map[string]oauthhandlers.AuthenticationChallenger{"login": passwordchallenger.NewBasicAuthChallenger("openshift")}, nil, nil)

	storage := registrystorage.New(accessTokenRegistry, authorizeTokenRegistry, clientRegistry, oauthregistry.NewUserConversion())
	config := osinserver.NewDefaultServerConfig()

	grantChecker := oauthregistry.NewClientAuthorizationGrantChecker(clientAuthRegistry)
	grantHandler := oauthhandlers.NewAutoGrant()

	server := osinserver.New(
		config,
		storage,
		osinserver.AuthorizeHandlers{
			oauthhandlers.NewAuthorizeAuthenticator(
				authRequestHandler,
				authHandler,
				oauthhandlers.EmptyError{},
			),
			oauthhandlers.NewGrantCheck(
				grantChecker,
				grantHandler,
				oauthhandlers.EmptyError{},
			),
		},
		osinserver.AccessHandlers{
			oauthhandlers.NewDenyAccessAuthenticator(),
		},
		osinserver.NewDefaultErrorHandler(),
	)
	mux := http.NewServeMux()
	server.Install(mux, origin.OpenShiftOAuthAPIPrefix)
	oauthServer := httptest.NewServer(http.Handler(mux))
	defer oauthServer.Close()
	t.Logf("oauth server is on %v\n", oauthServer.URL)

	// create the default oauth clients with redirects to our server
	origin.CreateOrUpdateDefaultOAuthClients(oauthServer.URL, []string{oauthServer.URL}, clientRegistry)

	flags := pflag.NewFlagSet("test-flags", pflag.ContinueOnError)
	clientCfg := clientcmd.NewConfig()
	clientCfg.Bind(flags)
	flags.Parse(strings.Split("--master="+oauthServer.URL, " "))

	reader := bytes.NewBufferString("user\npass")

	accessToken, err := tokencmd.RequestToken(clientCfg.OpenShiftConfig(), reader, "", "")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(accessToken) == 0 {
		t.Error("Expected accessToken, but did not get one")
	}

	// lets see if this access token is any good
	token, err := accessTokenRegistry.GetAccessToken(kapi.NewContext(), accessToken)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if token.UserName != "user" {
		t.Errorf("Expected token for \"user\", but got: %#v", token)
	}
}
