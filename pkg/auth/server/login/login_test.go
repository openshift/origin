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
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
)

type testAuth struct {
	Username string
	Password string
	User     user.Info
	Success  bool
	Err      error
	Then     string
	Called   bool
}

func (t *testAuth) AuthenticatePassword(user, password string) (user.Info, bool, error) {
	t.Username = user
	t.Password = password
	return t.User, t.Success, t.Err
}

func (t *testAuth) AuthenticationSucceeded(user user.Info, then string, w http.ResponseWriter, req *http.Request) (bool, error) {
	t.Called = true
	t.User = user
	t.Then = then
	return false, nil
}

func TestLogin(t *testing.T) {
	testCases := map[string]struct {
		CSRF       csrf.CSRF
		Auth       *testAuth
		Path       string
		PostValues url.Values

		ExpectStatusCode int
		ExpectRedirect   string
		ExpectContains   []string
		ExpectThen       string
	}{
		"display form": {
			CSRF: &csrf.FakeCSRF{Token: "test"},
			Auth: &testAuth{},
			Path: "/login",

			ExpectStatusCode: 200,
			ExpectContains: []string{
				`action="/login"`,
				`name="csrf" value="test"`,
			},
		},
		"display form with errors": {
			CSRF: &csrf.FakeCSRF{Token: "test"},
			Auth: &testAuth{},
			Path: "?then=foo&reason=failed&username=user",

			ExpectStatusCode: 200,
			ExpectContains: []string{
				`action="/"`,
				`name="then" value="foo"`,
				`An authentication error occurred.`,
				`danger`,
			},
		},
		"redirect when POST fails CSRF": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           &testAuth{},
			Path:           "/login",
			PostValues:     url.Values{"csrf": []string{"wrong"}},
			ExpectRedirect: "/login?reason=token_expired",
		},
		"redirect with 'then' when POST fails CSRF": {
			CSRF:           &csrf.FakeCSRF{Token: "test"},
			Auth:           &testAuth{},
			Path:           "/login?then=test",
			PostValues:     url.Values{"csrf": []string{"wrong"}},
			ExpectRedirect: "/login?reason=token_expired&then=test",
		},
		"redirect when no username": {
			CSRF: &csrf.FakeCSRF{Token: "test"},
			Auth: &testAuth{},
			Path: "/login",
			PostValues: url.Values{
				"csrf": []string{"test"},
			},
			ExpectRedirect: "/login?reason=user_required",
		},
		"redirect when not authenticated": {
			CSRF: &csrf.FakeCSRF{Token: "test"},
			Auth: &testAuth{Success: false},
			Path: "/login",
			PostValues: url.Values{
				"csrf":     []string{"test"},
				"username": []string{"user"},
			},
			ExpectRedirect: "/login?reason=access_denied",
		},
		"redirect on auth error": {
			CSRF: &csrf.FakeCSRF{Token: "test"},
			Auth: &testAuth{Err: errors.New("failed")},
			Path: "/login",
			PostValues: url.Values{
				"csrf":     []string{"test"},
				"username": []string{"user"},
			},
			ExpectRedirect: "/login?reason=authentication_error",
		},
		"redirect on lookup error": {
			CSRF: &csrf.FakeCSRF{Token: "test"},
			Auth: &testAuth{Err: identitymapper.NewLookupError(nil, nil)},
			Path: "/login",
			PostValues: url.Values{
				"csrf":     []string{"test"},
				"username": []string{"user"},
			},
			ExpectRedirect: "/login?reason=mapping_lookup_error",
		},
		"redirect on claim error": {
			CSRF: &csrf.FakeCSRF{Token: "test"},
			Auth: &testAuth{Err: identitymapper.NewClaimError(nil, nil)},
			Path: "/login",
			PostValues: url.Values{
				"csrf":     []string{"test"},
				"username": []string{"user"},
			},
			ExpectRedirect: "/login?reason=mapping_claim_error",
		},
		"redirect preserving then param": {
			CSRF: &csrf.FakeCSRF{Token: "test"},
			Auth: &testAuth{Err: errors.New("failed")},
			Path: "/login",
			PostValues: url.Values{
				"csrf":     []string{"test"},
				"username": []string{"user"},
				"then":     []string{"anotherurl"},
			},
			ExpectRedirect: "/login?reason=authentication_error&then=anotherurl",
		},
		"login successful": {
			CSRF: &csrf.FakeCSRF{Token: "test"},
			Auth: &testAuth{Success: true, User: &user.DefaultInfo{Name: "user"}},
			Path: "/login?then=done",
			PostValues: url.Values{
				"csrf":     []string{"test"},
				"username": []string{"user"},
			},
			ExpectThen: "done",
		},
	}

	for k, testCase := range testCases {
		loginFormRenderer, err := NewLoginFormRenderer("")
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		server := httptest.NewServer(NewLogin("myprovider", testCase.CSRF, testCase.Auth, loginFormRenderer))

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
					t.Errorf("%s: did not find expected value %s", k, testCase.ExpectContains[i])
					continue
				}
			}
		}
	}
}

