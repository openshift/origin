package osinserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"code.google.com/p/goauth2/oauth"
	"github.com/RangelReale/osin"
	"github.com/RangelReale/osincli"

	"github.com/openshift/origin/pkg/oauth/server/osinserver/teststorage"
)

func TestClientCredentialFlow(t *testing.T) {
	storage := teststorage.New()
	storage.Clients["test"] = &osin.DefaultClient{
		Id:          "test",
		Secret:      "secret",
		RedirectUri: "http://localhost/redirect",
	}
	oauthServer := New(
		NewDefaultServerConfig(),
		storage,
		AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, w http.ResponseWriter, r *http.Request) bool {
			ar.Authorized = true
			return false
		}),
		AccessHandlerFunc(func(ar *osin.AccessRequest, w http.ResponseWriter, r *http.Request) {
			ar.Authorized = true
			ar.GenerateRefresh = false
		}),
	)
	mux := http.NewServeMux()
	oauthServer.Install(mux, "")
	server := httptest.NewServer(mux)

	config := &oauth.Config{
		ClientId:     "test",
		ClientSecret: "secret",
		Scope:        "a_scope",
		AuthURL:      server.URL + "/authorize",
		TokenURL:     server.URL + "/token",
	}

	transport := &oauth.Transport{Config: config}
	if err := transport.AuthenticateClient(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storage.AccessData == nil {
		t.Fatalf("unexpected nil access data")
	}
}

func TestAuthorizeStartFlow(t *testing.T) {
	storage := teststorage.New()
	oauthServer := New(
		NewDefaultServerConfig(),
		storage,
		AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, w http.ResponseWriter, r *http.Request) bool {
			ar.Authorized = true
			return false
		}),
		AccessHandlerFunc(func(ar *osin.AccessRequest, w http.ResponseWriter, r *http.Request) {
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

	storage.Clients["test"] = &osin.DefaultClient{
		Id:          "test",
		Secret:      "secret",
		RedirectUri: assertServer.URL + "/assert",
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
}
