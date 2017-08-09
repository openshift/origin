package v2

import (
	"net/http"
	"testing"
)

func TestBasicAuth(t *testing.T) {
	cases := []struct {
		*BasicAuthConfig
		name	 string
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
		client.BasicAuthConfig = tc.BasicAuthConfig
		client.doRequestFunc = addAuthCheck(t, tc.name, tc.BasicAuthConfig, client.doRequestFunc)
		client.prepareAndDo(http.MethodGet, client.URL, nil, nil)
	}
}

func addAuthCheck(t *testing.T, name string, authConfig *BasicAuthConfig, f doRequestFunc) doRequestFunc {
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