func TestValidateLoginTemplate(t *testing.T) {
	testCases := map[string]struct {
		Template      string
		TemplateValid bool
	}{
		"default login template": {
			Template:      defaultLoginTemplateString,
			TemplateValid: true,
		},
		"login template example": {
			Template:      LoginTemplateExample,
			TemplateValid: true,
		},
		"original login template example": {
			Template:      originalLoginTemplateExample,
			TemplateValid: true,
		},
		"template with missing parameter": {
			Template:      invalidLoginTemplate,
			TemplateValid: false,
		},
	}

	for k, testCase := range testCases {
		allErrs := ValidateLoginTemplate([]byte(testCase.Template))
		if testCase.TemplateValid {
			for _, err := range allErrs {
				t.Errorf("%s: template validation failed when it should have succeeded: %v", k, err)
			}
		} else if len(allErrs) == 0 {
			t.Errorf("%s: template validation succeeded when it should have failed", k)
		}
	}
}

// Make sure the original version of the default template always validates
// this is to avoid breaking existing customized templates.
const originalLoginTemplateExample = `<!DOCTYPE html>
<!--

This template can be modified and used to customize the login page. To replace
the login page, set master configuration option oauthConfig.templates.login to
the path of the template file. Don't remove parameters in curly braces below.

oauthConfig:
  templates:
    login: templates/login-template.html

-->
<html>
  <head>
    <title>Login</title>
    <style type="text/css">
      body {
        font-family: "Open Sans", Helvetica, Arial, sans-serif;
        font-size: 14px;
        margin: 15px;
      }

      input {
        margin-bottom: 10px;
        width: 300px;
      }

      .error {
        color: red;
        margin-bottom: 10px;
      }
    </style>
  </head>
  <body>

    {{ if .Error }}
      <div class="error">{{ .Error }}</div>
    {{ end }}

    <form action="{{ .Action }}" method="POST">
      <input type="hidden" name="{{ .Names.Then }}" value="{{ .Values.Then }}">
      <input type="hidden" name="{{ .Names.CSRF }}" value="{{ .Values.CSRF }}">

      <div>
        <label for="inputUsername">Username</label>
      </div>
      <div>
        <input type="text" id="inputUsername" autofocus="autofocus" type="text" name="{{ .Names.Username }}" value="{{ .Values.Username }}">
      </div>

      <div>
        <label for="inputPassword">Password</label>
      </div>
      <div>
        <input type="password" id="inputPassword" type="password" name="{{ .Names.Password }}" value="">
      </div>

      <button type="submit">Log In</button>

    </form>

  </body>
</html>
`

// This template is missing the CSRF hidden input and should fail validation.
const invalidLoginTemplate = `<!DOCTYPE html>
<!--

This template can be modified and used to customize the login page. To replace
the login page, set master configuration option oauthConfig.templates.login to
the path of the template file. Don't remove parameters in curly braces below.

oauthConfig:
  templates:
    login: templates/login-template.html

-->
<html>
  <head>
    <title>Login</title>
    <style type="text/css">
      body {
        font-family: "Open Sans", Helvetica, Arial, sans-serif;
        font-size: 14px;
        margin: 15px;
      }

      input {
        margin-bottom: 10px;
        width: 300px;
      }

      .error {
        color: red;
        margin-bottom: 10px;
      }
    </style>
  </head>
  <body>

    {{ if .Error }}
      <div class="error">{{ .Error }}</div>
    {{ end }}

    <form action="{{ .Action }}" method="POST">
      <input type="hidden" name="{{ .Names.Then }}" value="{{ .Values.Then }}">

      <div>
        <label for="inputUsername">Username</label>
      </div>
      <div>
        <input type="text" id="inputUsername" autofocus="autofocus" type="text" name="{{ .Names.Username }}" value="{{ .Values.Username }}">
      </div>

      <div>
        <label for="inputPassword">Password</label>
      </div>
      <div>
        <input type="password" id="inputPassword" type="password" name="{{ .Names.Password }}" value="">
      </div>

      <button type="submit">Log In</button>

    </form>

  </body>
</html>
`
