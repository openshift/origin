package grant

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apiserver/pkg/authentication/user"
	clienttesting "k8s.io/client-go/testing"

	oapi "github.com/openshift/api/oauth/v1"
	oauthfake "github.com/openshift/client-go/oauth/clientset/versioned/fake"
	oauthclientregistry "github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/oauthserver/server/csrf"
)

type testAuth struct {
	User    user.Info
	Success bool
	Err     error
}

func (t *testAuth) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	return t.User, t.Success, t.Err
}

func goodAuth(username string) *testAuth {
	return &testAuth{Success: true, User: &user.DefaultInfo{Name: username}}
}
func badAuth(err error) *testAuth {
	return &testAuth{Success: false, User: nil, Err: err}
}

func emptyClientRegistry() oauthclientregistry.Getter {
	return oauthfake.NewSimpleClientset().Oauth().OAuthClients()
}

func goodClientRegistry(clientID string, redirectURIs []string, literalScopes []string) oauthclientregistry.Getter {
	client := &oapi.OAuthClient{ObjectMeta: metav1.ObjectMeta{Name: clientID}, Secret: "mysecret", RedirectURIs: redirectURIs}
	client.Name = clientID
	if len(literalScopes) > 0 {
		client.ScopeRestrictions = []oapi.ScopeRestriction{{ExactValues: literalScopes}}
	}
	fakeOAuthClient := oauthfake.NewSimpleClientset(client)

	return fakeOAuthClient.Oauth().OAuthClients()
}
func badClientRegistry(err error) oauthclientregistry.Getter {
	fakeOAuthClient := oauthfake.NewSimpleClientset()
	fakeOAuthClient.PrependReactor("get", "oauthclients", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, err
	})
	return fakeOAuthClient.Oauth().OAuthClients()
}

func emptyAuthRegistry() *oauthfake.Clientset {
	return oauthfake.NewSimpleClientset()
}

func existingAuthRegistry(scopes []string) *oauthfake.Clientset {
	auth := &oapi.OAuthClientAuthorization{
		UserName:   "existingUserName",
		UserUID:    "existingUserUID",
		ClientName: "existingClientName",
		Scopes:     scopes,
	}
	auth.Name = "username:myclient"
	return oauthfake.NewSimpleClientset(auth)
}

