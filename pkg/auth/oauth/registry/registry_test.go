package registry

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RangelReale/osincli"
	kapi "k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	oapi "github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/test"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	"github.com/openshift/origin/pkg/oauth/server/osinserver/registrystorage"
	userapi "github.com/openshift/origin/pkg/user/api"
	usertest "github.com/openshift/origin/pkg/user/registry/test"
	"k8s.io/kubernetes/pkg/auth/user"
)

type testHandlers struct {
	User         user.Info
	Authenticate bool
	Err          error
	AuthNeed     bool
	AuthErr      error
	GrantNeed    bool
	GrantErr     error

	AuthNeedHandled bool
	AuthNeedErr     error

	GrantNeedGranted bool
	GrantNeedHandled bool
	GrantNeedErr     error

	HandledErr error
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
		ObjectMeta:   kapi.ObjectMeta{Name: "test"},
		Secret:       "secret",
		RedirectURIs: []string{assertServer.URL + "/assert"},
	}
	validClientAuth := &oapi.OAuthClientAuthorization{
		UserName:   "user",
		ClientName: "test",
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
				if !h.AuthNeed || h.GrantNeed || h.AuthErr != nil || h.GrantErr != nil {
					t.Errorf("expected request to need authentication: %#v", h)
				}
			},
		},
		"needs grant": {
			Client:      validClient,
			AuthSuccess: true,
			AuthUser: &user.DefaultInfo{
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
			AuthUser: &user.DefaultInfo{
				Name: "user",
			},
			ClientAuth: &oapi.OAuthClientAuthorization{
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
			AuthUser: &user.DefaultInfo{
				Name: "user",
			},
			ClientAuth: &oapi.OAuthClientAuthorization{
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
			AuthUser: &user.DefaultInfo{
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
			client.Err = apierrs.NewNotFound("client", "unknown")
		}
		grant := &test.ClientAuthorizationRegistry{
			ClientAuthorization: testCase.ClientAuth,
		}
		if testCase.ClientAuth == nil {
			grant.Err = apierrs.NewNotFound("clientAuthorization", "test:test")
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
					h,
				),
				handlers.NewGrantCheck(
					NewClientAuthorizationGrantChecker(grant),
					h,
					h,
				),
			},
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

func TestAuthenticateTokenNotFound(t *testing.T) {
	tokenRegistry := &test.AccessTokenRegistry{Err: apierrs.NewNotFound("AccessToken", "token")}
	userRegistry := usertest.NewUserRegistry()
	tokenAuthenticator := NewTokenAuthenticator(tokenRegistry, userRegistry, identitymapper.NoopGroupMapper{})

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if found {
		t.Error("Found token, but it should be missing!")
	}
	if err == nil {
		t.Error("Expected not found error")
	}
	if !apierrs.IsNotFound(err) {
		t.Error("Expected not found error")
	}
	if userInfo != nil {
		t.Errorf("Unexpected user: %v", userInfo)
	}
}
func TestAuthenticateTokenOtherGetError(t *testing.T) {
	tokenRegistry := &test.AccessTokenRegistry{Err: errors.New("get error")}
	userRegistry := usertest.NewUserRegistry()
	tokenAuthenticator := NewTokenAuthenticator(tokenRegistry, userRegistry, identitymapper.NoopGroupMapper{})

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if found {
		t.Error("Found token, but it should be missing!")
	}
	if err == nil {
		t.Error("Expected error is missing!")
	}
	if err.Error() != tokenRegistry.Err.Error() {
		t.Errorf("Expected error %v, but got error %v", tokenRegistry.Err, err)
	}
	if userInfo != nil {
		t.Errorf("Unexpected user: %v", userInfo)
	}
}
func TestAuthenticateTokenExpired(t *testing.T) {
	tokenRegistry := &test.AccessTokenRegistry{
		Err: nil,
		AccessToken: &oapi.OAuthAccessToken{
			ObjectMeta: kapi.ObjectMeta{CreationTimestamp: util.Time{time.Now().Add(-1 * time.Hour)}},
			ExpiresIn:  600, // 10 minutes
		},
	}
	userRegistry := usertest.NewUserRegistry()
	tokenAuthenticator := NewTokenAuthenticator(tokenRegistry, userRegistry, identitymapper.NoopGroupMapper{})

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if found {
		t.Error("Found token, but it should be missing!")
	}
	if err != ErrExpired {
		t.Errorf("Unexpected error: %v", err)
	}
	if userInfo != nil {
		t.Errorf("Unexpected user: %v", userInfo)
	}
}
func TestAuthenticateTokenValidated(t *testing.T) {
	tokenRegistry := &test.AccessTokenRegistry{
		Err: nil,
		AccessToken: &oapi.OAuthAccessToken{
			ObjectMeta: kapi.ObjectMeta{CreationTimestamp: util.Time{time.Now()}},
			ExpiresIn:  600, // 10 minutes
			UserName:   "foo",
			UserUID:    string("bar"),
		},
	}
	userRegistry := usertest.NewUserRegistry()
	userRegistry.Get["foo"] = &userapi.User{ObjectMeta: kapi.ObjectMeta{UID: "bar"}}

	tokenAuthenticator := NewTokenAuthenticator(tokenRegistry, userRegistry, identitymapper.NoopGroupMapper{})

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if !found {
		t.Error("Did not find a token!")
	}
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if userInfo == nil {
		t.Error("Did not get a user!")
	}
}
