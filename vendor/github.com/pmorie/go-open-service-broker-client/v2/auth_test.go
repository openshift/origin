package v2

import (
	"net/http"
	"strings"
	"testing"
)

func TestBasicAuth(t *testing.T) {
	cases := []struct {
		*BasicAuthConfig
		name string
	}{
		{
			name: "No Auth",
		},
		{
			BasicAuthConfig: &BasicAuthConfig{
				Username: "CoolUser",
				Password: "HardPassword",
			},
			name: "Basic Auth",
		},
	}

	for _, tc := range cases {
		client := newTestClient(t, tc.name, Version2_11(), true, httpChecks{}, httpReaction{})
		client.AuthConfig = &AuthConfig{
			BasicAuthConfig: tc.BasicAuthConfig,
		}
		client.doRequestFunc = addBasicAuthCheck(t, tc.name, tc.BasicAuthConfig, client.doRequestFunc)
		client.prepareAndDo(http.MethodGet, client.URL, nil, nil, nil)
	}
}

func TestBearerAuth(t *testing.T) {
	cases := []struct {
		*BearerConfig
		name string
	}{
		{
			name: "No Auth",
		},
		{
			BearerConfig: &BearerConfig{
				Token: "SuchToken",
			},
			name: "Bearer Auth",
		},
	}

	for _, tc := range cases {
		client := newTestClient(t, tc.name, Version2_11(), true, httpChecks{}, httpReaction{})
		client.AuthConfig = &AuthConfig{
			BearerConfig: tc.BearerConfig,
		}
		client.doRequestFunc = addBearerAuthCheck(t, tc.name, tc.BearerConfig, client.doRequestFunc)
		client.prepareAndDo(http.MethodGet, client.URL, nil, nil, nil)
	}
}

func addBasicAuthCheck(t *testing.T, name string, authConfig *BasicAuthConfig, f doRequestFunc) doRequestFunc {
	return func(request *http.Request) (*http.Response, error) {
		u, p, ok := request.BasicAuth()
		if !ok && authConfig != nil {
			t.Errorf("%s: Expected basic auth in request but none found", name)
			return nil, walkingGhostErr
		} else if ok && authConfig != nil {
			if u != authConfig.Username {
				t.Errorf("%s: basic auth username test failed: expected %q but got %q", name, authConfig.Username, u)
				return nil, walkingGhostErr
			}
			if p != authConfig.Password {
				t.Errorf("%s: basic auth password test failed: expected %q but got %q", name, authConfig.Password, p)
				return nil, walkingGhostErr
			}
		}

		return f(request)
	}
}

func addBearerAuthCheck(t *testing.T, name string, authConfig *BearerConfig, f doRequestFunc) doRequestFunc {
	return func(request *http.Request) (*http.Response, error) {
		auth := request.Header.Get("Authorization")
		if auth == "" {
			if authConfig != nil {
				t.Errorf("%s: Expected bearer auth in request but none found", name)
				return nil, walkingGhostErr
			}
			return f(request)
		}
		token, ok := parseBearerToken(auth)
		if !ok && authConfig != nil {
			t.Errorf("%s: Expected bearer auth in request but none found", name)
			return nil, walkingGhostErr
		} else if ok && authConfig != nil {
			if token != authConfig.Token {
				t.Errorf("%s: bearer token test failed: expected %q but got %q", name, authConfig.Token, token)
				return nil, walkingGhostErr
			}
		}

		return f(request)
	}
}

// parseBearerToken parses an HTTP Bearer Authentication token.
// "Bearer abcde" returns ("abcde", true).
func parseBearerToken(auth string) (token string, ok bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return "", false
	}
	return auth[len(prefix):], true
}
