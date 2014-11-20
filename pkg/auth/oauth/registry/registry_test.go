package registry

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/RangelReale/osincli"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
	oapi "github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/test"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	"github.com/openshift/origin/pkg/oauth/server/osinserver/registrystorage"
)

type testHandlers struct {
	User         api.UserInfo
	Authenticate bool
	Err          error
	AuthNeed     bool
	AuthErr      error
	GrantNeed    bool
	GrantErr     error
}

func (h *testHandlers) AuthenticationNeeded(w http.ResponseWriter, req *http.Request) {
	h.AuthNeed = true
}

func (h *testHandlers) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) {
	h.AuthErr = err
}

func (h *testHandlers) AuthenticateRequest(req *http.Request) (api.UserInfo, bool, error) {
	return h.User, h.Authenticate, h.Err
}

func (h *testHandlers) GrantNeeded(client api.Client, user api.UserInfo, grant *api.Grant, w http.ResponseWriter, req *http.Request) {
	h.GrantNeed = true
}

func (h *testHandlers) GrantError(err error, w http.ResponseWriter, req *http.Request) {
	h.GrantErr = err
}

func TestRegistryAndServer(t *testing.T) {
	ch := make(chan *http.Request, 1)
	assertServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ch <- req
	}))

	validClient := &oapi.Client{
		ObjectMeta:   kapi.ObjectMeta{Name: "test"},
		Secret:       "secret",
		RedirectURIs: []string{assertServer.URL + "/assert"},
	}
	validClientAuth := &oapi.ClientAuthorization{
		UserName:   "user",
		ClientName: "test",
	}

	testCases := map[string]struct {
		Client      *oapi.Client
		ClientAuth  *oapi.ClientAuthorization
		AuthSuccess bool
		AuthUser    api.UserInfo
		Scope       string
		Check       func(*testHandlers, *http.Request)
	}{
		"needs auth": {
			Client: validClient,
			Check: func(h *testHandlers, _ *http.Request) {
				if !h.AuthNeed || h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil {
					t.Errorf("expected request to need authentication: %#v", h)
				}
			},
		},
		"needs grant": {
			Client:      validClient,
			AuthSuccess: true,
			AuthUser: &api.DefaultUserInfo{
				Name: "user",
			},
			Check: func(h *testHandlers, _ *http.Request) {
				if h.AuthNeed || !h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil {
					t.Errorf("expected request to need to grant access: %#v", h)
				}
			},
		},
		"has non covered grant": {
			Client:      validClient,
			AuthSuccess: true,
			AuthUser: &api.DefaultUserInfo{
				Name: "user",
			},
			ClientAuth: &oapi.ClientAuthorization{
				UserName:   "user",
				ClientName: "test",
				Scopes:     []string{"test"},
			},
			Scope: "test other",
			Check: func(h *testHandlers, req *http.Request) {
				if h.AuthNeed || !h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil {
					t.Errorf("expected request to need to grant access because of uncovered scopes: %#v", h)
				}
			},
		},
		"has covered grant": {
			Client:      validClient,
			AuthSuccess: true,
			AuthUser: &api.DefaultUserInfo{
				Name: "user",
			},
			ClientAuth: &oapi.ClientAuthorization{
				UserName:   "user",
				ClientName: "test",
				Scopes:     []string{"test", "other"},
			},
			Scope: "test other",
			Check: func(h *testHandlers, req *http.Request) {
				if h.AuthNeed || h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil {
					t.Errorf("unexpected flow: %#v", h)
				}
			},
		},
		"has auth and grant": {
			Client:      validClient,
			AuthSuccess: true,
			AuthUser: &api.DefaultUserInfo{
				Name: "user",
			},
			ClientAuth: validClientAuth,
			Check: func(h *testHandlers, req *http.Request) {
				if h.AuthNeed || h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil {
					t.Errorf("unexpected flow: %#v", h)
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
	}

	for _, testCase := range testCases {
		h := &testHandlers{}
		h.Authenticate = testCase.AuthSuccess
		h.User = testCase.AuthUser
		access, authorize := &test.AccessTokenRegistry{}, &test.AuthorizeTokenRegistry{}
		client := &test.ClientRegistry{
			Client: testCase.Client,
		}
		if testCase.Client == nil {
			client.Err = errors.NewNotFound("client", "unknown")
		}
		grant := &test.ClientAuthorizationRegistry{
			ClientAuthorization: testCase.ClientAuth,
		}
		if testCase.ClientAuth == nil {
			grant.Err = errors.NewNotFound("clientAuthorization", "test:test")
		}
		storage := registrystorage.New(access, authorize, client, NewUserConversion())
		config := osinserver.NewDefaultServerConfig()
		server := osinserver.New(
			config,
			storage,
			osinserver.AuthorizeHandlers{
				handlers.NewAuthorizeAuthenticator(
					h,
					h,
				),
				handlers.NewGrantCheck(
					NewClientAuthorizationGrantChecker(grant),
					h,
				),
			},
			osinserver.AccessHandlers{
				handlers.NewDenyAccessAuthenticator(),
			},
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

func TestAuthenticateTokenNotFound(t *testing.T) {
	tokenRegistry := &test.AccessTokenRegistry{Err: errors.NewNotFound("AccessToken", "token")}
	tokenAuthenticator := NewTokenAuthenticator(tokenRegistry)

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if found {
		t.Error("Found token, but it should be missing!")
	}
	if err != nil {
		t.Error("Unexpected error: %v", err)
	}
	if userInfo != nil {
		t.Error("Unexpected user: %v", userInfo)
	}
}
func TestAuthenticateTokenOtherGetError(t *testing.T) {
	tokenRegistry := &test.AccessTokenRegistry{Err: fmt.Errorf("get error")}
	tokenAuthenticator := NewTokenAuthenticator(tokenRegistry)

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if found {
		t.Error("Found token, but it should be missing!")
	}
	if err == nil {
		t.Error("Expected error is missing!")
	}
	if err.Error() != tokenRegistry.Err.Error() {
		t.Error("Expected error %v, but got error %v", tokenRegistry.Err, err)
	}
	if userInfo != nil {
		t.Error("Unexpected user: %v", userInfo)
	}
}
func TestAuthenticateTokenExpired(t *testing.T) {
	tokenRegistry := &test.AccessTokenRegistry{
		Err: nil,
		AccessToken: &oapi.AccessToken{
			ObjectMeta: kapi.ObjectMeta{CreationTimestamp: util.Time{time.Now().Add(-1 * time.Hour)}},
			AuthorizeToken: oapi.AuthorizeToken{
				ExpiresIn: 600, // 10 minutes
			},
		},
	}
	tokenAuthenticator := NewTokenAuthenticator(tokenRegistry)

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if found {
		t.Error("Found token, but it should be missing!")
	}
	if err != nil {
		t.Error("Unexpected error: %v", err)
	}
	if userInfo != nil {
		t.Error("Unexpected user: %v", userInfo)
	}
}
func TestAuthenticateTokenValidated(t *testing.T) {
	tokenRegistry := &test.AccessTokenRegistry{
		Err: nil,
		AccessToken: &oapi.AccessToken{
			ObjectMeta: kapi.ObjectMeta{CreationTimestamp: util.Time{time.Now()}},
			AuthorizeToken: oapi.AuthorizeToken{
				ExpiresIn: 600, // 10 minutes
			},
		},
	}
	tokenAuthenticator := NewTokenAuthenticator(tokenRegistry)

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if !found {
		t.Error("Did not find a token!")
	}
	if err != nil {
		t.Error("Unexpected error: %v", err)
	}
	if userInfo == nil {
		t.Error("Did not get a user!")
	}
}
