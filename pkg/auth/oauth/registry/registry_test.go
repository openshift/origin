package registry

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RangelReale/osin"
	"github.com/RangelReale/osincli"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"
	clienttesting "k8s.io/client-go/testing"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	oapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthfake "github.com/openshift/origin/pkg/oauth/generated/internalclientset/fake"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	"github.com/openshift/origin/pkg/oauth/server/osinserver/registrystorage"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	usertest "github.com/openshift/origin/pkg/user/registry/test"
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

func TestAuthenticateTokenNotFound(t *testing.T) {
	fakeOAuthClient := oauthfake.NewSimpleClientset()
	userRegistry := usertest.NewUserRegistry()
	tokenAuthenticator := NewTokenAuthenticator(fakeOAuthClient.Oauth().OAuthAccessTokens(), userRegistry, identitymapper.NoopGroupMapper{})

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
	fakeOAuthClient := oauthfake.NewSimpleClientset()
	fakeOAuthClient.PrependReactor("get", "oauthaccesstokens", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("get error")
	})
	userRegistry := usertest.NewUserRegistry()
	tokenAuthenticator := NewTokenAuthenticator(fakeOAuthClient.Oauth().OAuthAccessTokens(), userRegistry, identitymapper.NoopGroupMapper{})

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if found {
		t.Error("Found token, but it should be missing!")
	}
	if err == nil {
		t.Error("Expected error is missing!")
	}
	if err.Error() != "get error" {
		t.Errorf("Expected error %v, but got error %v", "get error", err)
	}
	if userInfo != nil {
		t.Errorf("Unexpected user: %v", userInfo)
	}
}

func TestAuthenticateTokenExpired(t *testing.T) {
	fakeOAuthClient := oauthfake.NewSimpleClientset(
		// expired token that had a lifetime of 10 minutes
		&oapi.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{Name: "token1", CreationTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)}},
			ExpiresIn:  600,
			UserName:   "foo",
		},
		// non-expired token that has a lifetime of 10 minutes, but has a non-nil deletion timestamp
		&oapi.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{Name: "token2", CreationTimestamp: metav1.Time{Time: time.Now()}, DeletionTimestamp: &metav1.Time{}},
			ExpiresIn:  600,
			UserName:   "foo",
		},
	)
	userRegistry := usertest.NewUserRegistry()
	userRegistry.GetUsers["foo"] = &userapi.User{ObjectMeta: metav1.ObjectMeta{UID: "bar"}}

	tokenAuthenticator := NewTokenAuthenticator(fakeOAuthClient.Oauth().OAuthAccessTokens(), userRegistry, identitymapper.NoopGroupMapper{}, NewExpirationValidator())

	for _, tokenName := range []string{"token1", "token2"} {
		userInfo, found, err := tokenAuthenticator.AuthenticateToken(tokenName)
		if found {
			t.Error("Found token, but it should be missing!")
		}
		if err != errExpired {
			t.Errorf("Unexpected error: %v", err)
		}
		if userInfo != nil {
			t.Errorf("Unexpected user: %v", userInfo)
		}
	}
}

func TestAuthenticateTokenInvalidUID(t *testing.T) {
	fakeOAuthClient := oauthfake.NewSimpleClientset(
		&oapi.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{Name: "token", CreationTimestamp: metav1.Time{Time: time.Now()}},
			ExpiresIn:  600, // 10 minutes
			UserName:   "foo",
			UserUID:    string("bar1"),
		},
	)
	userRegistry := usertest.NewUserRegistry()
	userRegistry.GetUsers["foo"] = &userapi.User{ObjectMeta: metav1.ObjectMeta{UID: "bar2"}}

	tokenAuthenticator := NewTokenAuthenticator(fakeOAuthClient.Oauth().OAuthAccessTokens(), userRegistry, identitymapper.NoopGroupMapper{}, NewUIDValidator())

	userInfo, found, err := tokenAuthenticator.AuthenticateToken("token")
	if found {
		t.Error("Found token, but it should be missing!")
	}
	if err.Error() != "user.UID (bar2) does not match token.userUID (bar1)" {
		t.Errorf("Unexpected error: %v", err)
	}
	if userInfo != nil {
		t.Errorf("Unexpected user: %v", userInfo)
	}
}

