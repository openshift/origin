// +build integration,!no-etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"code.google.com/p/goauth2/oauth"
	"github.com/RangelReale/osin"
	"github.com/RangelReale/osincli"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/etcd"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	osinregistry "github.com/openshift/origin/pkg/oauth/server/osinserver/storage/registry"
)

func init() {
	requireEtcd()
}

type testUser struct {
	Err error
}

func (u *testUser) ConvertToAuthorizeToken(interface{}, *api.AuthorizeToken) error {
	return u.Err
}

func (u *testUser) ConvertToAccessToken(interface{}, *api.AccessToken) error {
	return u.Err
}

func (u *testUser) ConvertFromAuthorizeToken(*api.AuthorizeToken) (interface{}, error) {
	return nil, u.Err
}

func (u *testUser) ConvertFromAccessToken(*api.AccessToken) (interface{}, error) {
	return nil, u.Err
}

func TestOAuthStorage(t *testing.T) {
	registry := etcd.New(newEtcdClient())

	user := &testUser{}
	storage := osinregistry.NewStorage(registry, registry, registry, user)

	oauthServer := osinserver.New(
		osinserver.DefaultServerConfig,
		storage,
		osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, w http.ResponseWriter, r *http.Request) bool {
			ar.Authorized = true
			return true
		}),
		osinserver.AccessHandlerFunc(func(ar *osin.AccessRequest, w http.ResponseWriter, r *http.Request) bool {
			ar.Authorized = true
			ar.GenerateRefresh = false
			return true
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
		Name:         "test",
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
	oaclient = osinclient
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
	if _, err := client.Get(url); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	token := <-ch
	if token.AccessToken == "" {
		t.Errorf("unexpected empty access token: %#v", token)
	}

	actualToken, err := registry.GetAccessToken(token.AccessToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("%#v", actualToken)
}
