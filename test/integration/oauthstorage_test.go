// +build integration,!no-etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"code.google.com/p/goauth2/oauth"
	"github.com/RangelReale/osin"
	"github.com/RangelReale/osincli"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/etcd"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	registrystorage "github.com/openshift/origin/pkg/oauth/server/osinserver/registrystorage"
)

func init() {
	requireEtcd()
}

type testUser struct {
	UserName string
	UserUID  string
	Err      error
}

func (u *testUser) ConvertToAuthorizeToken(_ interface{}, token *api.AuthorizeToken) error {
	token.UserName = u.UserName
	token.UserUID = u.UserUID
	return u.Err
}

func (u *testUser) ConvertToAccessToken(_ interface{}, token *api.AccessToken) error {
	token.AuthorizeToken.UserName = u.UserName
	token.AuthorizeToken.UserUID = u.UserUID
	return u.Err
}

func (u *testUser) ConvertFromAuthorizeToken(*api.AuthorizeToken) (interface{}, error) {
	return u.UserName, u.Err
}

func (u *testUser) ConvertFromAccessToken(*api.AccessToken) (interface{}, error) {
	return u.UserName, u.Err
}

func TestOAuthStorage(t *testing.T) {
	deleteAllEtcdKeys()
	interfaces, _ := latest.InterfacesFor(latest.Version)
	etcdClient := newEtcdClient()
	etcdHelper := tools.EtcdHelper{etcdClient, interfaces.Codec, tools.RuntimeVersionAdapter{interfaces.MetadataAccessor}}
	registry := etcd.New(etcdHelper)

	user := &testUser{UserName: "test", UserUID: "1"}
	storage := registrystorage.New(registry, registry, registry, user)

	oauthServer := osinserver.New(
		osinserver.NewDefaultServerConfig(),
		storage,
		osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, w http.ResponseWriter, r *http.Request) bool {
			ar.UserData = "test"
			ar.Authorized = true
			return false
		}),
		osinserver.AccessHandlerFunc(func(ar *osin.AccessRequest, w http.ResponseWriter, r *http.Request) {
			ar.UserData = "test"
			ar.Authorized = true
			ar.GenerateRefresh = false
		}),
	)
	mux := http.NewServeMux()
	oauthServer.Install(mux, "")
	server := httptest.NewServer(mux)

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

	registry.CreateClient(&api.Client{
		ObjectMeta:   kapi.ObjectMeta{Name: "test"},
		Secret:       "secret",
		RedirectURIs: []string{assertServer.URL + "/assert"},
	})
	storedClient, err := storage.GetClient("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storedClient.GetSecret() != "secret" {
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

	config := &oauth.Config{
		ClientId:     "test",
		ClientSecret: "",
		Scope:        "a_scope",
		RedirectURL:  assertServer.URL + "/assert",
		AuthURL:      server.URL + "/authorize",
		TokenURL:     server.URL + "/token",
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

	actualToken, err := registry.GetAccessToken(token.AccessToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actualToken.AuthorizeToken.UserUID != "1" || actualToken.AuthorizeToken.UserName != "test" {
		t.Errorf("unexpected stored token: %#v", actualToken)
	}
}
