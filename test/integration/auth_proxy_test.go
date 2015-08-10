// +build integration,!no-etcd

package integration

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools/etcdtest"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/headerrequest"
	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
	oauthregistry "github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	"github.com/openshift/origin/pkg/cmd/server/origin"
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
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireEtcd()
}

var (
	validUsers = []User{
		{ID: "sanefarmer", Password: "who?", Name: "Sane Farmer", Email: "insane_farmer@example.org"},
		{ID: "unsightlycook", Password: "what?", Name: "Unsightly Cook", Email: "beautiful_cook@example.org"},
		{ID: "novelresearcher", Password: "why?", Name: "Novel Researcher", Email: "trite_researcher@example.org"},
	}
)

func TestAuthProxyOnAuthorize(t *testing.T) {
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

	// this auth request handler is the one that is supposed to recognize information from a front proxy
	authRequestHandler := headerrequest.NewAuthenticator("front-proxy-test", headerrequest.NewDefaultConfig(), identityMapper)
	authHandler := &oauthhandlers.EmptyAuth{}

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

	// set up a front proxy guarding the oauth server
	proxyHTTPHandler := NewBasicAuthChallenger("TestRegistryAndServer", validUsers, NewXRemoteUserProxyingHandler(oauthServer.URL))
	proxyServer := httptest.NewServer(proxyHTTPHandler)
	defer proxyServer.Close()
	t.Logf("proxy server is on %v\n", proxyServer.URL)

	// need to prime clients so that we can get back a code.  the client must be valid
	createClient(t, clientRegistry, &oauthapi.OAuthClient{ObjectMeta: kapi.ObjectMeta{Name: "test"}, Secret: "secret", RedirectURIs: []string{oauthServer.URL}})

	// our simple URL to get back a code.  We want to go through the front proxy
	rawAuthorizeRequest := proxyServer.URL + origin.OpenShiftOAuthAPIPrefix + "/authorize?response_type=code&client_id=test"

	// the first request we make to the front proxy should challenge us for authentication info
	shouldBeAChallengeResponse, err := http.Get(rawAuthorizeRequest)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if shouldBeAChallengeResponse.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected Unauthorized, but got %v", shouldBeAChallengeResponse.StatusCode)
	}

	// create an http.Client to make our next request.  We need a custom Transport to authenticate us through our front proxy
	// and a custom CheckRedirect so that we can keep track of the redirect responses we're getting
	// OAuth requests a few redirects that we don't really care about checking, so this simpler than using a round tripper
	// and manually handling redirects and setting our auth information every time for the front proxy
	redirectedUrls := make([]url.URL, 10)
	httpClient := http.Client{
		CheckRedirect: getRedirectMethod(t, &redirectedUrls),
		Transport:     kclient.NewBasicAuthRoundTripper("sanefarmer", "who?", http.DefaultTransport),
	}

	// make our authorize request again, but this time our transport has properly set the auth info for the front proxy
	req, err := http.NewRequest("GET", rawAuthorizeRequest, nil)
	_, err = httpClient.Do(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// check the last redirect and see if we got a code
	foundCode := ""
	if len(redirectedUrls) > 0 {
		foundCode = redirectedUrls[len(redirectedUrls)-1].Query().Get("code")
	}

	if len(foundCode) == 0 {
		t.Errorf("Did not find code in any redirect: %v", redirectedUrls)
	} else {
		t.Logf("Found code %v\n", foundCode)
	}
}

func createClient(t *testing.T, clientRegistry clientregistry.Registry, client *oauthapi.OAuthClient) {
	if _, err := clientRegistry.CreateClient(kapi.NewContext(), client); err != nil {
		t.Errorf("Error creating client: %v due to %v\n", client, err)
	}
}

type checkRedirect func(req *http.Request, via []*http.Request) error

func getRedirectMethod(t *testing.T, redirectRecord *[]url.URL) checkRedirect {
	return func(req *http.Request, via []*http.Request) error {
		t.Logf("Going to %v\n", req.URL)
		*redirectRecord = append(*redirectRecord, *req.URL)

		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
}