func TestAuthenticateTokenValidated(t *testing.T) {
	fakeOAuthClient := oauthfake.NewSimpleClientset(
		&oapi.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{Name: "token", CreationTimestamp: metav1.Time{Time: time.Now()}},
			ExpiresIn:  600, // 10 minutes
			UserName:   "foo",
			UserUID:    string("bar"),
		},
	)
	userRegistry := usertest.NewUserRegistry()
	userRegistry.GetUsers["foo"] = &userapi.User{ObjectMeta: metav1.ObjectMeta{UID: "bar"}}

	tokenAuthenticator := NewTokenAuthenticator(fakeOAuthClient.Oauth().OAuthAccessTokens(), userRegistry, identitymapper.NoopGroupMapper{}, NewExpirationValidator(), NewUIDValidator())

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

type fakeOAuthClientLister struct {
	clients oauthclient.OAuthClientInterface
}

func (f fakeOAuthClientLister) Get(name string) (*oapi.OAuthClient, error) {
	return f.clients.Get(name, metav1.GetOptions{})
}

func (f fakeOAuthClientLister) List(selector labels.Selector) ([]*oapi.OAuthClient, error) {
	var list []*oapi.OAuthClient
	ret, _ := f.clients.List(metav1.ListOptions{})
	for i := range ret.Items {
		list = append(list, &ret.Items[i])
	}
	return list, nil
}

func checkToken(t *testing.T, name string, authf authenticator.Token, tokens oauthclient.OAuthAccessTokenInterface, present bool) {
	userInfo, found, err := authf.AuthenticateToken(name)
	if present {
		if !found {
			t.Errorf("Did not find token %s!", name)
		}
		if err != nil {
			t.Errorf("Unexpected error checking for token %s: %v", name, err)
		}
		if userInfo == nil {
			t.Errorf("Did not get a user for token %s!", name)
		}
	} else {
		if found {
			token, _ := tokens.Get(name, metav1.GetOptions{})
			t.Errorf("Found token (created=%s, timeout=%di, now=%s), but it should be gone!",
				token.CreationTimestamp, token.InactivityTimeoutSeconds, time.Now())
		}
		if err != errTimedout {
			t.Errorf("Unexpected error checking absence of token %s: %v", name, err)
		}
		if userInfo != nil {
			t.Errorf("Unexpected user checking absence of token %s: %v", name, userInfo)
		}
	}
}

