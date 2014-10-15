package login

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/auth/api"
)

type testCSRF struct {
	Token string
	Err   error
}

func (t *testCSRF) Generate() (string, error) {
	return t.Token, t.Err
}

func (t *testCSRF) Check(token string) (bool, error) {
	return t.Token == token, t.Err
}

type testAuth struct {
	Username string
	Password string
	User     api.UserInfo
	Success  bool
	Err      error
	Then     string
	Called   bool
}

func (t *testAuth) AuthenticatePassword(user, password string) (api.UserInfo, bool, error) {
	t.Username = user
	t.Password = password
	return t.User, t.Success, t.Err
}

func (t *testAuth) AuthenticationSucceeded(user api.UserInfo, then string, w http.ResponseWriter, req *http.Request) {
	t.Called = true
	t.User = user
	t.Then = then
}

func TestLogin(t *testing.T) {
	testCases := map[string]struct {
		CSRF       *testCSRF
		Auth       *testAuth
		Path       string
		PostValues url.Values

		ExpectStatusCode int
		ExpectRedirect   string
		ExpectContains   []string
		ExpectThen       string
	}{
		"display form": {
			CSRF: &testCSRF{Token: "test"},
			Auth: &testAuth{},
			Path: "/login",

			ExpectStatusCode: 200,
			ExpectContains: []string{
				`action="/login"`,
				`name="csrf" value="test"`,
			},
		},
		"display form with errors": {
			CSRF: &testCSRF{Token: "test"},
			Auth: &testAuth{},
			Path: "?then=foo&reason=failed&username=user",

			ExpectStatusCode: 200,
			ExpectContains: []string{
				`action="/"`,
				`name="then" value="foo"`,
				`"message">An unknown error has occured`,
			},
		},
		"redirect when POST fails CSRF": {
			CSRF:           &testCSRF{Token: "test"},
			Auth:           &testAuth{},
			Path:           "/login",
			PostValues:     url.Values{"csrf": []string{"wrong"}},
			ExpectRedirect: "/login?reason=token+expired",
		},
		"redirect with 'then' when POST fails CSRF": {
			CSRF:           &testCSRF{Token: "test"},
			Auth:           &testAuth{},
			Path:           "/login?then=test",
			PostValues:     url.Values{"csrf": []string{"wrong"}},
			ExpectRedirect: "/login?reason=token+expired&then=test",
		},
		"redirect when no username": {
			CSRF: &testCSRF{Token: "test"},
			Auth: &testAuth{},
			Path: "/login",
			PostValues: url.Values{
				"csrf": []string{"test"},
			},
			ExpectRedirect: "/login?reason=user+required",
		},
		"redirect when not authenticated": {
			CSRF: &testCSRF{Token: "test"},
			Auth: &testAuth{Success: false},
			Path: "/login",
			PostValues: url.Values{
				"csrf":     []string{"test"},
				"username": []string{"user"},
			},
			ExpectRedirect: "/login?reason=access+denied",
		},
		"redirect on auth error": {
			CSRF: &testCSRF{Token: "test"},
			Auth: &testAuth{Err: errors.New("failed")},
			Path: "/login",
			PostValues: url.Values{
				"csrf":     []string{"test"},
				"username": []string{"user"},
			},
			ExpectRedirect: "/login?reason=unknown+error",
		},
		"login successful": {
			CSRF: &testCSRF{Token: "test"},
			Auth: &testAuth{Success: true, User: &api.DefaultUserInfo{Name: "user"}},
			Path: "/login?then=done",
			PostValues: url.Values{
				"csrf":     []string{"test"},
				"username": []string{"user"},
			},
			ExpectThen: "done",
		},
	}

	for k, testCase := range testCases {
		server := httptest.NewServer(NewLogin(testCase.CSRF, testCase.Auth, DefaultLoginFormRenderer))

		var resp *http.Response
		if testCase.PostValues != nil {
			r, err := postForm(server.URL+testCase.Path, testCase.PostValues)
			if err != nil {
				t.Errorf("%s: unexpected error: %v", k, err)
				continue
			}
			resp = r
		} else {
			r, err := getUrl(server.URL + testCase.Path)
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

		if testCase.ExpectThen != "" && (!testCase.Auth.Called || testCase.Auth.Then != testCase.ExpectThen) {
			t.Errorf("%s: did not find expected 'then' value: %#v", k, testCase.Auth)
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