func TestGrant(t *testing.T) {
	testCases := map[string]struct {
		CSRF           csrf.CSRF
		Auth           *testAuth
		ClientRegistry oauthclientregistry.Getter
		AuthRegistry   *oauthfake.Clientset

		Path       string
		PostValues url.Values

		ExpectStatusCode        int
		ExpectCreatedAuthScopes []string
		ExpectUpdatedAuthScopes []string
		ExpectRedirect          string
		ExpectContains          []string
		ExpectThen              string
	}{
		"display form": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           goodAuth("username"),
			ClientRegistry: goodClientRegistry("myclient", []string{"myredirect"}, []string{"myscope1", "myscope2"}),
			AuthRegistry:   emptyAuthRegistry(),
			Path:           "/grant?client_id=myclient&scope=myscope1%20myscope2&redirect_uri=/myredirect&then=/authorize",

			ExpectStatusCode: 200,
			ExpectContains: []string{
				`action="grant"`,
				`name="csrf" value="test"`,
				`name="client_id" value="myclient"`,
				`checked name="scope" value="myscope1"`,
				`checked name="scope" value="myscope2"`,
				`name="redirect_uri" value="/myredirect"`,
				`name="then" value="/authorize"`,
			},
		},

		"display form with existing scopes": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           goodAuth("username"),
			ClientRegistry: goodClientRegistry("myclient", []string{"myredirect"}, []string{"newscope1", "newscope2", "existingscope1", "existingscope2"}),
			AuthRegistry:   existingAuthRegistry([]string{"existingscope1", "existingscope2"}),
			Path:           "/grant?client_id=myclient&scope=newscope1%20newscope2%20existingscope1%20existingscope2&redirect_uri=/myredirect&then=/authorize",

			ExpectStatusCode: 200,
			ExpectContains: []string{
				`action="grant"`,
				`name="csrf" value="test"`,
				`name="client_id" value="myclient"`,
				`checked name="scope" value="newscope1"`,
				`checked name="scope" value="newscope1"`,
				`type="hidden" name="scope" value="existingscope1"`,
				`type="hidden" name="scope" value="existingscope2"`,
				`name="redirect_uri" value="/myredirect"`,
				`name="then" value="/authorize"`,
			},
		},

		"Unauthenticated with redirect": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           badAuth(nil),
			ClientRegistry: emptyClientRegistry(),
			AuthRegistry:   emptyAuthRegistry(),
			Path:           "/grant?then=/authorize",

			ExpectStatusCode: 302,
			ExpectRedirect:   "/authorize",
		},

		"Unauthenticated without redirect": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           badAuth(nil),
			ClientRegistry: emptyClientRegistry(),
			AuthRegistry:   emptyAuthRegistry(),
			Path:           "/grant",

			ExpectStatusCode: 200,
			ExpectContains:   []string{"reauthenticate"},
		},

		"Auth error with redirect": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           badAuth(errors.New("Auth error")),
			ClientRegistry: emptyClientRegistry(),
			AuthRegistry:   emptyAuthRegistry(),
			Path:           "/grant?then=/authorize",

			ExpectStatusCode: 302,
			ExpectRedirect:   "/authorize",
		},

		"Auth error without redirect": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           badAuth(errors.New("Auth error")),
			ClientRegistry: emptyClientRegistry(),
			AuthRegistry:   emptyAuthRegistry(),
			Path:           "/grant",

			ExpectStatusCode: 200,
			ExpectContains:   []string{"reauthenticate"},
		},

		"error when POST fails CSRF": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           goodAuth("username"),
			ClientRegistry: goodClientRegistry("myclient", []string{"myredirect"}, []string{"myscope1", "myscope2"}),
			AuthRegistry:   emptyAuthRegistry(),
			Path:           "/grant",
			PostValues: url.Values{
				"client_id":    {"myclient"},
				"scope":        {"myscope1", "myscope2"},
				"redirect_uri": {"/myredirect"},
				"then":         {"/authorize"},
				"csrf":         {"wrong"},
			},

			ExpectStatusCode: 200,
			ExpectContains:   []string{"CSRF"},
		},

		"error when POST fails user check": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           goodAuth("username"),
			ClientRegistry: goodClientRegistry("myclient", []string{"myredirect"}, []string{"myscope1", "myscope2"}),
			AuthRegistry:   emptyAuthRegistry(),
			Path:           "/grant",
			PostValues: url.Values{
				"client_id":    {"myclient"},
				"scope":        {"myscope1", "myscope2"},
				"redirect_uri": {"/myredirect"},
				"then":         {"/authorize"},
				"csrf":         {"test"},
				"user_name":    {"wrong"},
			},

			ExpectStatusCode: 200,
			ExpectContains:   []string{"User did not match"},
		},

		"error displaying form with invalid client": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           goodAuth("username"),
			ClientRegistry: badClientRegistry(nil),
			AuthRegistry:   emptyAuthRegistry(),
			Path:           "/grant",

			ExpectStatusCode: 200,
			ExpectContains:   []string{"find client"},
		},

		"error submitting form with invalid client": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           goodAuth("username"),
			ClientRegistry: badClientRegistry(nil),
			AuthRegistry:   emptyAuthRegistry(),
			Path:           "/grant",
			PostValues: url.Values{
				"approve":      {"true"},
				"client_id":    {"myclient"},
				"scope":        {"myscope1", "myscope2"},
				"redirect_uri": {"/myredirect"},
				"then":         {"/authorize"},
				"csrf":         {"test"},
				"user_name":    {"username"},
			},

			ExpectStatusCode: 200,
			ExpectContains:   []string{"find client"},
		},

		"successful create grant with redirect": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           goodAuth("username"),
			ClientRegistry: goodClientRegistry("myclient", []string{"myredirect"}, []string{"myscope1", "myscope2"}),
			AuthRegistry:   emptyAuthRegistry(),
			Path:           "/grant",
			PostValues: url.Values{
				"approve":      {"true"},
				"client_id":    {"myclient"},
				"scope":        {"myscope1", "myscope2"},
				"redirect_uri": {"/myredirect"},
				"then":         {"/authorize"},
				"csrf":         {"test"},
				"user_name":    {"username"},
			},

			ExpectStatusCode:        302,
			ExpectCreatedAuthScopes: []string{"myscope1", "myscope2"},
			ExpectRedirect:          "/authorize?scope=myscope1+myscope2",
		},

		"successful create grant without redirect": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           goodAuth("username"),
			ClientRegistry: goodClientRegistry("myclient", []string{"myredirect"}, []string{"myscope1", "myscope2"}),
			AuthRegistry:   emptyAuthRegistry(),
			Path:           "/grant",
			PostValues: url.Values{
				"approve":      {"true"},
				"client_id":    {"myclient"},
				"scope":        {"myscope1", "myscope2"},
				"redirect_uri": {"/myredirect"},
				"csrf":         {"test"},
				"user_name":    {"username"},
			},

			ExpectStatusCode:        200,
			ExpectCreatedAuthScopes: []string{"myscope1", "myscope2"},
			ExpectContains: []string{
				"granted",
				"no redirect",
			},
		},

		"successful update grant with identical scopes": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           goodAuth("username"),
			ClientRegistry: goodClientRegistry("myclient", []string{"myredirect"}, []string{"myscope1", "myscope2"}),
			AuthRegistry:   existingAuthRegistry([]string{"myscope2", "myscope1"}),
			Path:           "/grant",
			PostValues: url.Values{
				"approve":      {"true"},
				"client_id":    {"myclient"},
				"scope":        {"myscope1", "myscope2"},
				"redirect_uri": {"/myredirect"},
				"then":         {"/authorize"},
				"csrf":         {"test"},
				"user_name":    {"username"},
			},

			ExpectStatusCode:        302,
			ExpectUpdatedAuthScopes: []string{"myscope1", "myscope2"},
			ExpectRedirect:          "/authorize?scope=myscope1+myscope2",
		},

		"successful update grant with partial additional scopes": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           goodAuth("username"),
			ClientRegistry: goodClientRegistry("myclient", []string{"myredirect"}, []string{"newscope1", "newscope2", "existingscope1", "existingscope2"}),
			AuthRegistry:   existingAuthRegistry([]string{"existingscope2", "existingscope1"}),
			Path:           "/grant",
			PostValues: url.Values{
				"approve":      {"true"},
				"client_id":    {"myclient"},
				"scope":        {"newscope1", "existingscope1"},
				"redirect_uri": {"/myredirect"},
				"then":         {"/authorize?scope=newscope1+newscope2+existingscope1"},
				"csrf":         {"test"},
				"user_name":    {"username"},
			},

			ExpectStatusCode:        302,
			ExpectUpdatedAuthScopes: []string{"existingscope1", "existingscope2", "newscope1"},
			ExpectRedirect:          "/authorize?scope=newscope1+existingscope1",
		},

		"successful update grant with additional scopes": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           goodAuth("username"),
			ClientRegistry: goodClientRegistry("myclient", []string{"myredirect"}, []string{"newscope1", "existingscope1", "existingscope2"}),
			AuthRegistry:   existingAuthRegistry([]string{"existingscope2", "existingscope1"}),
			Path:           "/grant",
			PostValues: url.Values{
				"approve":      {"true"},
				"client_id":    {"myclient"},
				"scope":        {"newscope1", "existingscope1"},
				"redirect_uri": {"/myredirect"},
				"then":         {"/authorize"},
				"csrf":         {"test"},
				"user_name":    {"username"},
			},

			ExpectStatusCode:        302,
			ExpectUpdatedAuthScopes: []string{"existingscope1", "existingscope2", "newscope1"},
			ExpectRedirect:          "/authorize?scope=newscope1+existingscope1",
		},

		"successful reject grant via deny button": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           goodAuth("username"),
			ClientRegistry: goodClientRegistry("myclient", []string{"myredirect"}, []string{"myscope1", "myscope2"}),
			AuthRegistry:   existingAuthRegistry([]string{"existingscope2", "existingscope1"}),
			Path:           "/grant",
			PostValues: url.Values{
				"deny":         {"true"},
				"client_id":    {"myclient"},
				"scope":        {"newscope1", "existingscope1"},
				"redirect_uri": {"/myredirect"},
				"then":         {"/authorize"},
				"csrf":         {"test"},
				"user_name":    {"username"},
			},

			ExpectStatusCode: 302,
			ExpectRedirect:   "/authorize?error=access_denied",
		},

		"successful reject grant via unchecking all requested scopes and approving": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           goodAuth("username"),
			ClientRegistry: goodClientRegistry("myclient", []string{"myredirect"}, []string{"myscope1", "myscope2"}),
			AuthRegistry:   existingAuthRegistry([]string{"existingscope2", "existingscope1"}),
			Path:           "/grant",
			PostValues: url.Values{
				"approve":   {"true"},
				"client_id": {"myclient"},
				// "scope":       {"newscope1", "existingscope1"},
				"redirect_uri": {"/myredirect"},
				"then":         {"/authorize"},
				"csrf":         {"test"},
				"user_name":    {"username"},
			},

			ExpectStatusCode: 302,
			ExpectRedirect:   "/authorize?error=access_denied",
		},
	}

	for k, testCase := range testCases {
		server := httptest.NewServer(NewGrant(testCase.CSRF, testCase.Auth, DefaultFormRenderer, testCase.ClientRegistry, testCase.AuthRegistry.Oauth().OAuthClientAuthorizations()))

		var resp *http.Response
		if testCase.PostValues != nil {
			r, err := postForm(server.URL+testCase.Path, testCase.PostValues)
			if err != nil {
				t.Errorf("%s: unexpected error: %v", k, err)
				continue
			}
			resp = r
		} else {
			r, err := getURL(server.URL + testCase.Path)
			if err != nil {
				t.Errorf("%s: unexpected error: %v", k, err)
				continue
			}
			resp = r
		}
		defer resp.Body.Close()

		if testCase.ExpectStatusCode != 0 && testCase.ExpectStatusCode != resp.StatusCode {
			t.Errorf("%s: unexpected response: %#v", k, resp)
			continue
		}

		if len(testCase.ExpectCreatedAuthScopes) > 0 {
			found := false
			for _, action := range testCase.AuthRegistry.Actions() {
				if action.Matches("create", "oauthclientauthorizations") {
					found = true
					auth := action.(clienttesting.CreateAction).GetObject().(*oapi.OAuthClientAuthorization)
					if !reflect.DeepEqual(testCase.ExpectCreatedAuthScopes, auth.Scopes) {
						t.Errorf("%s: expected created scopes %v, got %v", k, testCase.ExpectCreatedAuthScopes, auth.Scopes)
						break
					}
				}
			}
			if !found {
				t.Errorf("%s: expected created auth, got nil", k)
				continue
			}
		}

		if len(testCase.ExpectUpdatedAuthScopes) > 0 {
			found := false
			for _, action := range testCase.AuthRegistry.Actions() {
				if action.Matches("update", "oauthclientauthorizations") {
					found = true
					auth := action.(clienttesting.UpdateAction).GetObject().(*oapi.OAuthClientAuthorization)
					if !reflect.DeepEqual(testCase.ExpectUpdatedAuthScopes, auth.Scopes) {
						t.Errorf("%s: expected updated scopes %v, got %v", k, testCase.ExpectUpdatedAuthScopes, auth.Scopes)
						break
					}
				}
			}
			if !found {
				t.Errorf("%s: expected updated auth, got nil", k)
				continue
			}
		}

		if testCase.ExpectRedirect != "" {
			uri, err := resp.Location()
			if err != nil {
				t.Errorf("%s: unexpected error: %v", k, err)
				continue
			}
			if uri.String() != server.URL+testCase.ExpectRedirect {
				t.Errorf("%s: unexpected redirect: %s", k, uri.String())
			}
		}

		if len(testCase.ExpectContains) > 0 {
			data, _ := ioutil.ReadAll(resp.Body)
			body := string(data)
			for i := range testCase.ExpectContains {
				if !strings.Contains(body, testCase.ExpectContains[i]) {
					t.Errorf("%s: did not find expected value %s: %s", k, testCase.ExpectContains[i], body)
					continue
				}
			}
		}
	}
}

func postForm(url string, body url.Values) (resp *http.Response, err error) {
	tr := knet.SetTransportDefaults(&http.Transport{})
	req, err := http.NewRequest("POST", url, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return tr.RoundTrip(req)
}

func getURL(url string) (resp *http.Response, err error) {
	tr := knet.SetTransportDefaults(&http.Transport{})
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return tr.RoundTrip(req)
}
