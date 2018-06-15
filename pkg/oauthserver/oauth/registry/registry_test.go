package registry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RangelReale/osin"
	"github.com/RangelReale/osincli"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"

	oapi "github.com/openshift/api/oauth/v1"
	oauthfake "github.com/openshift/client-go/oauth/clientset/versioned/fake"
	"github.com/openshift/origin/pkg/oauthserver/api"
	"github.com/openshift/origin/pkg/oauthserver/oauth/handlers"
	"github.com/openshift/origin/pkg/oauthserver/osinserver"
	"github.com/openshift/origin/pkg/oauthserver/osinserver/registrystorage"
)

type testHandlers struct {
	AuthorizeHandler osinserver.AuthorizeHandler

	User         user.Info
	Authenticate bool
	Err          error
	AuthNeed     bool
	AuthErr      error
	GrantNeed    bool
	GrantErr     error

	HandleAuthorizeReq     *osin.AuthorizeRequest
	HandleAuthorizeResp    *osin.Response
	HandleAuthorizeHandled bool
	HandleAuthorizeErr     error

	AuthNeedHandled bool
	AuthNeedErr     error

	GrantNeedGranted bool
	GrantNeedHandled bool
	GrantNeedErr     error

	HandledErr error
}

func (h *testHandlers) HandleAuthorize(ar *osin.AuthorizeRequest, resp *osin.Response, w http.ResponseWriter) (handled bool, err error) {
	h.HandleAuthorizeReq = ar
	h.HandleAuthorizeResp = resp
	h.HandleAuthorizeHandled, h.HandleAuthorizeErr = h.AuthorizeHandler.HandleAuthorize(ar, resp, w)
	return h.HandleAuthorizeHandled, h.HandleAuthorizeErr
}

func (h *testHandlers) AuthenticationNeeded(client api.Client, w http.ResponseWriter, req *http.Request) (bool, error) {
	h.AuthNeed = true
	return h.AuthNeedHandled, h.AuthNeedErr
}

func (h *testHandlers) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) (bool, error) {
	h.AuthErr = err
	return true, nil
}

func (h *testHandlers) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	return h.User, h.Authenticate, h.Err
}

func (h *testHandlers) GrantNeeded(user user.Info, grant *api.Grant, w http.ResponseWriter, req *http.Request) (bool, bool, error) {
	h.GrantNeed = true
	return h.GrantNeedGranted, h.GrantNeedHandled, h.GrantNeedErr
}

func (h *testHandlers) GrantError(err error, w http.ResponseWriter, req *http.Request) (bool, error) {
	h.GrantErr = err
	return true, nil
}

func (h *testHandlers) HandleError(err error, w http.ResponseWriter, req *http.Request) {
	h.HandledErr = err
}

