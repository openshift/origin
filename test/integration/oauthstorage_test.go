package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RangelReale/osin"
	"github.com/RangelReale/osincli"
	"golang.org/x/oauth2"

	kapi "k8s.io/kubernetes/pkg/api"

	originrest "github.com/openshift/origin/pkg/cmd/server/origin/rest"
	"github.com/openshift/origin/pkg/oauth/api"
	accesstokenregistry "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	authorizetokenregistry "github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken"
	authorizetokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken/etcd"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	clientetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclient/etcd"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	registrystorage "github.com/openshift/origin/pkg/oauth/server/osinserver/registrystorage"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

type testUser struct {
	UserName string
	UserUID  string
	Err      error
}

func (u *testUser) ConvertToAuthorizeToken(_ interface{}, token *api.OAuthAuthorizeToken) error {
	token.UserName = u.UserName
	token.UserUID = u.UserUID
	return u.Err
}

func (u *testUser) ConvertToAccessToken(_ interface{}, token *api.OAuthAccessToken) error {
	token.UserName = u.UserName
	token.UserUID = u.UserUID
	return u.Err
}

func (u *testUser) ConvertFromAuthorizeToken(*api.OAuthAuthorizeToken) (interface{}, error) {
	return u.UserName, u.Err
}

func (u *testUser) ConvertFromAccessToken(*api.OAuthAccessToken) (interface{}, error) {
	return u.UserName, u.Err
}

func TestOAuthStorage(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	optsGetter := originrest.StorageOptions(*masterOptions)

	clientStorage, err := clientetcd.NewREST(optsGetter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clientRegistry := clientregistry.NewRegistry(clientStorage)

	accessTokenStorage, err := accesstokenetcd.NewREST(optsGetter, clientRegistry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	accessTokenRegistry := accesstokenregistry.NewRegistry(accessTokenStorage)

	authorizeTokenStorage, err := authorizetokenetcd.NewREST(optsGetter, clientRegistry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	authorizeTokenRegistry := authorizetokenregistry.NewRegistry(authorizeTokenStorage)

	user := &testUser{UserName: "test", UserUID: "1"}
	storage := registrystorage.New(accessTokenRegistry, authorizeTokenRegistry, clientRegistry, user)

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

	clientRegistry.CreateClient(kapi.NewContext(), &api.OAuthClient{
		ObjectMeta:        kapi.ObjectMeta{Name: "test"},
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

	actualToken, err := accessTokenRegistry.GetAccessToken(kapi.NewContext(), token.AccessToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actualToken.UserUID != "1" || actualToken.UserName != "test" {
		t.Errorf("unexpected stored token: %#v", actualToken)
	}
}
