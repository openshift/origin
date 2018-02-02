package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RangelReale/osin"
	"github.com/RangelReale/osincli"
	"golang.org/x/oauth2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	oauthapi "github.com/openshift/api/oauth/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	"github.com/openshift/origin/pkg/oauthserver/osinserver"
	"github.com/openshift/origin/pkg/oauthserver/osinserver/registrystorage"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

type testUser struct {
	UserName string
	UserUID  string
	Err      error
}

func (u *testUser) ConvertToAuthorizeToken(_ interface{}, token *oauthapi.OAuthAuthorizeToken) error {
	token.UserName = u.UserName
	token.UserUID = u.UserUID
	return u.Err
}

func (u *testUser) ConvertToAccessToken(_ interface{}, token *oauthapi.OAuthAccessToken) error {
	token.UserName = u.UserName
	token.UserUID = u.UserUID
	return u.Err
}

func (u *testUser) ConvertFromAuthorizeToken(*oauthapi.OAuthAuthorizeToken) (interface{}, error) {
	return u.UserName, u.Err
}

func (u *testUser) ConvertFromAccessToken(*oauthapi.OAuthAccessToken) (interface{}, error) {
	return u.UserName, u.Err
}

func TestOAuthStorage(t *testing.T) {
	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterOptions)

	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterAPI(masterOptions)
	if err != nil {
		t.Fatal(err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	oauthClient, err := oauthclient.NewForConfig(clusterAdminClientConfig)
	if err != nil {
		t.Fatal(err)
	}

	user := &testUser{UserName: "test", UserUID: "1"}
	storage := registrystorage.New(oauthClient.OAuthAccessTokens(), oauthClient.OAuthAuthorizeTokens(), oauthClient.OAuthClients(), user, 0)

	oauthServer := osinserver.New(
		osinserver.NewDefaultServerConfig(),
		storage,
		osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, resp *osin.Response, w http.ResponseWriter) (bool, error) {
			ar.UserData = "test"
			ar.Authorized = true
			return false, nil
		}),
		osinserver.AccessHandlerFunc(func(ar *osin.AccessRequest, w http.ResponseWriter) error {
			ar.UserData = "test"
			ar.Authorized = true
			ar.GenerateRefresh = false
			return nil
		}),
		osinserver.NewDefaultErrorHandler(),
	)
	mux := http.NewServeMux()
	oauthServer.Install(mux, "")
	server := httptest.NewServer(mux)
	defer server.Close()

	ch := make(chan *osincli.AccessData, 1)
	var oaclient *osincli.Client
	var authReq *osincli.AuthorizeRequest
	assertServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := authReq.HandleRequest(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tokenReq := oaclient.NewAccessRequest(osincli.AUTHORIZATION_CODE, data)
		token, err := tokenReq.GetToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ch <- token
	}))

	oauthClient.OAuthClients().Create(&oauthapi.OAuthClient{
		ObjectMeta:        metav1.ObjectMeta{Name: "test"},
		Secret:            "secret",
		AdditionalSecrets: []string{"secret1"},
		RedirectURIs:      []string{assertServer.URL + "/assert"},
	})
	storedClient, err := storage.GetClient("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !osin.CheckClientSecret(storedClient, "secret") {
		t.Fatalf("unexpected stored client: %#v", storedClient)
	}
	if !osin.CheckClientSecret(storedClient, "secret1") {
		t.Fatalf("unexpected stored client: %#v", storedClient)
	}
	if osin.CheckClientSecret(storedClient, "secret2") {
		t.Fatalf("unexpected stored client: %#v", storedClient)
	}

	oaclientConfig := &osincli.ClientConfig{
		ClientId:     "test",
		ClientSecret: "secret",
		RedirectUrl:  assertServer.URL + "/assert",
		AuthorizeUrl: server.URL + "/authorize",
		TokenUrl:     server.URL + "/token",
	}
	osinclient, err := osincli.NewClient(oaclientConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oaclient = osinclient // initialize the assert server client as well
	authReq = oaclient.NewAuthorizeRequest(osincli.CODE)

	config := &oauth2.Config{
		ClientID:     "test",
		ClientSecret: "",
		Scopes:       []string{"user:info"},
		RedirectURL:  assertServer.URL + "/assert",
		Endpoint: oauth2.Endpoint{
			AuthURL:  server.URL + "/authorize",
			TokenURL: server.URL + "/token",
		},
	}
	url := config.AuthCodeURL("")
	client := http.Client{ /*CheckRedirect: func(req *http.Request, via []*http.Request) error {
		t.Logf("redirect (%d): to %s, %#v", len(via), req.URL, req)
		return nil
	}*/}

	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response: %#v", resp)
	}

	token := <-ch
	if token.AccessToken == "" {
		t.Errorf("unexpected access token: %#v", token)
	}

	actualToken, err := oauthClient.OAuthAccessTokens().Get(token.AccessToken, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actualToken.UserUID != "1" || actualToken.UserName != "test" {
		t.Errorf("unexpected stored token: %#v", actualToken)
	}
}
