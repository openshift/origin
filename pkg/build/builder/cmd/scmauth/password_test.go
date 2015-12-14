package scmauth

import (
	"fmt"
	"net/url"
	"testing"
)

func TestPasswordHandles(t *testing.T) {
	tests := map[string]bool{
		"username": true,
		"user":     false,
		"token":    true,
		"ca.crt":   false,
		"password": true,
	}
	up := UsernamePassword{}
	for k, v := range tests {
		if a := up.Handles(k); a != v {
			t.Errorf("unexpected result for %s: %v", k, a)
		}
	}
}

func TestPassword(t *testing.T) {

	testcases := map[string]struct {
		URL      string
		Token    string
		Password string
		Username string

		ExpectedSourceURL string
		ExpectedConfigURL string
	}{
		// No user/pw in URL, no user/pw/token in secret: no gitconfig written, no URL override returned
		"no auth": {
			URL:      "http://example.com",
			Token:    "",
			Password: "",
			Username: "",

			ExpectedSourceURL: "",
			ExpectedConfigURL: "",
		},

		// User in URL, no user/pw/token in secret: no gitconfig written, no URL override returned
		"user only in URL": {
			URL:      "http://urluser@example.com",
			Token:    "",
			Password: "",
			Username: "",

			ExpectedSourceURL: "",
			ExpectedConfigURL: "",
		},

		// User and pw in URL, no user/pw/token in secret: gitconfig written with user/pw from URL, URL override returned with user/pw stripped
		"user/pw only in URL": {
			URL:      "http://urluser:urlpw@example.com",
			Token:    "",
			Password: "",
			Username: "",

			ExpectedSourceURL: "http://example.com",
			ExpectedConfigURL: "http://urluser:urlpw@example.com",
		},

		// User and pw in URL, user and pw in secret: gitconfig written with user/pw from secret, URL override returned with user/pw stripped
		"user/pw in URL, pw in secret": {
			URL:      "http://urluser:urlpw@example.com",
			Token:    "",
			Password: "secretpw",
			Username: "secretuser",

			ExpectedSourceURL: "http://example.com",
			ExpectedConfigURL: "http://secretuser:secretpw@example.com",
		},

		// User in URL only, password in secret only: gitconfig written with user from URL, password from secret, URL override returned with user stripped
		"user in URL, pw in secret": {
			URL:      "http://urluser@example.com",
			Token:    "",
			Password: "secretpw",
			Username: "",

			ExpectedSourceURL: "http://example.com",
			ExpectedConfigURL: "http://urluser:secretpw@example.com",
		},

		// No user/pw in URL, password in secret: gitconfig written with default user, password from secret
		"no user in URL, pw in secret": {
			URL:      "http://example.com",
			Token:    "",
			Password: "secretpw",
			Username: "",

			ExpectedSourceURL: "http://example.com",
			ExpectedConfigURL: fmt.Sprintf("http://%s:secretpw@example.com", DefaultUsername),
		},
	}

	for k, tc := range testcases {
		u, _ := url.Parse(tc.URL)
		sourceURL, configURL, err := doSetup(*u, tc.Username, tc.Password, tc.Token, removeCredentials)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}

		if len(tc.ExpectedSourceURL) == 0 && sourceURL != nil {
			t.Errorf("%s: Expected no source url, got %v", k, sourceURL)
		}
		if len(tc.ExpectedConfigURL) == 0 && configURL != nil {
			t.Errorf("%s: Expected no config url, got %v", k, configURL)
		}

		sourceURLString := ""
		if sourceURL != nil {
			sourceURLString = sourceURL.String()
		}

		configURLString := ""
		if configURL != nil {
			configURLString = configURL.String()
		}

		if tc.ExpectedSourceURL != sourceURLString {
			t.Errorf("%s: expected source URL override %q, got %q", k, tc.ExpectedSourceURL, sourceURLString)
		}
		if tc.ExpectedConfigURL != configURLString {
			t.Errorf("%s: expected config URL %q, got %q", k, tc.ExpectedConfigURL, configURLString)
		}
	}
}
