package csrf

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCookieGenerate(t *testing.T) {

	testCases := map[string]struct {
		Name           string
		Path           string
		Domain         string
		Secure         bool
		HTTPOnly       bool
		ExistingCookie *http.Cookie

		ExpectToken     string
		ExpectSetCookie bool
	}{
		"use existing": {
			Name:           "csrf",
			ExistingCookie: &http.Cookie{Name: "csrf", Value: "existingvalue"},

			ExpectToken:     "existingvalue",
			ExpectSetCookie: false,
		},

		"set missing": {
			Name: "csrf",

			ExpectSetCookie: true,
		},

		"set missing with other cookies": {
			Name:           "csrf",
			ExistingCookie: &http.Cookie{Name: "csrf2", Value: "existingvalue"},

			ExpectSetCookie: true,
		},

		"set missing with cookie options": {
			Name:     "csrf",
			Path:     "/",
			Domain:   "foo.com",
			Secure:   true,
			HTTPOnly: true,

			ExpectSetCookie: true,
		},
	}

	for k, testCase := range testCases {
		csrf := NewCookieCSRF(testCase.Name, testCase.Path, testCase.Domain, testCase.Secure, testCase.HTTPOnly)

		req, _ := http.NewRequest("GET", "/", nil)
		if testCase.ExistingCookie != nil {
			req.AddCookie(testCase.ExistingCookie)
		}

		w := httptest.NewRecorder()
		token, err := csrf.Generate(w, req)

		if err != nil {
			t.Errorf("%s: Unexpected error: %v", k, err)
			continue
		}

		if len(testCase.ExpectToken) != 0 && token != testCase.ExpectToken {
			t.Errorf("%s: Unexpected token %s, got %s", k, testCase.ExpectToken, token)
			continue
		}

		setCookie := w.Header().Get("Set-Cookie")
		if testCase.ExpectSetCookie {
			if len(setCookie) == 0 {
				t.Errorf("%s: Expected set-cookie header", k)
				continue
			}

			protoCookie := &http.Cookie{
				Name:     testCase.Name,
				Value:    token,
				Path:     testCase.Path,
				Domain:   testCase.Domain,
				Secure:   testCase.Secure,
				HttpOnly: testCase.HTTPOnly,
			}
			if setCookie != protoCookie.String() {
				t.Errorf("%s: Expected Set-Cookie header of \"%s\", got \"%s\"", k, protoCookie.String(), setCookie)
				continue
			}
		} else {
			if len(setCookie) > 0 {
				t.Errorf("%s: Didn't expect set-cookie header, got %s", k, setCookie)
				continue
			}
		}
	}
}

func TestCookieCheck(t *testing.T) {

	testCases := map[string]struct {
		Name           string
		Token          string
		ExistingCookie *http.Cookie

		ExpectCheck bool
	}{
		"fail empty token": {
			Name:           "csrf",
			Token:          "",
			ExistingCookie: &http.Cookie{Name: "csrf", Value: "existingvalue"},

			ExpectCheck: false,
		},

		"fail empty cookie": {
			Name:           "csrf",
			Token:          "mytoken",
			ExistingCookie: &http.Cookie{Name: "csrf", Value: ""},

			ExpectCheck: false,
		},

		"fail missing cookie": {
			Name:  "csrf",
			Token: "mytoken",

			ExpectCheck: false,
		},

		"fail mismatch cookie": {
			Name:           "csrf",
			Token:          "mytoken",
			ExistingCookie: &http.Cookie{Name: "csrf", Value: "existingvalue"},

			ExpectCheck: false,
		},

		"fail mismatch cookie name": {
			Name:           "csrf",
			Token:          "mytoken",
			ExistingCookie: &http.Cookie{Name: "csrf2", Value: "mytoken"},

			ExpectCheck: false,
		},

		"pass matching cookie": {
			Name:           "csrf",
			Token:          "existingvalue",
			ExistingCookie: &http.Cookie{Name: "csrf", Value: "existingvalue"},

			ExpectCheck: true,
		},
	}

	for k, testCase := range testCases {
		csrf := NewCookieCSRF(testCase.Name, "", "", false, false)

		req, _ := http.NewRequest("GET", "/", nil)
		if testCase.ExistingCookie != nil {
			req.AddCookie(testCase.ExistingCookie)
		}

		ok, err := csrf.Check(req, testCase.Token)

		if err != nil {
			t.Errorf("%s: Unexpected error: %v", k, err)
			continue
		}

		if ok != testCase.ExpectCheck {
			t.Errorf("%s: Expected check to return %v, returned %v", k, testCase.ExpectCheck, ok)
			continue
		}
	}
}