func TestRegistryAndServer(t *testing.T) {
	ch := make(chan *http.Request, 1)
	assertServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ch <- req
	}))

	validClient := &oapi.OAuthClient{
		ObjectMeta:   metav1.ObjectMeta{Name: "test"},
		Secret:       "secret",
		RedirectURIs: []string{assertServer.URL + "/assert"},
	}

	restrictedClient := &oapi.OAuthClient{
		ObjectMeta:   metav1.ObjectMeta{Name: "test"},
		Secret:       "secret",
		RedirectURIs: []string{assertServer.URL + "/assert"},
		ScopeRestrictions: []oapi.ScopeRestriction{
			{ExactValues: []string{"user:info"}},
		},
	}

	testCases := map[string]struct {
		Client      *oapi.OAuthClient
		ClientAuth  *oapi.OAuthClientAuthorization
		AuthSuccess bool
		AuthUser    user.Info
		Scope       string
		Check       func(*testHandlers, *http.Request)
	}{
		"needs auth": {
			Client: validClient,
			Check: func(h *testHandlers, _ *http.Request) {
				if !h.AuthNeed || h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil || h.HandleAuthorizeReq.Authorized {
					t.Errorf("expected request to need authentication: %#v", h)
				}
			},
		},
		"needs grant": {
			Client:      validClient,
			AuthSuccess: true,
			AuthUser: &user.DefaultInfo{
				Name: "user",
				UID:  "1",
			},
			Check: func(h *testHandlers, _ *http.Request) {
				if h.AuthNeed || !h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil || h.HandleAuthorizeReq.Authorized {
					t.Errorf("expected request to need to grant access: %#v", h)
				}
			},
		},
		"invalid scope": {
			Client:      validClient,
			AuthSuccess: true,
			AuthUser: &user.DefaultInfo{
				Name: "user",
			},
			Scope: "some-scope",
			Check: func(h *testHandlers, _ *http.Request) {
				if h.AuthNeed || h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil || h.HandleAuthorizeReq.Authorized || h.HandleAuthorizeResp.ErrorId != "invalid_scope" {
					t.Errorf("expected invalid_scope error: %#v, %#v, %#v", h, h.HandleAuthorizeReq, h.HandleAuthorizeResp)
				}
			},
		},
		"disallowed scope": {
			Client:      restrictedClient,
			AuthSuccess: true,
			AuthUser: &user.DefaultInfo{
				Name: "user",
			},
			Scope: "user:full",
			Check: func(h *testHandlers, _ *http.Request) {
				if h.AuthNeed || h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil || h.HandleAuthorizeReq.Authorized || h.HandleAuthorizeResp.ErrorId != "access_denied" {
					t.Errorf("expected access_denied error: %#v, %#v, %#v", h, h.HandleAuthorizeReq, h.HandleAuthorizeResp)
				}
			},
		},
		"has non covered grant": {
			Client:      validClient,
			AuthSuccess: true,
			AuthUser: &user.DefaultInfo{
				Name: "user",
				UID:  "1",
			},
			ClientAuth: &oapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "user:test"},
				UserName:   "user",
				UserUID:    "1",
				ClientName: "test",
				Scopes:     []string{"user:info"},
			},
			Scope: "user:info user:check-access",
			Check: func(h *testHandlers, req *http.Request) {
				if h.AuthNeed || !h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil || h.HandleAuthorizeReq.Authorized {
					t.Errorf("expected request to need to grant access because of uncovered scopes: %#v", h)
				}
			},
		},
		"has covered grant": {
			Client:      validClient,
			AuthSuccess: true,
			AuthUser: &user.DefaultInfo{
				Name: "user",
				UID:  "1",
			},
			ClientAuth: &oapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "user:test"},
				UserName:   "user",
				UserUID:    "1",
				ClientName: "test",
				Scopes:     []string{"user:info", "user:check-access"},
			},
			Scope: "user:info user:check-access",
			Check: func(h *testHandlers, req *http.Request) {
				if h.AuthNeed || h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil || !h.HandleAuthorizeReq.Authorized || h.HandleAuthorizeResp.ErrorId != "" {
					t.Errorf("unexpected flow: %#v, %#v, %#v", h, h.HandleAuthorizeReq, h.HandleAuthorizeResp)
				}
			},
		},
		"has auth and grant": {
			Client:      validClient,
			AuthSuccess: true,
			AuthUser: &user.DefaultInfo{
				Name: "user",
				UID:  "1",
			},
			ClientAuth: &oapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "user:test"},
				UserName:   "user",
				UserUID:    "1",
				ClientName: "test",
				Scopes:     []string{"user:full"},
			},
			Check: func(h *testHandlers, req *http.Request) {
				if h.AuthNeed || h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil || !h.HandleAuthorizeReq.Authorized || h.HandleAuthorizeResp.ErrorId != "" {
					t.Errorf("unexpected flow: %#v, %#v, %#v", h, h.HandleAuthorizeReq, h.HandleAuthorizeResp)
					return
				}
				if req == nil {
					t.Errorf("unexpected nil assertion request")
					return
				}
				if code := req.URL.Query().Get("code"); code == "" {
					t.Errorf("expected query param 'code', got: %#v", req)
				}
			},
		},
		"has auth with no UID, mimics impersonation": {
			Client:      validClient,
			AuthSuccess: true,
			AuthUser: &user.DefaultInfo{
				Name: "user",
			},
			ClientAuth: &oapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "user:test"},
				UserName:   "user",
				UserUID:    "2",
				ClientName: "test",
				Scopes:     []string{"user:full"},
			},
			Check: func(h *testHandlers, r *http.Request) {
				if h.AuthNeed || h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil || h.HandleAuthorizeReq.Authorized || h.HandleAuthorizeResp.ErrorId != "server_error" {
					t.Errorf("expected server_error error: %#v, %#v, %#v", h, h.HandleAuthorizeReq, h.HandleAuthorizeResp)
				}
			},
		},
		"has auth and grant with different UIDs": {
			Client:      validClient,
			AuthSuccess: true,
			AuthUser: &user.DefaultInfo{
				Name: "user",
				UID:  "1",
			},
			ClientAuth: &oapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "user:test"},
				UserName:   "user",
				UserUID:    "2",
				ClientName: "test",
				Scopes:     []string{"user:full"},
			},
			Check: func(h *testHandlers, _ *http.Request) {
				if h.AuthNeed || !h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil || h.HandleAuthorizeReq.Authorized {
					t.Errorf("expected request to need to grant access due to UID mismatch: %#v", h)
				}
			},
		},
	}

	for _, testCase := range testCases {
		h := &testHandlers{}
		h.Authenticate = testCase.AuthSuccess
		h.User = testCase.AuthUser
		objs := []runtime.Object{}
		if testCase.Client != nil {
			objs = append(objs, testCase.Client)
		}
		if testCase.ClientAuth != nil {
			objs = append(objs, testCase.ClientAuth)
		}
		fakeOAuthClient := oauthfake.NewSimpleClientset(objs...)
		storage := registrystorage.New(fakeOAuthClient.Oauth().OAuthAccessTokens(), fakeOAuthClient.Oauth().OAuthAuthorizeTokens(), fakeOAuthClient.Oauth().OAuthClients(), NewUserConversion(), 0)
		config := osinserver.NewDefaultServerConfig()

		h.AuthorizeHandler = osinserver.AuthorizeHandlers{
			handlers.NewAuthorizeAuthenticator(
				h,
				h,
				h,
			),
			handlers.NewGrantCheck(
				NewClientAuthorizationGrantChecker(fakeOAuthClient.Oauth().OAuthClientAuthorizations()),
				h,
				h,
			),
		}

		server := osinserver.New(
			config,
			storage,
			h,
			osinserver.AccessHandlers{
				handlers.NewDenyAccessAuthenticator(),
			},
			h,
		)
		mux := http.NewServeMux()
		server.Install(mux, "")
		s := httptest.NewServer(mux)

		oaclientConfig := &osincli.ClientConfig{
			ClientId:     "test",
			ClientSecret: "secret",
			RedirectUrl:  assertServer.URL + "/assert",
			AuthorizeUrl: s.URL + "/authorize",
			TokenUrl:     s.URL + "/token",
			Scope:        testCase.Scope,
		}
		oaclient, err := osincli.NewClient(oaclientConfig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		aReq := oaclient.NewAuthorizeRequest(osincli.CODE)
		if _, err := http.Get(aReq.GetAuthorizeUrl().String()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var req *http.Request
		select {
		case out := <-ch:
			req = out
		default:
		}

		testCase.Check(h, req)
	}
}
