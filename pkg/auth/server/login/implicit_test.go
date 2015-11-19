package login

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/auth/user"

	"github.com/openshift/origin/pkg/auth/server/csrf"
)

type testImplicit struct {
	Request *http.Request
	User    user.Info
	Success bool
	Err     error
	Then    string
	Called  bool
}

func (t *testImplicit) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	t.Request = req
	return t.User, t.Success, t.Err
}

func (t *testImplicit) AuthenticationSucceeded(user user.Info, then string, w http.ResponseWriter, req *http.Request) (bool, error) {
	t.Called = true
	t.User = user
	t.Then = then
	return false, nil
}

func TestImplicit(t *testing.T) {
	testCases := map[string]struct {
		CSRF       csrf.CSRF
		Implicit   *testImplicit
		Path       string
		PostValues url.Values

		ExpectStatusCode int
		ExpectRedirect   string
		ExpectContains   []string
		ExpectThen       string
	}{
		"display confirm form": {
			CSRF:     &csrf.FakeCSRF{Token: "test", Err: nil},
			Implicit: &testImplicit{Success: true, User: &user.DefaultInfo{Name: "user"}},
			Path:     "/login",
			ExpectContains: []string{
				`action="/login"`,
				`You are now logged in as`,
			},
		},
		"successful POST redirects": {
			CSRF:       &csrf.FakeCSRF{Token: "test", Err: nil},
			Implicit:   &testImplicit{Success: true, User: &user.DefaultInfo{Name: "user"}},
			Path:       "/login?then=%2Ffoo",
			PostValues: url.Values{"csrf": []string{"test"}},
			ExpectThen: "/foo",
		},
		"redirect when POST fails CSRF": {
			CSRF:           &csrf.FakeCSRF{Token: "test", Err: nil},
			Implicit:       &testImplicit{Success: true, User: &user.DefaultInfo{Name: "user"}},
			Path:           "/login",
			PostValues:     url.Values{"csrf": []string{"wrong"}},
			ExpectRedirect: "/login?reason=token+expired",
		},
		"redirect when not authenticated": {
			CSRF:           &csrf.FakeCSRF{Token: "test", Err: nil},
			Implicit:       &testImplicit{Success: false},
			Path:           "/login",
			PostValues:     url.Values{"csrf": []string{"test"}},
			ExpectRedirect: "/login?reason=access+denied",
		},
		"redirect on auth failure": {
			CSRF:           &csrf.FakeCSRF{Token: "test", Err: nil},
			Implicit:       &testImplicit{Err: errors.New("failed")},
			Path:           "/login",
			PostValues:     url.Values{"csrf": []string{"test"}},
			ExpectRedirect: "/login?reason=access+denied",
		},
		"expect GET error": {
			CSRF:           &csrf.FakeCSRF{Token: "test", Err: nil},
			Implicit:       &testImplicit{Err: errors.New("failed")},
			ExpectContains: []string{`"message">An unknown error has occurred. Contact your administrator`},
		},
	}

	for k, testCase := range testCases {
		server := httptest.NewServer(NewConfirm(testCase.CSRF, testCase.Implicit, DefaultConfirmFormRenderer))

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

		if testCase.ExpectRedirect != "" {
			uri, err := resp.Location()
			if err != nil {
				t.Errorf("%s: unexpected error: %v", testCase.ExpectRedirect, err)
				continue
			}
			if uri.String() != server.URL+testCase.ExpectRedirect {
				t.Errorf("%s: unexpected redirect: %s", testCase.ExpectRedirect, uri.String())
			}
		}

		if testCase.ExpectThen != "" && (!testCase.Implicit.Called || testCase.Implicit.Then != testCase.ExpectThen) {
			t.Errorf("%s: did not find expected 'then' value: %#v", k, testCase.Implicit)
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
