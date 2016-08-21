package integration

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktransport "k8s.io/kubernetes/pkg/client/transport"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

var (
	validUsers = []User{
		{ID: "sanefarmer", Password: "who?", Name: "Sane Farmer", Email: "insane_farmer@example.org"},
		{ID: "unsightlycook", Password: "what?", Name: "Unsightly Cook", Email: "beautiful_cook@example.org"},
		{ID: "novelresearcher", Password: "why?", Name: "Novel Researcher", Email: "trite_researcher@example.org"},
	}
)

func TestAuthProxyOnAuthorize(t *testing.T) {
	idp := configapi.IdentityProvider{}
	idp.Name = "front-proxy"
	idp.Provider = &configapi.RequestHeaderIdentityProvider{Headers: []string{"X-Remote-User"}}
	idp.MappingMethod = "claim"

	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatal(err)
	}
	masterConfig.OAuthConfig.IdentityProviders = []configapi.IdentityProvider{idp}

	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	// set up a front proxy guarding the oauth server
	proxyHTTPHandler := NewBasicAuthChallenger("TestRegistryAndServer", validUsers, NewXRemoteUserProxyingHandler(clusterAdminClientConfig.Host))
	proxyServer := httptest.NewServer(proxyHTTPHandler)
	defer proxyServer.Close()
	t.Logf("proxy server is on %v\n", proxyServer.URL)

	// need to prime clients so that we can get back a code.  the client must be valid
	result := clusterAdminClient.RESTClient.Post().Resource("oAuthClients").Body(&oauthapi.OAuthClient{ObjectMeta: kapi.ObjectMeta{Name: "test"}, Secret: "secret", RedirectURIs: []string{clusterAdminClientConfig.Host}}).Do()
	if result.Error() != nil {
		t.Fatal(result.Error())
	}

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
		Transport:     ktransport.NewBasicAuthRoundTripper("sanefarmer", "who?", insecureTransport()),
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