func TestAuthenticateTokenTimeout(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	defaultTimeout := int32(30) // 30 seconds
	clientTimeout := int32(15)  // 15 seconds so flush -> 5 seconds
	minTimeout := int32(10)     // 10 seconds

	testClient := oapi.OAuthClient{
		ObjectMeta:                          metav1.ObjectMeta{Name: "testClient"},
		AccessTokenInactivityTimeoutSeconds: &clientTimeout,
	}
	quickClient := oapi.OAuthClient{
		ObjectMeta:                          metav1.ObjectMeta{Name: "quickClient"},
		AccessTokenInactivityTimeoutSeconds: &minTimeout,
	}
	slowClient := oapi.OAuthClient{
		ObjectMeta: metav1.ObjectMeta{Name: "slowClient"},
	}
	testToken := oapi.OAuthAccessToken{
		ObjectMeta:               metav1.ObjectMeta{Name: "testToken", CreationTimestamp: metav1.Time{Time: time.Now()}},
		ClientName:               "testClient",
		ExpiresIn:                600, // 10 minutes
		UserName:                 "foo",
		UserUID:                  string("bar"),
		InactivityTimeoutSeconds: clientTimeout,
	}
	quickToken := oapi.OAuthAccessToken{
		ObjectMeta:               metav1.ObjectMeta{Name: "quickToken", CreationTimestamp: metav1.Time{Time: time.Now()}},
		ClientName:               "quickClient",
		ExpiresIn:                600, // 10 minutes
		UserName:                 "foo",
		UserUID:                  string("bar"),
		InactivityTimeoutSeconds: minTimeout,
	}
	slowToken := oapi.OAuthAccessToken{
		ObjectMeta:               metav1.ObjectMeta{Name: "slowToken", CreationTimestamp: metav1.Time{Time: time.Now()}},
		ClientName:               "slowClient",
		ExpiresIn:                600, // 10 minutes
		UserName:                 "foo",
		UserUID:                  string("bar"),
		InactivityTimeoutSeconds: defaultTimeout,
	}
	emergToken := oapi.OAuthAccessToken{
		ObjectMeta:               metav1.ObjectMeta{Name: "emergToken", CreationTimestamp: metav1.Time{Time: time.Now()}},
		ClientName:               "quickClient",
		ExpiresIn:                600, // 10 minutes
		UserName:                 "foo",
		UserUID:                  string("bar"),
		InactivityTimeoutSeconds: 5, // super short timeout
	}
	fakeOAuthClient := oauthfake.NewSimpleClientset(&testToken, &quickToken, &slowToken, &emergToken, &testClient, &quickClient, &slowClient)
	userRegistry := usertest.NewUserRegistry()
	userRegistry.GetUsers["foo"] = &userapi.User{ObjectMeta: metav1.ObjectMeta{UID: "bar"}}
	accessTokenGetter := fakeOAuthClient.Oauth().OAuthAccessTokens()
	oauthClients := fakeOAuthClient.Oauth().OAuthClients()
	lister := &fakeOAuthClientLister{
		clients: oauthClients,
	}
	timeouts := NewTimeoutValidator(accessTokenGetter, lister, defaultTimeout, minTimeout)
	tokenAuthenticator := NewTokenAuthenticator(accessTokenGetter, userRegistry, identitymapper.NoopGroupMapper{}, timeouts)

	go timeouts.Run(stopCh)

	// wait to see that the other thread has updated the timeouts
	time.Sleep(1 * time.Second)

	// TIME: 1 seconds have passed here

	// first time should succeed for all
	checkToken(t, "testToken", tokenAuthenticator, accessTokenGetter, true)
	checkToken(t, "quickToken", tokenAuthenticator, accessTokenGetter, true)
	checkToken(t, "slowToken", tokenAuthenticator, accessTokenGetter, true)
	// this should cause an emergency flush, if not the next auth will fail,
	// as the token will be timed out
	checkToken(t, "emergToken", tokenAuthenticator, accessTokenGetter, true)

	// wait 5 seconds
	time.Sleep(5 * time.Second)

	// TIME: 6th second

	// See if emergency flush happened
	checkToken(t, "emergToken", tokenAuthenticator, accessTokenGetter, true)

	// wait for timeout (minTimeout + 1 - the previously waited 6 seconds)
	time.Sleep(time.Duration(minTimeout-5) * time.Second)

	// TIME: 11th second

	// now we change the testClient and see if the testToken will still be
	// valid instead of timing out
	changeClient, ret := oauthClients.Get("testClient", metav1.GetOptions{})
	if ret != nil {
		t.Error("Failed to get testClient")
	} else {
		longTimeout := int32(20)
		changeClient.AccessTokenInactivityTimeoutSeconds = &longTimeout
		_, ret = oauthClients.Update(changeClient)
		if ret != nil {
			t.Error("Failed to update testClient")
		}
	}

	// this should fail
	checkToken(t, "quickToken", tokenAuthenticator, accessTokenGetter, false)
	// while this should get updated
	checkToken(t, "testToken", tokenAuthenticator, accessTokenGetter, true)

	// wait for timeout
	time.Sleep(time.Duration(clientTimeout+1) * time.Second)

	// TIME: 27th second

	// this should get updated
	checkToken(t, "slowToken", tokenAuthenticator, accessTokenGetter, true)

	// while this should not fail
	checkToken(t, "testToken", tokenAuthenticator, accessTokenGetter, true)
	// and should be updated to last at least till the 31st second
	token, err := accessTokenGetter.Get("testToken", metav1.GetOptions{})
	if err != nil {
		t.Error("Failed to get testToken")
	} else {
		if token.InactivityTimeoutSeconds < 31 {
			t.Errorf("Expected timeout in more than 31 seconds, found: %d", token.InactivityTimeoutSeconds)
		}
	}

	//now change testClient again, so that tokens do not expire anymore
	changeclient, ret := oauthClients.Get("testClient", metav1.GetOptions{})
	if ret != nil {
		t.Error("Failed to get testClient")
	} else {
		changeclient.AccessTokenInactivityTimeoutSeconds = new(int32)
		_, ret = oauthClients.Update(changeclient)
		if ret != nil {
			t.Error("Failed to update testClient")
		}
	}

	// and wait until test token should time out, and has been flushed for sure
	time.Sleep(time.Duration(minTimeout) * time.Second)

	// while this should not fail
	checkToken(t, "testToken", tokenAuthenticator, accessTokenGetter, true)
	// and should be updated to have a ZERO timeout
	token, err = accessTokenGetter.Get("testToken", metav1.GetOptions{})
	if err != nil {
		t.Error("Failed to get testToken")
	} else {
		if token.InactivityTimeoutSeconds != 0 {
			t.Errorf("Expected timeout of 0 seconds, found: %d", token.InactivityTimeoutSeconds)
		}
	}
}
